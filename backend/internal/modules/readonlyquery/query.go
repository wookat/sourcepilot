// Package readonlyquery holds the tenant-scoped, masked, read-only queries
// shared by the MCP entry (POST /api/mcp) and the Open API entry
// (/api/open/v1/*). Every query enforces the tenant scope, performs no write,
// and its outputs exclude secrets, credentials, contact details and internal
// UUIDs; customer names keep only their first character.
package readonlyquery

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/trademind-ai/trademind/backend/internal/modules/order"
	"github.com/trademind-ai/trademind/backend/internal/modules/orderexception"
	"gorm.io/gorm"
)

// MaxPageSize caps one page of results for every query.
const MaxPageSize = 100

// ErrOrderNotFound indicates the order number does not exist within the
// tenant scope (cross-tenant lookups answer identically).
var ErrOrderNotFound = errors.New("order not found")

// ErrBadInput marks caller-supplied parameter errors (answered as 400 by the
// Open API entry, as a tool error by MCP).
var ErrBadInput = errors.New("bad input")

// Service executes the shared read-only queries.
type Service struct {
	DB *gorm.DB
	// Exceptions backs the exception queries; nil disables them.
	Exceptions *orderexception.Service
}

// NormPage clamps pagination inputs (default page 1, size 20, max 100).
func NormPage(page, pageSize int) (int, int) {
	if page < 1 {
		page = 1
	}
	if pageSize < 1 {
		pageSize = 20
	}
	if pageSize > MaxPageSize {
		pageSize = MaxPageSize
	}
	return page, pageSize
}

// MaskName keeps the first rune of a customer name and hides the rest.
func MaskName(name string) string {
	r := []rune(strings.TrimSpace(name))
	if len(r) == 0 {
		return ""
	}
	return string(r[0]) + "**"
}

// ParseEnum accepts an empty string or one of the allowed values; anything
// else is ErrBadInput (no silent empty-result degradation).
func ParseEnum(name, s string, allowed ...string) (string, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return "", nil
	}
	for _, a := range allowed {
		if s == a {
			return s, nil
		}
	}
	return "", fmt.Errorf("%w: invalid %s %q (want one of %s)", ErrBadInput, name, s, strings.Join(allowed, "/"))
}

// ParseDate accepts YYYY-MM-DD or an empty string (nil).
func ParseDate(s string) (*time.Time, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return nil, nil
	}
	t, err := time.Parse("2006-01-02", s)
	if err != nil {
		return nil, fmt.Errorf("%w: invalid date %q (want YYYY-MM-DD)", ErrBadInput, s)
	}
	return &t, nil
}

// OrdersQueryIn filters the order list query.
type OrdersQueryIn struct {
	Status        string `json:"status,omitempty" jsonschema:"订单状态过滤（pending/paid/processing/shipped/delivered/cancelled/refunded/closed）"`
	PaymentStatus string `json:"paymentStatus,omitempty" jsonschema:"支付状态过滤（unpaid/paid/partially_refunded/refunded）"`
	Platform      string `json:"platform,omitempty" jsonschema:"平台过滤（如 douyin）"`
	Keyword       string `json:"keyword,omitempty" jsonschema:"按订单号模糊匹配"`
	StartDate     string `json:"startDate,omitempty" jsonschema:"创建时间起（YYYY-MM-DD，含当日）"`
	EndDate       string `json:"endDate,omitempty" jsonschema:"创建时间止（YYYY-MM-DD，含当日）"`
	Page          int    `json:"page,omitempty" jsonschema:"页码，默认 1"`
	PageSize      int    `json:"pageSize,omitempty" jsonschema:"每页条数，默认 20，最大 100"`
}

// OrderSummary is one masked order row.
type OrderSummary struct {
	OrderNo           string  `json:"orderNo"`
	Platform          string  `json:"platform"`
	Status            string  `json:"status"`
	PaymentStatus     string  `json:"paymentStatus"`
	FulfillmentStatus string  `json:"fulfillmentStatus"`
	Currency          string  `json:"currency"`
	TotalAmount       float64 `json:"totalAmount"`
	CustomerName      string  `json:"customerName,omitempty"`
	CreatedAt         string  `json:"createdAt"`
}

// OrdersQueryOut is the order list payload.
type OrdersQueryOut struct {
	Total int64          `json:"total"`
	Items []OrderSummary `json:"items"`
}

// OrdersQuery lists masked order summaries within the tenant scope.
func (s *Service) OrdersQuery(ctx context.Context, tenantID int64, in OrdersQueryIn) (OrdersQueryOut, error) {
	var out OrdersQueryOut
	page, ps := NormPage(in.Page, in.PageSize)
	start, err := ParseDate(in.StartDate)
	if err != nil {
		return out, err
	}
	end, err := ParseDate(in.EndDate)
	if err != nil {
		return out, err
	}
	status, err := ParseEnum("status", in.Status,
		order.StatusPending, order.StatusPaid, order.StatusProcessing, order.StatusShipped,
		order.StatusDelivered, order.StatusCancelled, order.StatusRefunded, order.StatusClosed)
	if err != nil {
		return out, err
	}
	payStatus, err := ParseEnum("paymentStatus", in.PaymentStatus,
		order.PaymentUnpaid, order.PaymentPaid, order.PaymentPartiallyRefunded, order.PaymentRefunded)
	if err != nil {
		return out, err
	}
	tx := s.DB.WithContext(ctx).Model(&order.Order{}).Where("tenant_id = ?", tenantID)
	if status != "" {
		tx = tx.Where("status = ?", status)
	}
	if payStatus != "" {
		tx = tx.Where("payment_status = ?", payStatus)
	}
	if v := strings.TrimSpace(in.Platform); v != "" {
		tx = tx.Where("platform = ?", v)
	}
	if v := strings.TrimSpace(in.Keyword); v != "" {
		tx = tx.Where("order_no ILIKE ?", "%"+v+"%")
	}
	if start != nil {
		tx = tx.Where("created_at >= ?", *start)
	}
	if end != nil {
		tx = tx.Where("created_at < ?", end.Add(24*time.Hour))
	}
	if err := tx.Count(&out.Total).Error; err != nil {
		return out, fmt.Errorf("orders_query: %w", err)
	}
	var rows []order.Order
	if err := tx.Order("created_at DESC").Offset((page - 1) * ps).Limit(ps).Find(&rows).Error; err != nil {
		return out, fmt.Errorf("orders_query: %w", err)
	}
	out.Items = make([]OrderSummary, 0, len(rows))
	for _, r := range rows {
		out.Items = append(out.Items, orderSummaryOf(r))
	}
	return out, nil
}

func orderSummaryOf(r order.Order) OrderSummary {
	return OrderSummary{
		OrderNo:           r.OrderNo,
		Platform:          r.Platform,
		Status:            r.Status,
		PaymentStatus:     r.PaymentStatus,
		FulfillmentStatus: r.FulfillmentStatus,
		Currency:          r.Currency,
		TotalAmount:       r.TotalAmount,
		CustomerName:      MaskName(r.CustomerName),
		CreatedAt:         r.CreatedAt.UTC().Format(time.RFC3339),
	}
}

// OrderItemSummary is one masked order line item.
type OrderItemSummary struct {
	ProductTitle string  `json:"productTitle"`
	SKUName      string  `json:"skuName,omitempty"`
	SKUCode      string  `json:"skuCode,omitempty"`
	Quantity     int     `json:"quantity"`
	UnitPrice    float64 `json:"unitPrice"`
	TotalPrice   float64 `json:"totalPrice"`
}

// OrderDetailOut is one masked order with its line items.
type OrderDetailOut struct {
	OrderSummary
	PaidAt    string             `json:"paidAt,omitempty"`
	ShippedAt string             `json:"shippedAt,omitempty"`
	Items     []OrderItemSummary `json:"items"`
}

// OrderDetail loads one order by order number within the tenant scope.
// Unknown and cross-tenant order numbers both return ErrOrderNotFound.
func (s *Service) OrderDetail(ctx context.Context, tenantID int64, orderNo string) (OrderDetailOut, error) {
	var out OrderDetailOut
	orderNo = strings.TrimSpace(orderNo)
	if orderNo == "" {
		return out, ErrOrderNotFound
	}
	var row order.Order
	err := s.DB.WithContext(ctx).Model(&order.Order{}).
		Where("tenant_id = ? AND order_no = ?", tenantID, orderNo).
		Preload("Items").
		First(&row).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return out, ErrOrderNotFound
		}
		return out, fmt.Errorf("order_detail: %w", err)
	}
	out.OrderSummary = orderSummaryOf(row)
	if row.PaidAt != nil {
		out.PaidAt = row.PaidAt.UTC().Format(time.RFC3339)
	}
	if row.ShippedAt != nil {
		out.ShippedAt = row.ShippedAt.UTC().Format(time.RFC3339)
	}
	out.Items = make([]OrderItemSummary, 0, len(row.Items))
	for _, it := range row.Items {
		out.Items = append(out.Items, OrderItemSummary{
			ProductTitle: it.ProductTitle,
			SKUName:      it.SKUName,
			SKUCode:      it.SKUCode,
			Quantity:     it.Quantity,
			UnitPrice:    it.UnitPrice,
			TotalPrice:   it.TotalPrice,
		})
	}
	return out, nil
}

// InventoryQueryIn filters the SKU stock query.
type InventoryQueryIn struct {
	Keyword      string `json:"keyword,omitempty" jsonschema:"按 SKU 编码 / SKU 名称 / 商品标题模糊匹配"`
	LowStockOnly bool   `json:"lowStockOnly,omitempty" jsonschema:"仅返回库存不高于预警线的 SKU"`
	Page         int    `json:"page,omitempty" jsonschema:"页码，默认 1"`
	PageSize     int    `json:"pageSize,omitempty" jsonschema:"每页条数，默认 20，最大 100"`
}

// InventoryItem is one SKU stock row.
type InventoryItem struct {
	SKUCode      string `json:"skuCode"`
	SKUName      string `json:"skuName,omitempty"`
	ProductTitle string `json:"productTitle"`
	Stock        *int   `json:"stock,omitempty"`
	WarningStock int    `json:"warningStock"`
	StockStatus  string `json:"stockStatus,omitempty"`
}

// InventoryQueryOut is the SKU stock payload.
type InventoryQueryOut struct {
	Total int64           `json:"total"`
	Items []InventoryItem `json:"items"`
}

type skuRow struct {
	SKUCode      string
	SKUName      string
	ProductTitle string
	Stock        *int
	WarningStock int
	StockStatus  string
}

// InventoryQuery lists SKU stock rows within the tenant scope.
func (s *Service) InventoryQuery(ctx context.Context, tenantID int64, in InventoryQueryIn) (InventoryQueryOut, error) {
	var out InventoryQueryOut
	page, ps := NormPage(in.Page, in.PageSize)
	tx := s.DB.WithContext(ctx).Table("product_skus").
		Joins("JOIN products ON products.id = product_skus.product_id AND products.deleted_at IS NULL").
		Where("products.tenant_id = ?", tenantID)
	if v := strings.TrimSpace(in.Keyword); v != "" {
		like := "%" + v + "%"
		tx = tx.Where("product_skus.sku_code ILIKE ? OR product_skus.sku_name ILIKE ? OR products.title ILIKE ?", like, like, like)
	}
	if in.LowStockOnly {
		tx = tx.Where("product_skus.stock IS NOT NULL AND product_skus.stock <= product_skus.warning_stock")
	}
	if err := tx.Count(&out.Total).Error; err != nil {
		return out, fmt.Errorf("inventory_query: %w", err)
	}
	var rows []skuRow
	if err := tx.Select("product_skus.sku_code AS sku_code, product_skus.sku_name AS sku_name, products.title AS product_title, product_skus.stock AS stock, product_skus.warning_stock AS warning_stock, product_skus.stock_status AS stock_status").
		Order("product_skus.created_at DESC").
		Offset((page - 1) * ps).Limit(ps).
		Scan(&rows).Error; err != nil {
		return out, fmt.Errorf("inventory_query: %w", err)
	}
	out.Items = make([]InventoryItem, 0, len(rows))
	for _, r := range rows {
		out.Items = append(out.Items, InventoryItem{
			SKUCode:      r.SKUCode,
			SKUName:      r.SKUName,
			ProductTitle: r.ProductTitle,
			Stock:        r.Stock,
			WarningStock: r.WarningStock,
			StockStatus:  r.StockStatus,
		})
	}
	return out, nil
}

// ReportSummaryIn bounds the business summary query.
type ReportSummaryIn struct {
	StartDate string `json:"startDate,omitempty" jsonschema:"统计起始日（YYYY-MM-DD，含当日；默认近 30 天）"`
	EndDate   string `json:"endDate,omitempty" jsonschema:"统计截止日（YYYY-MM-DD，含当日；默认今天）"`
}

// CurrencySales is paid sales aggregated per currency (no FX conversion).
type CurrencySales struct {
	Currency   string  `json:"currency"`
	PaidAmount float64 `json:"paidAmount"`
	OrderCount int64   `json:"orderCount"`
}

// ReportSummaryOut is the business summary payload.
type ReportSummaryOut struct {
	StartDate          string          `json:"startDate"`
	EndDate            string          `json:"endDate"`
	OrderCount         int64           `json:"orderCount"`
	PaidOrderCount     int64           `json:"paidOrderCount"`
	SalesByCurrency    []CurrencySales `json:"salesByCurrency"`
	OpenExceptionCount int64           `json:"openExceptionCount"`
	LowStockSKUCount   int64           `json:"lowStockSkuCount"`
}

// ReportSummary aggregates order, sales, exception and low-stock counters
// within the tenant scope. Amounts stay in their original currency.
func (s *Service) ReportSummary(ctx context.Context, tenantID int64, in ReportSummaryIn) (ReportSummaryOut, error) {
	var out ReportSummaryOut
	start, err := ParseDate(in.StartDate)
	if err != nil {
		return out, err
	}
	end, err := ParseDate(in.EndDate)
	if err != nil {
		return out, err
	}
	now := time.Now().UTC().Truncate(24 * time.Hour)
	if end == nil {
		end = &now
	}
	if start == nil {
		s := end.Add(-29 * 24 * time.Hour)
		start = &s
	}
	endExcl := end.Add(24 * time.Hour)
	out.StartDate = start.Format("2006-01-02")
	out.EndDate = end.Format("2006-01-02")

	base := func() *gorm.DB {
		return s.DB.WithContext(ctx).Model(&order.Order{}).
			Where("tenant_id = ?", tenantID).
			Where("created_at >= ? AND created_at < ?", *start, endExcl)
	}
	if err := base().Count(&out.OrderCount).Error; err != nil {
		return out, fmt.Errorf("report_summary: %w", err)
	}
	if err := base().Where("payment_status = ?", order.PaymentPaid).Count(&out.PaidOrderCount).Error; err != nil {
		return out, fmt.Errorf("report_summary: %w", err)
	}
	var sales []CurrencySales
	if err := base().Where("payment_status = ?", order.PaymentPaid).
		Select("currency AS currency, COALESCE(SUM(total_amount),0) AS paid_amount, COUNT(*) AS order_count").
		Group("currency").Order("currency").
		Scan(&sales).Error; err != nil {
		return out, fmt.Errorf("report_summary: %w", err)
	}
	out.SalesByCurrency = sales
	if out.SalesByCurrency == nil {
		out.SalesByCurrency = []CurrencySales{}
	}

	if s.Exceptions != nil {
		sum, err := s.Exceptions.SummaryOpenExceptions(ctx, orderexception.ListOrderExceptionsRequest{
			TenantID: &tenantID,
		})
		if err != nil {
			return out, fmt.Errorf("report_summary: exceptions: %w", err)
		}
		out.OpenExceptionCount = sum.TotalOpen
	}

	if err := s.DB.WithContext(ctx).Table("product_skus").
		Joins("JOIN products ON products.id = product_skus.product_id AND products.deleted_at IS NULL").
		Where("products.tenant_id = ?", tenantID).
		Where("product_skus.stock IS NOT NULL AND product_skus.stock <= product_skus.warning_stock").
		Count(&out.LowStockSKUCount).Error; err != nil {
		return out, fmt.Errorf("report_summary: %w", err)
	}
	return out, nil
}

// ExceptionsPendingIn filters the pending exception query.
type ExceptionsPendingIn struct {
	ExceptionType string `json:"exceptionType,omitempty" jsonschema:"异常类型过滤（如 sku_unmatched/insufficient_stock）"`
	Severity      string `json:"severity,omitempty" jsonschema:"级别过滤（low/medium/high/critical）"`
	Page          int    `json:"page,omitempty" jsonschema:"页码，默认 1"`
	PageSize      int    `json:"pageSize,omitempty" jsonschema:"每页条数，默认 20，最大 100"`
}

// ExceptionItem is one masked pending exception row.
type ExceptionItem struct {
	ExceptionType   string `json:"exceptionType"`
	Severity        string `json:"severity"`
	Status          string `json:"status"`
	OrderNo         string `json:"orderNo,omitempty"`
	Platform        string `json:"platform,omitempty"`
	ShopName        string `json:"shopName,omitempty"`
	SKUCode         string `json:"skuCode,omitempty"`
	ProductTitle    string `json:"productTitle,omitempty"`
	Quantity        int    `json:"quantity,omitempty"`
	ErrorMessage    string `json:"errorMessage,omitempty"`
	SuggestedAction string `json:"suggestedAction,omitempty"`
	CreatedAt       string `json:"createdAt"`
}

// ExceptionsPendingOut is the pending exception payload.
type ExceptionsPendingOut struct {
	Total     int64           `json:"total"`
	TotalOpen int64           `json:"totalOpen"`
	Items     []ExceptionItem `json:"items"`
}

// ExceptionsPending lists pending exceptions within the tenant scope.
func (s *Service) ExceptionsPending(ctx context.Context, tenantID int64, in ExceptionsPendingIn) (ExceptionsPendingOut, error) {
	var out ExceptionsPendingOut
	page, ps := NormPage(in.Page, in.PageSize)
	excType, err := ParseEnum("exceptionType", in.ExceptionType,
		orderexception.TypeSKUUnmatched, orderexception.TypeSKUAmbiguous,
		orderexception.TypeInsufficientStock, orderexception.TypeInventoryDeductFailed,
		orderexception.TypeInventoryRestoreFailed, orderexception.TypeInventorySyncFailed,
		orderexception.TypeOrderSyncPartialFailed, orderexception.TypeMissingOrderItem,
		orderexception.TypeProcurementBlocked, orderexception.TypeNegativeMargin,
		orderexception.TypeUnknown)
	if err != nil {
		return out, err
	}
	severity, err := ParseEnum("severity", in.Severity,
		orderexception.SeverityLow, orderexception.SeverityMedium,
		orderexception.SeverityHigh, orderexception.SeverityCritical)
	if err != nil {
		return out, err
	}
	if s.Exceptions == nil {
		return out, fmt.Errorf("exceptions_pending: unavailable")
	}
	res, err := s.Exceptions.ListOrderExceptions(ctx, orderexception.ListOrderExceptionsRequest{
		ExceptionType: excType,
		Severity:      severity,
		Page:          page,
		PageSize:      ps,
		TenantID:      &tenantID,
	})
	if err != nil {
		return out, fmt.Errorf("exceptions_pending: %w", err)
	}
	out.Total = res.Total
	out.TotalOpen = res.Summary.TotalOpen
	out.Items = make([]ExceptionItem, 0, len(res.List))
	for _, r := range res.List {
		out.Items = append(out.Items, ExceptionItem{
			ExceptionType:   r.ExceptionType,
			Severity:        r.Severity,
			Status:          r.Status,
			OrderNo:         r.OrderNo,
			Platform:        r.Platform,
			ShopName:        r.ShopName,
			SKUCode:         r.SKUCode,
			ProductTitle:    r.ProductTitle,
			Quantity:        r.Quantity,
			ErrorMessage:    r.ErrorMessage,
			SuggestedAction: r.SuggestedAction,
			CreatedAt:       r.CreatedAt.UTC().Format(time.RFC3339),
		})
	}
	return out, nil
}
