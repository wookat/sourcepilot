package order

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/trademind-ai/trademind/backend/internal/modules/operationlog"
	"github.com/trademind-ai/trademind/backend/internal/pkg/adminperm"
	"gorm.io/datatypes"
	"gorm.io/gorm"
)

// ErrReviewRuleNotFound is returned for missing / cross-tenant rules (404).
var ErrReviewRuleNotFound = errors.New("order review rule not found")

// ReviewRuleBody is the create / update payload for an order review rule.
type ReviewRuleBody struct {
	Name                     string    `json:"name"`
	Priority                 *int      `json:"priority"`
	Enabled                  *bool     `json:"enabled"`
	Action                   string    `json:"action"`
	MinAmount                *float64  `json:"minAmount"`
	MaxAmount                *float64  `json:"maxAmount"`
	AddressKeywords          *[]string `json:"addressKeywords"`
	RemarkKeywords           *[]string `json:"remarkKeywords"`
	Platforms                *[]string `json:"platforms"`
	ShopIDs                  *[]string `json:"shopIds"`
	MaxTotalQuantity         *int      `json:"maxTotalQuantity"`
	MaxSKUQuantity           *int      `json:"maxSkuQuantity"`
	RepeatReceiverMinOrders  *int      `json:"repeatReceiverMinOrders"`
	RepeatReceiverWindowDays *int      `json:"repeatReceiverWindowDays"`
	ClearMinAmount           bool      `json:"clearMinAmount,omitempty"`
	ClearMaxAmount           bool      `json:"clearMaxAmount,omitempty"`
	ClearMaxTotalQuantity    bool      `json:"clearMaxTotalQuantity,omitempty"`
	ClearMaxSKUQuantity      bool      `json:"clearMaxSkuQuantity,omitempty"`
	ClearRepeatReceiver      bool      `json:"clearRepeatReceiver,omitempty"`
}

func validReviewAction(a string) bool {
	for _, v := range ValidReviewActions() {
		if v == a {
			return true
		}
	}
	return false
}

func validateReviewAmounts(minV, maxV *float64) error {
	if minV != nil && *minV < 0 {
		return fmt.Errorf("金额下限不能为负数")
	}
	if maxV != nil && *maxV < 0 {
		return fmt.Errorf("金额上限不能为负数")
	}
	if minV != nil && maxV != nil && *minV > *maxV {
		return fmt.Errorf("金额下限不能大于上限")
	}
	return nil
}

func validatePositive(v *int, label string) error {
	if v != nil && *v < 1 {
		return fmt.Errorf("%s必须大于 0", label)
	}
	return nil
}

// ruleHasCondition ensures at least one condition is configured so a rule
// cannot become an accidental catch-all.
func ruleHasCondition(r *OrderReviewRule) bool {
	return r.MinAmount != nil || r.MaxAmount != nil ||
		len(jsonStringList([]byte(r.AddressKeywords))) > 0 ||
		len(jsonStringList([]byte(r.RemarkKeywords))) > 0 ||
		len(jsonStringList([]byte(r.Platforms))) > 0 ||
		len(jsonStringList([]byte(r.ShopIDs))) > 0 ||
		r.MaxTotalQuantity != nil || r.MaxSKUQuantity != nil ||
		(r.RepeatReceiverMinOrders != nil && *r.RepeatReceiverMinOrders > 0)
}

// ListReviewRules returns tenant review rules ordered by priority.
func (s *Service) ListReviewRules(c *gin.Context) ([]OrderReviewRule, error) {
	if s == nil || s.DB == nil {
		return nil, fmt.Errorf("order: no db")
	}
	tid, err := adminperm.TenantIDFromGin(c)
	if err != nil {
		return nil, err
	}
	var rows []OrderReviewRule
	if err := s.DB.WithContext(c.Request.Context()).
		Where("tenant_id = ?", tid).
		Order("priority ASC, created_at ASC").Find(&rows).Error; err != nil {
		return nil, err
	}
	return rows, nil
}

func applyReviewRuleBody(row *OrderReviewRule, body ReviewRuleBody, isCreate bool) error {
	if name := strings.TrimSpace(body.Name); name != "" {
		row.Name = name
	}
	if isCreate && strings.TrimSpace(row.Name) == "" {
		return fmt.Errorf("规则名称不能为空")
	}
	if a := strings.TrimSpace(body.Action); a != "" {
		if !validReviewAction(a) {
			return fmt.Errorf("无效的规则动作：%s", a)
		}
		row.Action = a
	}
	if isCreate && row.Action == "" {
		return fmt.Errorf("请选择规则动作")
	}
	if body.Priority != nil {
		row.Priority = *body.Priority
	}
	if body.Enabled != nil {
		row.Enabled = *body.Enabled
	}
	if body.MinAmount != nil {
		row.MinAmount = body.MinAmount
	}
	if body.ClearMinAmount {
		row.MinAmount = nil
	}
	if body.MaxAmount != nil {
		row.MaxAmount = body.MaxAmount
	}
	if body.ClearMaxAmount {
		row.MaxAmount = nil
	}
	if err := validateReviewAmounts(row.MinAmount, row.MaxAmount); err != nil {
		return err
	}
	if body.AddressKeywords != nil {
		row.AddressKeywords = mustJSONList(normalizeReviewStrings(*body.AddressKeywords))
	}
	if body.RemarkKeywords != nil {
		row.RemarkKeywords = mustJSONList(normalizeReviewStrings(*body.RemarkKeywords))
	}
	if body.Platforms != nil {
		row.Platforms = mustJSONList(normalizeReviewStrings(*body.Platforms))
	}
	if body.ShopIDs != nil {
		ids := normalizeReviewStrings(*body.ShopIDs)
		for _, raw := range ids {
			if _, err := uuid.Parse(raw); err != nil {
				return fmt.Errorf("无效的店铺 ID：%s", raw)
			}
		}
		row.ShopIDs = mustJSONList(ids)
	}
	if body.MaxTotalQuantity != nil {
		row.MaxTotalQuantity = body.MaxTotalQuantity
	}
	if body.ClearMaxTotalQuantity {
		row.MaxTotalQuantity = nil
	}
	if body.MaxSKUQuantity != nil {
		row.MaxSKUQuantity = body.MaxSKUQuantity
	}
	if body.ClearMaxSKUQuantity {
		row.MaxSKUQuantity = nil
	}
	if body.RepeatReceiverMinOrders != nil {
		row.RepeatReceiverMinOrders = body.RepeatReceiverMinOrders
	}
	if body.RepeatReceiverWindowDays != nil {
		row.RepeatReceiverWindowDays = body.RepeatReceiverWindowDays
	}
	if body.ClearRepeatReceiver {
		row.RepeatReceiverMinOrders = nil
		row.RepeatReceiverWindowDays = nil
	}
	if err := validatePositive(row.MaxTotalQuantity, "商品总数量阈值"); err != nil {
		return err
	}
	if err := validatePositive(row.MaxSKUQuantity, "单 SKU 数量阈值"); err != nil {
		return err
	}
	if err := validatePositive(row.RepeatReceiverMinOrders, "同收件人订单数阈值"); err != nil {
		return err
	}
	if err := validatePositive(row.RepeatReceiverWindowDays, "同收件人统计窗口天数"); err != nil {
		return err
	}
	if !ruleHasCondition(row) {
		return fmt.Errorf("至少需要配置一个触发条件")
	}
	return nil
}

// CreateReviewRule adds a review rule in the current tenant.
func (s *Service) CreateReviewRule(c *gin.Context, body ReviewRuleBody, adminID *uuid.UUID) (*OrderReviewRule, error) {
	if s == nil || s.DB == nil {
		return nil, fmt.Errorf("order: no db")
	}
	tid, err := adminperm.TenantIDFromGin(c)
	if err != nil {
		return nil, err
	}
	row := OrderReviewRule{TenantID: tid, Enabled: true}
	if err := applyReviewRuleBody(&row, body, true); err != nil {
		return nil, err
	}
	if body.ShopIDs != nil {
		if err := adminperm.EnsureStoresOperable(c, s.DB, normalizeReviewStrings(*body.ShopIDs)); err != nil {
			return nil, err
		}
	}
	wantEnabled := row.Enabled
	if err := s.DB.WithContext(c.Request.Context()).Create(&row).Error; err != nil {
		return nil, err
	}
	if !wantEnabled {
		// The gorm default:true tag overrides a zero-value bool on insert.
		if err := s.DB.WithContext(c.Request.Context()).Model(&OrderReviewRule{}).
			Where("id = ?", row.ID).Update("enabled", false).Error; err != nil {
			return nil, err
		}
		row.Enabled = false
	}
	s.logReview(c, adminID, "order_review_rule.create", row.ID.String(),
		fmt.Sprintf("name=%s action=%s", row.Name, row.Action))
	return &row, nil
}

// UpdateReviewRule edits a review rule in the current tenant.
func (s *Service) UpdateReviewRule(c *gin.Context, id uuid.UUID, body ReviewRuleBody, adminID *uuid.UUID) (*OrderReviewRule, error) {
	row, err := s.findReviewRuleScoped(c, id)
	if err != nil {
		return nil, err
	}
	if err := applyReviewRuleBody(row, body, false); err != nil {
		return nil, err
	}
	if body.ShopIDs != nil {
		if err := adminperm.EnsureStoresOperable(c, s.DB, normalizeReviewStrings(*body.ShopIDs)); err != nil {
			return nil, err
		}
	}
	if err := s.DB.WithContext(c.Request.Context()).Save(row).Error; err != nil {
		return nil, err
	}
	s.logReview(c, adminID, "order_review_rule.update", row.ID.String(),
		fmt.Sprintf("name=%s enabled=%v action=%s", row.Name, row.Enabled, row.Action))
	return row, nil
}

// DeleteReviewRule removes a review rule in the current tenant.
func (s *Service) DeleteReviewRule(c *gin.Context, id uuid.UUID, adminID *uuid.UUID) error {
	row, err := s.findReviewRuleScoped(c, id)
	if err != nil {
		return err
	}
	if err := s.DB.WithContext(c.Request.Context()).
		Where("id = ? AND tenant_id = ?", row.ID, row.TenantID).Delete(&OrderReviewRule{}).Error; err != nil {
		return err
	}
	s.logReview(c, adminID, "order_review_rule.delete", row.ID.String(), fmt.Sprintf("name=%s", row.Name))
	return nil
}

func (s *Service) findReviewRuleScoped(c *gin.Context, id uuid.UUID) (*OrderReviewRule, error) {
	if s == nil || s.DB == nil {
		return nil, fmt.Errorf("order: no db")
	}
	tid, err := adminperm.TenantIDFromGin(c)
	if err != nil {
		return nil, err
	}
	var row OrderReviewRule
	if err := s.DB.WithContext(c.Request.Context()).
		First(&row, "id = ? AND tenant_id = ?", id, tid).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrReviewRuleNotFound
		}
		return nil, err
	}
	return &row, nil
}

// ReviewRuleDryRunSample is one matched order preview row.
type ReviewRuleDryRunSample struct {
	OrderID uuid.UUID `json:"orderId"`
	OrderNo string    `json:"orderNo"`
	Amount  float64   `json:"amount"`
	Reason  string    `json:"reason"`
}

// ReviewRuleDryRunResult reports how many recent orders a rule would hit.
type ReviewRuleDryRunResult struct {
	Scanned int                      `json:"scanned"`
	Matched int                      `json:"matched"`
	Samples []ReviewRuleDryRunSample `json:"samples"`
}

const reviewDryRunScanLimit = 500
const reviewDryRunSampleLimit = 10

// DryRunReviewRule evaluates a rule payload against the tenant's most recent
// orders (no writes) and reports the would-be hit count plus samples.
func (s *Service) DryRunReviewRule(c *gin.Context, body ReviewRuleBody) (*ReviewRuleDryRunResult, error) {
	if s == nil || s.DB == nil {
		return nil, fmt.Errorf("order: no db")
	}
	tid, err := adminperm.TenantIDFromGin(c)
	if err != nil {
		return nil, err
	}
	rule := OrderReviewRule{TenantID: tid, Enabled: true}
	if strings.TrimSpace(body.Action) == "" {
		body.Action = ReviewActionReview
	}
	if strings.TrimSpace(body.Name) == "" {
		body.Name = "dry-run"
	}
	if err := applyReviewRuleBody(&rule, body, true); err != nil {
		return nil, err
	}
	rule.Enabled = true

	ctx := c.Request.Context()
	scoped := s.DB.WithContext(ctx).
		Where("tenant_id = ? AND deleted_at IS NULL", tid)
	scoped, err = adminperm.ApplyStoreScope(c, s.DB, scoped, "shop_id")
	if err != nil {
		return nil, err
	}
	var orders []Order
	if err := scoped.Order("created_at DESC, id DESC").
		Limit(reviewDryRunScanLimit).Find(&orders).Error; err != nil {
		return nil, err
	}
	res := &ReviewRuleDryRunResult{Scanned: len(orders), Samples: []ReviewRuleDryRunSample{}}
	if len(orders) == 0 {
		return res, nil
	}
	ids := make([]uuid.UUID, len(orders))
	for i := range orders {
		ids[i] = orders[i].ID
	}
	var items []OrderItem
	if err := s.DB.WithContext(ctx).Where("order_id IN ?", ids).Find(&items).Error; err != nil {
		return nil, err
	}
	byOrder := map[uuid.UUID][]OrderItem{}
	for _, it := range items {
		byOrder[it.OrderID] = append(byOrder[it.OrderID], it)
	}
	tx := s.DB.WithContext(ctx)
	for i := range orders {
		o := &orders[i]
		reason, ok, err := matchReviewRule(tx, rule, buildReviewInput(o, byOrder[o.ID]))
		if err != nil {
			return nil, err
		}
		if !ok {
			continue
		}
		res.Matched++
		if len(res.Samples) < reviewDryRunSampleLimit {
			res.Samples = append(res.Samples, ReviewRuleDryRunSample{
				OrderID: o.ID, OrderNo: o.OrderNo, Amount: o.TotalAmount, Reason: reason,
			})
		}
	}
	return res, nil
}

func mustJSONList(in []string) datatypes.JSON {
	b, _ := json.Marshal(in)
	return datatypes.JSON(b)
}

func normalizeReviewStrings(in []string) []string {
	out := make([]string, 0, len(in))
	for _, v := range in {
		if t := strings.TrimSpace(v); t != "" {
			out = append(out, t)
		}
	}
	return out
}

func (s *Service) logReview(c *gin.Context, adminID *uuid.UUID, action, resourceID, msg string) {
	if s == nil || s.OpLog == nil {
		return
	}
	_ = s.OpLog.Write(c, operationlog.WriteOpts{
		AdminUserID: adminID,
		Action:      action,
		Resource:    "order_review",
		ResourceID:  resourceID,
		Status:      "success",
		Message:     msg,
	})
}
