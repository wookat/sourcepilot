package order

import (
	"errors"
	"fmt"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/trademind-ai/trademind/backend/internal/modules/operationlog"
	"github.com/trademind-ai/trademind/backend/internal/pkg/adminperm"
	"github.com/trademind-ai/trademind/backend/internal/pkg/pagination"
	"gorm.io/gorm"
)

// ErrAutomationRuleNotFound is returned for missing / cross-tenant rules (404).
var ErrAutomationRuleNotFound = errors.New("order automation rule not found")

// ErrAutomationLogNotFound is returned for missing / cross-tenant logs (404).
var ErrAutomationLogNotFound = errors.New("order automation log not found")

// AutomationRuleBody is the create / update payload for an automation rule.
type AutomationRuleBody struct {
	Name                string    `json:"name"`
	Priority            *int      `json:"priority"`
	Enabled             *bool     `json:"enabled"`
	TriggerEvent        string    `json:"triggerEvent"`
	Action              string    `json:"action"`
	MinAmount           *float64  `json:"minAmount"`
	MaxAmount           *float64  `json:"maxAmount"`
	Platforms           *[]string `json:"platforms"`
	ShopIDs             *[]string `json:"shopIds"`
	RequireReviewPassed *bool     `json:"requireReviewPassed"`
	ClearMinAmount      bool      `json:"clearMinAmount,omitempty"`
	ClearMaxAmount      bool      `json:"clearMaxAmount,omitempty"`
}

func validAutomationEvent(e string) bool {
	for _, v := range ValidAutomationEvents() {
		if v == e {
			return true
		}
	}
	return false
}

func validAutomationAction(a string) bool {
	for _, v := range ValidAutomationActions() {
		if v == a {
			return true
		}
	}
	return false
}

func applyAutomationRuleBody(row *OrderAutomationRule, body AutomationRuleBody, isCreate bool) error {
	if name := strings.TrimSpace(body.Name); name != "" {
		row.Name = name
	}
	if isCreate && strings.TrimSpace(row.Name) == "" {
		return fmt.Errorf("规则名称不能为空")
	}
	if e := strings.TrimSpace(body.TriggerEvent); e != "" {
		if !validAutomationEvent(e) {
			return fmt.Errorf("无效的触发时机：%s", e)
		}
		row.TriggerEvent = e
	}
	if isCreate && row.TriggerEvent == "" {
		return fmt.Errorf("请选择触发时机")
	}
	if a := strings.TrimSpace(body.Action); a != "" {
		if !validAutomationAction(a) {
			return fmt.Errorf("无效的自动动作：%s", a)
		}
		row.Action = a
	}
	if isCreate && row.Action == "" {
		return fmt.Errorf("请选择自动动作")
	}
	if !AutomationActionAllowed(row.TriggerEvent, row.Action) {
		return fmt.Errorf("触发时机与自动动作不匹配")
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
	if body.RequireReviewPassed != nil {
		row.RequireReviewPassed = *body.RequireReviewPassed
	}
	if row.Action == AutomationActionConfirmPayment && row.MaxAmount == nil {
		return fmt.Errorf("自动确认付款属于低风险限定动作，必须配置金额上限")
	}
	return nil
}

// ListAutomationRules returns tenant automation rules ordered by priority.
func (s *Service) ListAutomationRules(c *gin.Context) ([]OrderAutomationRule, error) {
	if s == nil || s.DB == nil {
		return nil, fmt.Errorf("order: no db")
	}
	tid, err := adminperm.TenantIDFromGin(c)
	if err != nil {
		return nil, err
	}
	var rows []OrderAutomationRule
	if err := s.DB.WithContext(c.Request.Context()).
		Where("tenant_id = ?", tid).
		Order("priority ASC, created_at ASC").Find(&rows).Error; err != nil {
		return nil, err
	}
	return rows, nil
}

// CreateAutomationRule adds an automation rule in the current tenant.
func (s *Service) CreateAutomationRule(c *gin.Context, body AutomationRuleBody, adminID *uuid.UUID) (*OrderAutomationRule, error) {
	if s == nil || s.DB == nil {
		return nil, fmt.Errorf("order: no db")
	}
	tid, err := adminperm.TenantIDFromGin(c)
	if err != nil {
		return nil, err
	}
	row := OrderAutomationRule{TenantID: tid, Enabled: true}
	if err := applyAutomationRuleBody(&row, body, true); err != nil {
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
		if err := s.DB.WithContext(c.Request.Context()).Model(&OrderAutomationRule{}).
			Where("id = ?", row.ID).Update("enabled", false).Error; err != nil {
			return nil, err
		}
		row.Enabled = false
	}
	s.logAutomation(c, adminID, "order_automation_rule.create", row.ID.String(),
		fmt.Sprintf("name=%s event=%s action=%s", row.Name, row.TriggerEvent, row.Action))
	return &row, nil
}

// UpdateAutomationRule edits an automation rule in the current tenant.
func (s *Service) UpdateAutomationRule(c *gin.Context, id uuid.UUID, body AutomationRuleBody, adminID *uuid.UUID) (*OrderAutomationRule, error) {
	row, err := s.findAutomationRuleScoped(c, id)
	if err != nil {
		return nil, err
	}
	if err := applyAutomationRuleBody(row, body, false); err != nil {
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
	s.logAutomation(c, adminID, "order_automation_rule.update", row.ID.String(),
		fmt.Sprintf("name=%s enabled=%v event=%s action=%s", row.Name, row.Enabled, row.TriggerEvent, row.Action))
	return row, nil
}

// DeleteAutomationRule removes an automation rule in the current tenant.
func (s *Service) DeleteAutomationRule(c *gin.Context, id uuid.UUID, adminID *uuid.UUID) error {
	row, err := s.findAutomationRuleScoped(c, id)
	if err != nil {
		return err
	}
	if err := s.DB.WithContext(c.Request.Context()).
		Where("id = ? AND tenant_id = ?", row.ID, row.TenantID).Delete(&OrderAutomationRule{}).Error; err != nil {
		return err
	}
	s.logAutomation(c, adminID, "order_automation_rule.delete", row.ID.String(), fmt.Sprintf("name=%s", row.Name))
	return nil
}

func (s *Service) findAutomationRuleScoped(c *gin.Context, id uuid.UUID) (*OrderAutomationRule, error) {
	if s == nil || s.DB == nil {
		return nil, fmt.Errorf("order: no db")
	}
	tid, err := adminperm.TenantIDFromGin(c)
	if err != nil {
		return nil, err
	}
	var row OrderAutomationRule
	if err := s.DB.WithContext(c.Request.Context()).
		First(&row, "id = ? AND tenant_id = ?", id, tid).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrAutomationRuleNotFound
		}
		return nil, err
	}
	return &row, nil
}

// AutomationDryRunSample is one would-hit order preview row.
type AutomationDryRunSample struct {
	OrderID uuid.UUID `json:"orderId"`
	OrderNo string    `json:"orderNo"`
	Amount  float64   `json:"amount"`
	Reason  string    `json:"reason"`
	// Blocked marks orders the safety boundary would skip (审单待审/挂起).
	Blocked bool `json:"blocked"`
}

// AutomationDryRunResult reports how many recent orders the rule conditions
// would match; blocked counts orders the #240 safety boundary would skip.
type AutomationDryRunResult struct {
	Scanned int                      `json:"scanned"`
	Matched int                      `json:"matched"`
	Blocked int                      `json:"blocked"`
	Samples []AutomationDryRunSample `json:"samples"`
}

// DryRunAutomationRule evaluates a rule payload against the tenant's most
// recent orders (no writes, no actions executed).
func (s *Service) DryRunAutomationRule(c *gin.Context, body AutomationRuleBody) (*AutomationDryRunResult, error) {
	if s == nil || s.DB == nil {
		return nil, fmt.Errorf("order: no db")
	}
	tid, err := adminperm.TenantIDFromGin(c)
	if err != nil {
		return nil, err
	}
	rule := OrderAutomationRule{TenantID: tid, Enabled: true}
	if strings.TrimSpace(body.Name) == "" {
		body.Name = "dry-run"
	}
	if err := applyAutomationRuleBody(&rule, body, true); err != nil {
		return nil, err
	}

	ctx := c.Request.Context()
	tx := s.DB.WithContext(ctx).
		Where("tenant_id = ? AND deleted_at IS NULL", tid)
	tx, err = adminperm.ApplyStoreScope(c, s.DB, tx, "shop_id")
	if err != nil {
		return nil, err
	}
	var orders []Order
	if err := tx.Order("created_at DESC, id DESC").
		Limit(reviewDryRunScanLimit).Find(&orders).Error; err != nil {
		return nil, err
	}
	res := &AutomationDryRunResult{Scanned: len(orders), Samples: []AutomationDryRunSample{}}
	for i := range orders {
		o := &orders[i]
		reason, ok := matchAutomationRule(rule, o)
		if !ok {
			continue
		}
		res.Matched++
		blocked := ReviewBlocked(o.ReviewStatus)
		if blocked {
			res.Blocked++
			reason = "命中条件，但审单待审/挂起将按安全边界跳过"
		}
		if len(res.Samples) < reviewDryRunSampleLimit {
			res.Samples = append(res.Samples, AutomationDryRunSample{
				OrderID: o.ID, OrderNo: o.OrderNo, Amount: o.TotalAmount,
				Reason: reason, Blocked: blocked,
			})
		}
	}
	return res, nil
}

// AutomationLogQuery filters GET /order-automation-logs.
type AutomationLogQuery struct {
	Page     int
	PageSize int
	Status   string
	Event    string
	Action   string
	RuleID   string
	Keyword  string
}

// AutomationLogResult is a paginated execution log page.
type AutomationLogResult struct {
	Items      []OrderAutomationLog `json:"items"`
	Total      int64                `json:"total"`
	Page       int                  `json:"page"`
	PageSize   int                  `json:"pageSize"`
	TotalPages int                  `json:"totalPages"`
}

// ListAutomationLogs returns tenant execution logs, newest first.
func (s *Service) ListAutomationLogs(c *gin.Context, q AutomationLogQuery) (*AutomationLogResult, error) {
	if s == nil || s.DB == nil {
		return nil, fmt.Errorf("order: no db")
	}
	tid, err := adminperm.TenantIDFromGin(c)
	if err != nil {
		return nil, err
	}
	pg, err := pagination.NormalizePage(q.Page, q.PageSize)
	if err != nil {
		return nil, err
	}
	tx := s.DB.WithContext(c.Request.Context()).Model(&OrderAutomationLog{}).
		Where("tenant_id = ?", tid)
	// Store scope filters directly on the log's denormalized shop_id snapshot
	// (logs of shopless orders stay admin-only, matching the previous
	// order-subquery behavior).
	tx, err = adminperm.ApplyStoreScope(c, s.DB, tx, "shop_id")
	if err != nil {
		return nil, err
	}
	if st := strings.TrimSpace(q.Status); st != "" {
		tx = tx.Where("status = ?", st)
	}
	if e := strings.TrimSpace(q.Event); e != "" {
		tx = tx.Where("trigger_event = ?", e)
	}
	if a := strings.TrimSpace(q.Action); a != "" {
		tx = tx.Where("action = ?", a)
	}
	if rid := strings.TrimSpace(q.RuleID); rid != "" {
		id, err := uuid.Parse(rid)
		if err != nil {
			return nil, fmt.Errorf("无效的规则 ID")
		}
		tx = tx.Where("rule_id = ?", id)
	}
	if kw := strings.TrimSpace(q.Keyword); kw != "" {
		like := "%" + kw + "%"
		tx = tx.Where("order_no ILIKE ? OR rule_name ILIKE ?", like, like)
	}
	var total int64
	if err := tx.Count(&total).Error; err != nil {
		return nil, err
	}
	var items []OrderAutomationLog
	if err := tx.Order("created_at DESC, id DESC").
		Offset(pg.Offset).Limit(pg.Limit).Find(&items).Error; err != nil {
		return nil, err
	}
	if items == nil {
		items = []OrderAutomationLog{}
	}
	totalPages := int((total + int64(pg.Limit) - 1) / int64(pg.Limit))
	return &AutomationLogResult{
		Items: items, Total: total, Page: pg.Page, PageSize: pg.Limit,
		TotalPages: totalPages,
	}, nil
}

// ListOrderAutomationTrail returns automation log entries of one order for
// the order timeline (订单时间线「自动规则」留痕).
func (s *Service) ListOrderAutomationTrail(c *gin.Context, orderID uuid.UUID) ([]OrderAutomationLog, error) {
	if s == nil || s.DB == nil {
		return nil, fmt.Errorf("order: no db")
	}
	tid, err := adminperm.TenantIDFromGin(c)
	if err != nil {
		return nil, err
	}
	// 404 on missing / cross-tenant / unauthorized-store orders.
	if _, err := s.findOrderBare(c, orderID); err != nil {
		return nil, err
	}
	var items []OrderAutomationLog
	if err := s.DB.WithContext(c.Request.Context()).
		Where("tenant_id = ? AND order_id = ?", tid, orderID).
		Order("created_at ASC, id ASC").Find(&items).Error; err != nil {
		return nil, err
	}
	if items == nil {
		items = []OrderAutomationLog{}
	}
	return items, nil
}

// RetryAutomationLog re-executes one failed execution log entry.
func (s *Service) RetryAutomationLog(c *gin.Context, logID uuid.UUID, adminID *uuid.UUID) (*OrderAutomationLog, error) {
	if s == nil || s.DB == nil {
		return nil, fmt.Errorf("order: no db")
	}
	tid, err := adminperm.TenantIDFromGin(c)
	if err != nil {
		return nil, err
	}
	var row OrderAutomationLog
	if err := s.DB.WithContext(c.Request.Context()).
		First(&row, "id = ? AND tenant_id = ?", logID, tid).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrAutomationLogNotFound
		}
		return nil, err
	}
	if _, err := s.findOrderBare(c, row.OrderID); err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrAutomationLogNotFound
		}
		return nil, err
	}
	if row.Status != AutomationLogFailed {
		return nil, fmt.Errorf("仅失败记录支持重试")
	}
	updated, err := s.retryAutomationExecution(c.Request.Context(), &row)
	if err != nil {
		return nil, err
	}
	s.logAutomation(c, adminID, "order_automation_log.retry", row.ID.String(),
		fmt.Sprintf("rule=%s orderNo=%s status=%s", row.RuleName, row.OrderNo, updated.Status))
	return updated, nil
}

func (s *Service) logAutomation(c *gin.Context, adminID *uuid.UUID, action, resourceID, msg string) {
	if s == nil || s.OpLog == nil {
		return
	}
	_ = s.OpLog.Write(c, operationlog.WriteOpts{
		AdminUserID: adminID,
		Action:      action,
		Resource:    "order_automation",
		ResourceID:  resourceID,
		Status:      "success",
		Message:     msg,
	})
}
