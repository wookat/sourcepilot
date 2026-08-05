package finance

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math/big"
	"sort"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"gorm.io/gorm"

	"github.com/trademind-ai/trademind/backend/internal/modules/operationlog"
	"github.com/trademind-ai/trademind/backend/internal/modules/order"
	"github.com/trademind-ai/trademind/backend/internal/modules/procurement"
	"github.com/trademind-ai/trademind/backend/internal/pkg/adminperm"
	"github.com/trademind-ai/trademind/backend/internal/providers/fxrate"
)

// Sentinel errors mapped to HTTP statuses by the handler.
var (
	ErrBadRequest = errors.New("finance: bad request")
	ErrNotFound   = errors.New("finance: not found")
	ErrForbidden  = errors.New("finance: forbidden")
)

// Service implements the finance reconciliation flows. Proc resolves actual /
// reference purchase costs, Settings the fx table plus expense-type and
// estimated-fee configuration.
type Service struct {
	DB       *gorm.DB
	Settings fxrate.SettingsReader
	Proc     *procurement.Service
	OpLog    *operationlog.Service
}

// Expense type configuration (settings group finance, item expense_types):
// a JSON array of {code,label}. Missing / invalid config degrades to the
// built-in defaults; custom entries are appended after them.
const (
	ExpenseSettingsGroup = "finance"
	ExpenseTypesKey      = "expense_types"
	maxExpenseTypes      = 50
)

// ExpenseType is one configurable expense category.
type ExpenseType struct {
	Code  string `json:"code"`
	Label string `json:"label"`
}

// DefaultExpenseTypes are the built-in order/shop expense categories.
func DefaultExpenseTypes() []ExpenseType {
	return []ExpenseType{
		{Code: "platform_commission", Label: "平台佣金"},
		{Code: "promotion", Label: "推广费"},
		{Code: "shipping", Label: "运费"},
		{Code: "other", Label: "其他"},
	}
}

// ExpenseTypes merges the built-in defaults with tenant-configured custom
// types (settings group finance / expense_types).
func (s *Service) ExpenseTypes(ctx context.Context, tenantID int64) []ExpenseType {
	out := DefaultExpenseTypes()
	if s == nil || s.Settings == nil {
		return out
	}
	m, err := s.Settings.PlainByGroup(ctx, tenantID, ExpenseSettingsGroup)
	if err != nil || strings.TrimSpace(m[ExpenseTypesKey]) == "" {
		return out
	}
	var raw []ExpenseType
	if err := json.Unmarshal([]byte(m[ExpenseTypesKey]), &raw); err != nil {
		return out
	}
	seen := map[string]bool{}
	for _, t := range out {
		seen[t.Code] = true
	}
	for _, t := range raw {
		t.Code = strings.TrimSpace(t.Code)
		t.Label = strings.TrimSpace(t.Label)
		if t.Code == "" || t.Label == "" || seen[t.Code] {
			continue
		}
		seen[t.Code] = true
		out = append(out, t)
		if len(out) >= maxExpenseTypes {
			break
		}
	}
	return out
}

func (s *Service) expenseTypeLabels(ctx context.Context, tenantID int64) map[string]string {
	out := map[string]string{}
	for _, t := range s.ExpenseTypes(ctx, tenantID) {
		out[t.Code] = t.Label
	}
	return out
}

func (s *Service) validExpenseType(ctx context.Context, tenantID int64, code string) bool {
	_, ok := s.expenseTypeLabels(ctx, tenantID)[code]
	return ok
}

// loadScopedOrder loads a tenant order and enforces store visibility (orders
// outside the caller's scope answer not-found, no existence leak).
func (s *Service) loadScopedOrder(c *gin.Context, orderID uuid.UUID) (*order.Order, error) {
	tid, err := adminperm.TenantIDFromGin(c)
	if err != nil {
		return nil, err
	}
	var o order.Order
	if err := s.DB.WithContext(c.Request.Context()).
		Where("tenant_id = ?", tid).First(&o, "id = ?", orderID).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	if err := adminperm.EnsureStoreVisible(c, s.DB, o.ShopID); err != nil {
		return nil, ErrNotFound
	}
	return &o, nil
}

// requireOrderOperate enforces per-shop operate permission for write calls.
func (s *Service) requireOrderOperate(c *gin.Context, o *order.Order) error {
	if o.ShopID == nil {
		return nil
	}
	if !adminperm.RequireStoreOperate(c, s.DB, *o.ShopID) {
		return ErrForbidden
	}
	return nil
}

// ---- Payments ----

// PaymentBody is the create payload for one payment record.
type PaymentBody struct {
	OrderID    string  `json:"orderId"`
	Amount     float64 `json:"amount"`
	Currency   string  `json:"currency"`
	FeeAmount  float64 `json:"feeAmount"`
	ReceivedAt string  `json:"receivedAt"` // YYYY-MM-DD
	Channel    string  `json:"channel"`
	Remark     string  `json:"remark"`
}

func parseDay(raw string) (time.Time, error) {
	t, err := time.ParseInLocation("2006-01-02", strings.TrimSpace(raw), time.Local)
	if err != nil {
		return time.Time{}, fmt.Errorf("%w: 日期格式应为 YYYY-MM-DD", ErrBadRequest)
	}
	return t, nil
}

// CreatePayment registers one manual payment record against a scoped order.
func (s *Service) CreatePayment(c *gin.Context, body PaymentBody, adminID *uuid.UUID) (*PaymentRecord, error) {
	oid, err := uuid.Parse(strings.TrimSpace(body.OrderID))
	if err != nil {
		return nil, fmt.Errorf("%w: orderId 无效", ErrBadRequest)
	}
	o, err := s.loadScopedOrder(c, oid)
	if err != nil {
		return nil, err
	}
	if err := s.requireOrderOperate(c, o); err != nil {
		return nil, err
	}
	return s.createPaymentForOrder(c.Request.Context(), o, body, SourceManual, adminID)
}

// createPaymentForOrder persists one validated payment row (shared by the
// manual API and the import-wizard commit path, which scope the order first).
func (s *Service) createPaymentForOrder(ctx context.Context, o *order.Order, body PaymentBody, source string, adminID *uuid.UUID) (*PaymentRecord, error) {
	if body.Amount <= 0 {
		return nil, fmt.Errorf("%w: 回款金额必须大于 0", ErrBadRequest)
	}
	if body.FeeAmount < 0 {
		return nil, fmt.Errorf("%w: 手续费不能为负数", ErrBadRequest)
	}
	if body.FeeAmount > body.Amount {
		return nil, fmt.Errorf("%w: 手续费不能超过回款金额", ErrBadRequest)
	}
	cur := strings.ToUpper(strings.TrimSpace(body.Currency))
	if cur == "" {
		cur = o.Currency
	}
	if !fxrate.ValidCurrencyCode(cur) {
		return nil, fmt.Errorf("%w: 币种代码无效", ErrBadRequest)
	}
	receivedAt, err := parseDay(body.ReceivedAt)
	if err != nil {
		return nil, err
	}
	rec := &PaymentRecord{
		TenantID:   o.TenantID,
		OrderID:    o.ID,
		ShopID:     o.ShopID,
		Amount:     body.Amount,
		Currency:   cur,
		FeeAmount:  body.FeeAmount,
		ReceivedAt: receivedAt,
		Channel:    strings.TrimSpace(body.Channel),
		Remark:     strings.TrimSpace(body.Remark),
		Source:     source,
		CreatedBy:  adminID,
	}
	if err := s.DB.WithContext(ctx).Create(rec).Error; err != nil {
		return nil, err
	}
	return rec, nil
}

// CreatePaymentForImport persists one import-sourced payment row for an
// already-scoped order (the import commit path enforces tenant/store scope
// and operate permission before calling in).
func (s *Service) CreatePaymentForImport(ctx context.Context, o *order.Order, body PaymentBody, adminID *uuid.UUID) (*PaymentRecord, error) {
	return s.createPaymentForOrder(ctx, o, body, SourceImport, adminID)
}

// FindDuplicatePayment reports whether an identical payment row (same order,
// amount, currency and received day) already exists — the import-wizard
// duplicate rule.
func (s *Service) FindDuplicatePayment(ctx context.Context, tenantID int64, orderID uuid.UUID, amount float64, currency string, receivedAt time.Time) (bool, error) {
	var n int64
	dayStart := time.Date(receivedAt.Year(), receivedAt.Month(), receivedAt.Day(), 0, 0, 0, 0, receivedAt.Location())
	err := s.DB.WithContext(ctx).Model(&PaymentRecord{}).
		Where("tenant_id = ? AND order_id = ? AND currency = ? AND received_at >= ? AND received_at < ?",
			tenantID, orderID, strings.ToUpper(strings.TrimSpace(currency)), dayStart, dayStart.AddDate(0, 0, 1)).
		Where("amount = ?", amount).
		Count(&n).Error
	return n > 0, err
}

// DeletePayment removes one payment record (scoped via its order).
func (s *Service) DeletePayment(c *gin.Context, id uuid.UUID, adminID *uuid.UUID) error {
	tid, err := adminperm.TenantIDFromGin(c)
	if err != nil {
		return err
	}
	var rec PaymentRecord
	if err := s.DB.WithContext(c.Request.Context()).
		Where("tenant_id = ?", tid).First(&rec, "id = ?", id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return ErrNotFound
		}
		return err
	}
	if err := adminperm.EnsureStoreVisible(c, s.DB, rec.ShopID); err != nil {
		return ErrNotFound
	}
	if rec.ShopID != nil && !adminperm.RequireStoreOperate(c, s.DB, *rec.ShopID) {
		return ErrForbidden
	}
	if err := s.DB.WithContext(c.Request.Context()).Delete(&rec).Error; err != nil {
		return err
	}
	s.logOp(c, adminID, "finance.payment.delete", rec.ID.String(), rec.OrderID.String())
	return nil
}

// PaymentDTO is one payment list row with its order context.
type PaymentDTO struct {
	PaymentRecord
	OrderNo          string  `json:"orderNo"`
	OrderAmount      float64 `json:"orderAmount"`
	OrderCurrency    string  `json:"orderCurrency"`
	ShopName         string  `json:"shopName,omitempty"`
	SettlementStatus string  `json:"settlementStatus"`
	DiffAmount       float64 `json:"diffAmount"`
}

// ListPaymentsQuery filters the payment list.
type ListPaymentsQuery struct {
	OrderID  string
	ShopID   string
	Status   string
	Page     int
	PageSize int
}

// ListPayments returns tenant + store scoped payment rows (newest first)
// enriched with the per-order settlement status.
func (s *Service) ListPayments(c *gin.Context, q ListPaymentsQuery) ([]PaymentDTO, int64, error) {
	tx := s.DB.WithContext(c.Request.Context()).Model(&PaymentRecord{})
	tx, _, err := adminperm.ApplyTenantScope(c, tx)
	if err != nil {
		return nil, 0, err
	}
	tx, err = adminperm.ApplyStoreScope(c, s.DB, tx, "shop_id")
	if err != nil {
		return nil, 0, err
	}
	if raw := strings.TrimSpace(q.OrderID); raw != "" {
		oid, err := uuid.Parse(raw)
		if err != nil {
			return nil, 0, fmt.Errorf("%w: orderId 无效", ErrBadRequest)
		}
		tx = tx.Where("order_id = ?", oid)
	}
	if raw := strings.TrimSpace(q.ShopID); raw != "" {
		sid, err := uuid.Parse(raw)
		if err != nil {
			return nil, 0, fmt.Errorf("%w: shopId 无效", ErrBadRequest)
		}
		tx = tx.Where("shop_id = ?", sid)
	}
	var total int64
	if err := tx.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	page, pageSize := normPage(q.Page, q.PageSize)
	var recs []PaymentRecord
	if err := tx.Order("received_at DESC, created_at DESC").
		Offset((page - 1) * pageSize).Limit(pageSize).Find(&recs).Error; err != nil {
		return nil, 0, err
	}
	return s.enrichPayments(c, recs, q.Status, total)
}

func normPage(page, pageSize int) (int, int) {
	if page <= 0 {
		page = 1
	}
	if pageSize <= 0 || pageSize > 200 {
		pageSize = 20
	}
	return page, pageSize
}

func (s *Service) enrichPayments(c *gin.Context, recs []PaymentRecord, statusFilter string, total int64) ([]PaymentDTO, int64, error) {
	ctx := c.Request.Context()
	orderIDs := make([]uuid.UUID, 0, len(recs))
	seen := map[uuid.UUID]bool{}
	for _, r := range recs {
		if !seen[r.OrderID] {
			seen[r.OrderID] = true
			orderIDs = append(orderIDs, r.OrderID)
		}
	}
	orders, err := s.ordersByID(ctx, orderIDs)
	if err != nil {
		return nil, 0, err
	}
	sums, err := s.paymentSums(ctx, orderIDs)
	if err != nil {
		return nil, 0, err
	}
	shopNames, err := s.shopNames(ctx, recs)
	if err != nil {
		return nil, 0, err
	}
	out := make([]PaymentDTO, 0, len(recs))
	for _, r := range recs {
		dto := PaymentDTO{PaymentRecord: r}
		if o, ok := orders[r.OrderID]; ok {
			dto.OrderNo = o.OrderNo
			dto.OrderAmount = o.TotalAmount
			dto.OrderCurrency = o.Currency
			dto.SettlementStatus, dto.DiffAmount = settlementOf(o.TotalAmount, sums[r.OrderID])
		}
		if r.ShopID != nil {
			dto.ShopName = shopNames[*r.ShopID]
		}
		if f := strings.TrimSpace(statusFilter); f != "" && dto.SettlementStatus != f {
			continue
		}
		out = append(out, dto)
	}
	if strings.TrimSpace(statusFilter) != "" {
		total = int64(len(out))
	}
	return out, total, nil
}

// settlementOf derives the settlement status from the receivable and the
// summed received amount (both in the order currency).
func settlementOf(receivable, received float64) (string, float64) {
	diff := fxrate.Round2(new(big.Rat).Sub(fxrate.AmountRat(received), fxrate.AmountRat(receivable)))
	switch {
	case received == 0:
		return SettlementUnpaid, diff
	case diff > SettlementTolerance:
		return SettlementOver, diff
	case diff < -SettlementTolerance:
		return SettlementShort, diff
	default:
		return SettlementSettled, diff
	}
}

func (s *Service) ordersByID(ctx context.Context, ids []uuid.UUID) (map[uuid.UUID]order.Order, error) {
	out := map[uuid.UUID]order.Order{}
	if len(ids) == 0 {
		return out, nil
	}
	var rows []order.Order
	if err := s.DB.WithContext(ctx).
		Select("id, order_no, currency, total_amount, shop_id, platform, payment_status, created_at").
		Where("id IN ?", ids).Find(&rows).Error; err != nil {
		return nil, err
	}
	for _, o := range rows {
		out[o.ID] = o
	}
	return out, nil
}

// paymentSums sums received amounts per order (order-currency口径: rows in a
// different currency than the order are still summed as-is and surface in
// the差异 workbench for manual review).
func (s *Service) paymentSums(ctx context.Context, orderIDs []uuid.UUID) (map[uuid.UUID]float64, error) {
	out := map[uuid.UUID]float64{}
	if len(orderIDs) == 0 {
		return out, nil
	}
	type row struct {
		OrderID uuid.UUID `gorm:"column:order_id"`
		Total   float64   `gorm:"column:total"`
	}
	var rows []row
	if err := s.DB.WithContext(ctx).Model(&PaymentRecord{}).
		Select("order_id, SUM(amount) AS total").
		Where("order_id IN ? AND deleted_at IS NULL", orderIDs).
		Group("order_id").Scan(&rows).Error; err != nil {
		return nil, err
	}
	for _, r := range rows {
		out[r.OrderID] = r.Total
	}
	return out, nil
}

func (s *Service) shopNames(ctx context.Context, recs []PaymentRecord) (map[uuid.UUID]string, error) {
	ids := []uuid.UUID{}
	seen := map[uuid.UUID]bool{}
	for _, r := range recs {
		if r.ShopID != nil && !seen[*r.ShopID] {
			seen[*r.ShopID] = true
			ids = append(ids, *r.ShopID)
		}
	}
	return s.shopNamesByID(ctx, ids)
}

func (s *Service) shopNamesByID(ctx context.Context, ids []uuid.UUID) (map[uuid.UUID]string, error) {
	out := map[uuid.UUID]string{}
	if len(ids) == 0 {
		return out, nil
	}
	type row struct {
		ID   uuid.UUID `gorm:"column:id"`
		Name string    `gorm:"column:name"`
	}
	var rows []row
	if err := s.DB.WithContext(ctx).Table("shops").
		Select("id, shop_name AS name").Where("id IN ?", ids).Scan(&rows).Error; err != nil {
		return nil, err
	}
	for _, r := range rows {
		out[r.ID] = strings.TrimSpace(r.Name)
	}
	return out, nil
}

// ---- Order expenses ----

// OrderExpenseBody is the create payload for one order-level expense.
type OrderExpenseBody struct {
	OrderID    string  `json:"orderId"`
	TypeCode   string  `json:"typeCode"`
	Amount     float64 `json:"amount"`
	Currency   string  `json:"currency"`
	IncurredAt string  `json:"incurredAt"` // optional YYYY-MM-DD
	Remark     string  `json:"remark"`
}

// CreateOrderExpense registers one order-level expense entry.
func (s *Service) CreateOrderExpense(c *gin.Context, body OrderExpenseBody, adminID *uuid.UUID) (*OrderExpense, error) {
	oid, err := uuid.Parse(strings.TrimSpace(body.OrderID))
	if err != nil {
		return nil, fmt.Errorf("%w: orderId 无效", ErrBadRequest)
	}
	o, err := s.loadScopedOrder(c, oid)
	if err != nil {
		return nil, err
	}
	if err := s.requireOrderOperate(c, o); err != nil {
		return nil, err
	}
	if body.Amount <= 0 {
		return nil, fmt.Errorf("%w: 费用金额必须大于 0", ErrBadRequest)
	}
	code := strings.TrimSpace(body.TypeCode)
	if !s.validExpenseType(c.Request.Context(), o.TenantID, code) {
		return nil, fmt.Errorf("%w: 费用类型无效", ErrBadRequest)
	}
	cur := strings.ToUpper(strings.TrimSpace(body.Currency))
	if cur == "" {
		cur = o.Currency
	}
	if !fxrate.ValidCurrencyCode(cur) {
		return nil, fmt.Errorf("%w: 币种代码无效", ErrBadRequest)
	}
	exp := &OrderExpense{
		TenantID:  o.TenantID,
		OrderID:   o.ID,
		ShopID:    o.ShopID,
		TypeCode:  code,
		Amount:    body.Amount,
		Currency:  cur,
		Remark:    strings.TrimSpace(body.Remark),
		CreatedBy: adminID,
	}
	if raw := strings.TrimSpace(body.IncurredAt); raw != "" {
		t, err := parseDay(raw)
		if err != nil {
			return nil, err
		}
		exp.IncurredAt = &t
	}
	if err := s.DB.WithContext(c.Request.Context()).Create(exp).Error; err != nil {
		return nil, err
	}
	s.logOp(c, adminID, "finance.expense.create", exp.ID.String(), code)
	return exp, nil
}

// DeleteOrderExpense removes one order-level expense entry.
func (s *Service) DeleteOrderExpense(c *gin.Context, id uuid.UUID, adminID *uuid.UUID) error {
	tid, err := adminperm.TenantIDFromGin(c)
	if err != nil {
		return err
	}
	var exp OrderExpense
	if err := s.DB.WithContext(c.Request.Context()).
		Where("tenant_id = ?", tid).First(&exp, "id = ?", id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return ErrNotFound
		}
		return err
	}
	if err := adminperm.EnsureStoreVisible(c, s.DB, exp.ShopID); err != nil {
		return ErrNotFound
	}
	if exp.ShopID != nil && !adminperm.RequireStoreOperate(c, s.DB, *exp.ShopID) {
		return ErrForbidden
	}
	if err := s.DB.WithContext(c.Request.Context()).Delete(&exp).Error; err != nil {
		return err
	}
	s.logOp(c, adminID, "finance.expense.delete", exp.ID.String(), exp.OrderID.String())
	return nil
}

// ---- Shop monthly expenses ----

// ShopExpenseBody is the create payload for one shop-monthly expense.
type ShopExpenseBody struct {
	ShopID   string  `json:"shopId"`
	Month    string  `json:"month"` // YYYY-MM
	TypeCode string  `json:"typeCode"`
	Amount   float64 `json:"amount"`
	Currency string  `json:"currency"`
	Remark   string  `json:"remark"`
}

func validMonth(raw string) bool {
	_, err := time.Parse("2006-01", strings.TrimSpace(raw))
	return err == nil
}

// CreateShopExpense registers one shop-level monthly expense entry.
func (s *Service) CreateShopExpense(c *gin.Context, body ShopExpenseBody, adminID *uuid.UUID) (*ShopMonthlyExpense, error) {
	sid, err := uuid.Parse(strings.TrimSpace(body.ShopID))
	if err != nil {
		return nil, fmt.Errorf("%w: shopId 无效", ErrBadRequest)
	}
	tid, err := adminperm.TenantIDFromGin(c)
	if err != nil {
		return nil, err
	}
	if !adminperm.RequireStoreView(c, s.DB, sid) {
		return nil, ErrNotFound
	}
	if !adminperm.RequireStoreOperate(c, s.DB, sid) {
		return nil, ErrForbidden
	}
	var n int64
	if err := s.DB.WithContext(c.Request.Context()).Table("shops").
		Where("id = ? AND tenant_id = ? AND deleted_at IS NULL", sid, tid).Count(&n).Error; err != nil {
		return nil, err
	}
	if n == 0 {
		return nil, ErrNotFound
	}
	if !validMonth(body.Month) {
		return nil, fmt.Errorf("%w: 月份格式应为 YYYY-MM", ErrBadRequest)
	}
	if body.Amount <= 0 {
		return nil, fmt.Errorf("%w: 费用金额必须大于 0", ErrBadRequest)
	}
	code := strings.TrimSpace(body.TypeCode)
	if !s.validExpenseType(c.Request.Context(), tid, code) {
		return nil, fmt.Errorf("%w: 费用类型无效", ErrBadRequest)
	}
	cur := strings.ToUpper(strings.TrimSpace(body.Currency))
	if cur == "" {
		cur = "CNY"
	}
	if !fxrate.ValidCurrencyCode(cur) {
		return nil, fmt.Errorf("%w: 币种代码无效", ErrBadRequest)
	}
	exp := &ShopMonthlyExpense{
		TenantID:  tid,
		ShopID:    sid,
		Month:     strings.TrimSpace(body.Month),
		TypeCode:  code,
		Amount:    body.Amount,
		Currency:  cur,
		Remark:    strings.TrimSpace(body.Remark),
		CreatedBy: adminID,
	}
	if err := s.DB.WithContext(c.Request.Context()).Create(exp).Error; err != nil {
		return nil, err
	}
	s.logOp(c, adminID, "finance.shop_expense.create", exp.ID.String(), body.Month)
	return exp, nil
}

// DeleteShopExpense removes one shop-monthly expense entry.
func (s *Service) DeleteShopExpense(c *gin.Context, id uuid.UUID, adminID *uuid.UUID) error {
	tid, err := adminperm.TenantIDFromGin(c)
	if err != nil {
		return err
	}
	var exp ShopMonthlyExpense
	if err := s.DB.WithContext(c.Request.Context()).
		Where("tenant_id = ?", tid).First(&exp, "id = ?", id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return ErrNotFound
		}
		return err
	}
	if !adminperm.RequireStoreView(c, s.DB, exp.ShopID) {
		return ErrNotFound
	}
	if !adminperm.RequireStoreOperate(c, s.DB, exp.ShopID) {
		return ErrForbidden
	}
	if err := s.DB.WithContext(c.Request.Context()).Delete(&exp).Error; err != nil {
		return err
	}
	s.logOp(c, adminID, "finance.shop_expense.delete", exp.ID.String(), exp.Month)
	return nil
}

// ShopExpenseDTO is one shop-monthly expense list row.
type ShopExpenseDTO struct {
	ShopMonthlyExpense
	ShopName  string `json:"shopName,omitempty"`
	TypeLabel string `json:"typeLabel,omitempty"`
}

// ListShopExpenses returns tenant + store scoped shop-monthly expenses.
func (s *Service) ListShopExpenses(c *gin.Context, shopID, month string, page, pageSize int) ([]ShopExpenseDTO, int64, error) {
	tx := s.DB.WithContext(c.Request.Context()).Model(&ShopMonthlyExpense{})
	tx, tid, err := adminperm.ApplyTenantScope(c, tx)
	if err != nil {
		return nil, 0, err
	}
	tx, err = adminperm.ApplyStoreScope(c, s.DB, tx, "shop_id")
	if err != nil {
		return nil, 0, err
	}
	if raw := strings.TrimSpace(shopID); raw != "" {
		sid, err := uuid.Parse(raw)
		if err != nil {
			return nil, 0, fmt.Errorf("%w: shopId 无效", ErrBadRequest)
		}
		tx = tx.Where("shop_id = ?", sid)
	}
	if raw := strings.TrimSpace(month); raw != "" {
		if !validMonth(raw) {
			return nil, 0, fmt.Errorf("%w: 月份格式应为 YYYY-MM", ErrBadRequest)
		}
		tx = tx.Where("month = ?", raw)
	}
	var total int64
	if err := tx.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	page, pageSize = normPage(page, pageSize)
	var rows []ShopMonthlyExpense
	if err := tx.Order("month DESC, created_at DESC").
		Offset((page - 1) * pageSize).Limit(pageSize).Find(&rows).Error; err != nil {
		return nil, 0, err
	}
	ids := []uuid.UUID{}
	seen := map[uuid.UUID]bool{}
	for _, r := range rows {
		if !seen[r.ShopID] {
			seen[r.ShopID] = true
			ids = append(ids, r.ShopID)
		}
	}
	names, err := s.shopNamesByID(c.Request.Context(), ids)
	if err != nil {
		return nil, 0, err
	}
	labels := s.expenseTypeLabels(c.Request.Context(), tid)
	out := make([]ShopExpenseDTO, 0, len(rows))
	for _, r := range rows {
		out = append(out, ShopExpenseDTO{ShopMonthlyExpense: r, ShopName: names[r.ShopID], TypeLabel: labels[r.TypeCode]})
	}
	return out, total, nil
}

// sortedCurrencies returns map keys in stable order.
func sortedKeys[T any](m map[string]T) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

func (s *Service) logOp(c *gin.Context, adminID *uuid.UUID, action, resourceID, message string) {
	if s.OpLog == nil {
		return
	}
	_ = s.OpLog.Write(c, operationlog.WriteOpts{
		AdminUserID: adminID,
		Action:      action,
		Resource:    "finance",
		ResourceID:  resourceID,
		Status:      "success",
		Message:     message,
	})
}
