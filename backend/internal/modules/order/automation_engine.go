package order

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/trademind-ai/trademind/backend/internal/modules/operationlog"
	"github.com/trademind-ai/trademind/backend/internal/modules/waybill"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// AutomationHooks injects cross-module actions the automation engine cannot
// call directly (order must not import procurement). Wired in the API router.
type AutomationHooks struct {
	// GenerateProcurement creates purchase orders for one sales order using
	// the same path as the manual批量生成采购单. It returns a human-readable
	// summary or an error carrying the blocker reasons.
	GenerateProcurement func(ctx context.Context, tenantID int64, orderID uuid.UUID, idempotencyKey string) (string, error)
	// PlanWarehouse picks one发货仓 able to cover every demand line under the
	// strategy (inventory.PlanOrderWarehouse). Insufficient stock surfaces as
	// an error so the engine records a visible failed log.
	PlanWarehouse func(ctx context.Context, tenantID int64, strategy string, demands []AutomationWarehouseDemand) (*AutomationWarehousePlan, error)
}

// AutomationWarehouseDemand is one SKU quantity the order needs (matched lines only).
type AutomationWarehouseDemand struct {
	ProductSKUID uuid.UUID
	SKUCode      string
	Quantity     int
}

// AutomationWarehousePlan is the chosen warehouse for one order.
type AutomationWarehousePlan struct {
	WarehouseID   uuid.UUID
	WarehouseName string
}

const (
	automationMaxAttempts = 3
	automationMaxDepth    = 2
)

// AutomationSkip marks a hook outcome whose precondition cannot be satisfied
// by retrying (e.g. the order has no item rows). The engine records it as a
// skipped log entry instead of a retryable failure.
type AutomationSkip struct{ Reason string }

func (e *AutomationSkip) Error() string { return e.Reason }

// FireOrderEvent runs all enabled automation rules bound to the event against
// the order. It is safe to call after any state event; failures are recorded
// in order_automation_logs and never break the triggering flow.
func (s *Service) FireOrderEvent(ctx context.Context, tenantID int64, orderID uuid.UUID, event string) {
	s.fireOrderEvent(ctx, tenantID, orderID, event, 1)
}

func (s *Service) fireOrderEvent(ctx context.Context, tenantID int64, orderID uuid.UUID, event string, depth int) {
	if s == nil || s.DB == nil || depth > automationMaxDepth {
		return
	}
	var rules []OrderAutomationRule
	if err := s.DB.WithContext(ctx).
		Where("tenant_id = ? AND trigger_event = ? AND enabled = TRUE", tenantID, event).
		Order("priority ASC, created_at ASC").Find(&rules).Error; err != nil || len(rules) == 0 {
		return
	}
	var o Order
	if err := s.DB.WithContext(ctx).
		First(&o, "id = ? AND tenant_id = ?", orderID, tenantID).Error; err != nil {
		return
	}
	blocked := ReviewBlocked(o.ReviewStatus)
	for _, r := range rules {
		reason, matched := matchAutomationRule(r, &o)
		if !matched {
			continue
		}
		if _, done := s.automationAlreadyDone(ctx, r, &o); done {
			// Idempotency: this rule already ran for the order + event.
			continue
		}
		if blocked {
			// #240 safety boundary: pending/held orders are never automated.
			s.writeAutomationLog(ctx, r, &o, AutomationLogSkipped,
				"订单审单待审/挂起，按安全边界跳过自动化", 1)
			continue
		}
		status, outcome, attempts := s.executeAutomationAction(ctx, r, &o, reason)
		s.writeAutomationLog(ctx, r, &o, status, outcome, attempts)
		if status == AutomationLogSuccess && r.Action == AutomationActionConfirmPayment {
			// Auto-confirmed payment moves the order into待采购: chain the
			// order_paid event so e.g. auto-generate-procurement rules run.
			s.fireOrderEvent(ctx, tenantID, orderID, AutomationEventOrderPaid, depth+1)
		}
	}
}

// matchAutomationRule applies the rule's AND conditions to the order.
func matchAutomationRule(r OrderAutomationRule, o *Order) (string, bool) {
	var reasons []string
	if platforms := jsonStringList(json.RawMessage(r.Platforms)); len(platforms) > 0 {
		if !listContainsFold(platforms, o.Platform) {
			return "", false
		}
		reasons = append(reasons, fmt.Sprintf("平台命中（%s）", o.Platform))
	}
	if shopIDs := jsonStringList(json.RawMessage(r.ShopIDs)); len(shopIDs) > 0 {
		if o.ShopID == nil || !listContainsFold(shopIDs, o.ShopID.String()) {
			return "", false
		}
		reasons = append(reasons, "店铺命中")
	}
	if r.MinAmount != nil || r.MaxAmount != nil {
		if r.MinAmount != nil && o.TotalAmount < *r.MinAmount {
			return "", false
		}
		if r.MaxAmount != nil && o.TotalAmount > *r.MaxAmount {
			return "", false
		}
		reasons = append(reasons, fmt.Sprintf("订单金额 %.2f 落入阈值区间", o.TotalAmount))
	}
	if r.RequireReviewPassed {
		if o.ReviewStatus != ReviewStatusApproved && o.ReviewStatus != ReviewStatusAutoPassed {
			return "", false
		}
		reasons = append(reasons, "审单已通过")
	}
	if len(reasons) == 0 {
		reasons = append(reasons, "无附加条件")
	}
	return strings.Join(reasons, "；"), true
}

// executeAutomationAction runs the rule action with inline retries. It
// returns the log status, a human-readable outcome and the attempt count.
func (s *Service) executeAutomationAction(ctx context.Context, r OrderAutomationRule, o *Order, matchReason string) (string, string, int) {
	attempts := 0
	var lastErr error
	for attempts < automationMaxAttempts {
		attempts++
		status, outcome, err := s.runAutomationActionOnce(ctx, r, o)
		if err == nil {
			if outcome == "" {
				outcome = matchReason
			}
			return status, outcome, attempts
		}
		lastErr = err
	}
	return AutomationLogFailed, fmt.Sprintf("执行失败（本轮尝试 %d 次）：%s", attempts, lastErr.Error()), attempts
}

// automationAlreadyDone reports whether a success/skipped log already exists
// for the rule+order+event (idempotency guard).
func (s *Service) automationAlreadyDone(ctx context.Context, r OrderAutomationRule, o *Order) (*OrderAutomationLog, bool) {
	var prior OrderAutomationLog
	err := s.DB.WithContext(ctx).
		Where("dedup_key = ? AND status IN ?", automationDedupKey(r, o.ID), []string{AutomationLogSuccess, AutomationLogSkipped}).
		First(&prior).Error
	if err != nil {
		return nil, false
	}
	return &prior, true
}

// runAutomationActionOnce executes the action a single time. A returned error
// marks a retryable failure; a (status, outcome, nil) result is final.
func (s *Service) runAutomationActionOnce(ctx context.Context, r OrderAutomationRule, o *Order) (string, string, error) {
	switch r.Action {
	case AutomationActionConfirmPayment:
		return s.autoConfirmPayment(ctx, r, o)
	case AutomationActionGenerateProcurement:
		return s.autoGenerateProcurement(ctx, r, o)
	case AutomationActionMarkPrinted:
		return s.autoMarkPrinted(ctx, o)
	case AutomationActionNotifyShipping:
		return s.autoNotifyShipping(ctx, o)
	case AutomationActionApplyShippingRule:
		return s.autoApplyShippingRule(ctx, r, o)
	case AutomationActionAssignWarehouse:
		return s.autoAssignWarehouse(ctx, r, o)
	default:
		return AutomationLogFailed, "", fmt.Errorf("未知动作：%s", r.Action)
	}
}

func (s *Service) autoConfirmPayment(ctx context.Context, r OrderAutomationRule, o *Order) (string, string, error) {
	if r.MaxAmount == nil {
		return AutomationLogSkipped, "自动确认付款规则未配置金额上限（低风险限定），跳过", nil
	}
	if o.PaymentStatus != PaymentUnpaid {
		return AutomationLogSkipped, "订单不是未付款状态，无需自动确认付款", nil
	}
	if o.Status != StatusPending {
		return AutomationLogSkipped, fmt.Sprintf("订单状态 %s 不支持自动确认付款", o.Status), nil
	}
	now := time.Now().UTC()
	res := s.DB.WithContext(ctx).Model(&Order{}).
		Where("id = ? AND tenant_id = ? AND payment_status = ?", o.ID, o.TenantID, PaymentUnpaid).
		Updates(map[string]any{"payment_status": PaymentPaid, "status": StatusPaid, "paid_at": now})
	if res.Error != nil {
		return AutomationLogFailed, "", res.Error
	}
	if res.RowsAffected == 0 {
		return AutomationLogSkipped, "订单不是未付款状态，无需自动确认付款", nil
	}
	o.PaymentStatus = PaymentPaid
	o.Status = StatusPaid
	o.PaidAt = &now
	return AutomationLogSuccess, "已自动确认付款（低风险条件）", nil
}

func (s *Service) autoGenerateProcurement(ctx context.Context, r OrderAutomationRule, o *Order) (string, string, error) {
	if s.Automation == nil || s.Automation.GenerateProcurement == nil {
		return AutomationLogFailed, "", errors.New("采购生成能力未接入")
	}
	if o.PaymentStatus != PaymentPaid {
		return AutomationLogSkipped, "订单尚未付款，跳过自动生成采购单", nil
	}
	summary, err := s.Automation.GenerateProcurement(ctx, o.TenantID, o.ID,
		fmt.Sprintf("auto-%s-%s", r.ID.String(), o.ID.String()))
	if err != nil {
		var skip *AutomationSkip
		if errors.As(err, &skip) {
			return AutomationLogSkipped, skip.Reason, nil
		}
		return AutomationLogFailed, "", err
	}
	if summary == "" {
		summary = "已自动生成采购单"
	}
	return AutomationLogSuccess, summary, nil
}

func (s *Service) autoMarkPrinted(ctx context.Context, o *Order) (string, string, error) {
	if o.WaybillPrintedAt != nil {
		return AutomationLogSkipped, "订单已是打单状态，无需重复标记", nil
	}
	now := time.Now().UTC()
	if err := s.DB.WithContext(ctx).Model(&Order{}).
		Where("id = ? AND tenant_id = ?", o.ID, o.TenantID).
		Update("waybill_printed_at", now).Error; err != nil {
		return AutomationLogFailed, "", err
	}
	o.WaybillPrintedAt = &now
	return AutomationLogSuccess, "已自动标记打单", nil
}

func (s *Service) autoNotifyShipping(ctx context.Context, o *Order) (string, string, error) {
	if o.ShipReadyNotifiedAt != nil {
		return AutomationLogSkipped, "发货工作台已收到过该订单的备货通知", nil
	}
	now := time.Now().UTC()
	if err := s.DB.WithContext(ctx).Model(&Order{}).
		Where("id = ? AND tenant_id = ?", o.ID, o.TenantID).
		Update("ship_ready_notified_at", now).Error; err != nil {
		return AutomationLogFailed, "", err
	}
	o.ShipReadyNotifiedAt = &now
	return AutomationLogSuccess, "已通知发货工作台备货发货", nil
}

// autoApplyShippingRule evaluates the tenant's R111发货规则 and lands the
// matched carrier onto the order's planned-carrier fields. An existing plan
// is never overwritten (人工覆盖 / earlier automation wins).
func (s *Service) autoApplyShippingRule(ctx context.Context, r OrderAutomationRule, o *Order) (string, string, error) {
	if s.Waybill == nil {
		return AutomationLogFailed, "", errors.New("发货规则能力未接入")
	}
	if o.ShippedAt != nil {
		return AutomationLogSkipped, "订单已发货，无需再应用发货规则", nil
	}
	if o.PlannedCarrierCode != "" {
		label := o.PlannedCarrierName
		if label == "" {
			label = o.PlannedCarrierCode
		}
		return AutomationLogSkipped, fmt.Sprintf("订单已有物流商计划（%s），保留现有选择", label), nil
	}
	amount := o.TotalAmount
	rec, err := s.Waybill.RecommendTenant(ctx, o.TenantID, waybill.MatchAttrs{
		Platform: o.Platform,
		Amount:   &amount,
	})
	if err != nil {
		return AutomationLogFailed, "", err
	}
	if !rec.Matched {
		return AutomationLogSkipped, "没有命中任何发货规则，未推荐物流商", nil
	}
	mode := strings.TrimSpace(r.ShippingApplyMode)
	if mode == "" {
		mode = ShippingApplyModeRecommend
	}
	now := time.Now().UTC()
	res := s.DB.WithContext(ctx).Model(&Order{}).
		Where("id = ? AND tenant_id = ? AND (planned_carrier_code = '' OR planned_carrier_code IS NULL)", o.ID, o.TenantID).
		Updates(map[string]any{
			"planned_carrier_code": rec.CarrierCode,
			"planned_carrier_name": rec.CarrierName,
			"planned_carrier_mode": mode,
			"planned_carrier_rule": rec.RuleName,
			"planned_carrier_at":   now,
		})
	if res.Error != nil {
		return AutomationLogFailed, "", res.Error
	}
	if res.RowsAffected == 0 {
		return AutomationLogSkipped, "订单已有物流商计划，保留现有选择", nil
	}
	o.PlannedCarrierCode = rec.CarrierCode
	o.PlannedCarrierName = rec.CarrierName
	o.PlannedCarrierMode = mode
	o.PlannedCarrierRule = rec.RuleName
	o.PlannedCarrierAt = &now
	label := rec.CarrierName
	if label == "" {
		label = rec.CarrierCode
	}
	if mode == ShippingApplyModeApply {
		return AutomationLogSuccess, fmt.Sprintf("已按发货规则「%s」应用物流商：%s（发货时仍可人工改选）", rec.RuleName, label), nil
	}
	return AutomationLogSuccess, fmt.Sprintf("已按发货规则「%s」推荐物流商：%s（仅推荐，发货时人工确认）", rec.RuleName, label), nil
}

// autoAssignWarehouse plans a发货仓 for the order's matched SKU lines and
// stores it; insufficient stock is a visible failed log (retryable after补货).
func (s *Service) autoAssignWarehouse(ctx context.Context, r OrderAutomationRule, o *Order) (string, string, error) {
	if s.Automation == nil || s.Automation.PlanWarehouse == nil {
		return AutomationLogFailed, "", errors.New("分仓能力未接入")
	}
	if o.ShippedAt != nil {
		return AutomationLogSkipped, "订单已发货，无需自动分仓", nil
	}
	if o.AssignedWarehouseID != nil {
		label := o.AssignedWarehouseName
		if label == "" {
			label = o.AssignedWarehouseID.String()
		}
		return AutomationLogSkipped, fmt.Sprintf("订单已分配发货仓（%s），保留现有分配", label), nil
	}
	var items []OrderItem
	if err := s.DB.WithContext(ctx).
		Where("order_id = ?", o.ID).Find(&items).Error; err != nil {
		return AutomationLogFailed, "", err
	}
	demands := make([]AutomationWarehouseDemand, 0, len(items))
	for _, it := range items {
		if it.ProductSKUID == nil || *it.ProductSKUID == uuid.Nil || it.Quantity <= 0 {
			continue
		}
		demands = append(demands, AutomationWarehouseDemand{
			ProductSKUID: *it.ProductSKUID,
			SKUCode:      it.SKUCode,
			Quantity:     it.Quantity,
		})
	}
	if len(demands) == 0 {
		return AutomationLogSkipped, "订单没有已匹配本地 SKU 的商品行，无法自动分仓", nil
	}
	strategy := strings.TrimSpace(r.WarehouseStrategy)
	if strategy == "" {
		strategy = AutomationWarehouseStrategyDefault
	}
	plan, err := s.Automation.PlanWarehouse(ctx, o.TenantID, strategy, demands)
	if err != nil {
		return AutomationLogFailed, "", err
	}
	now := time.Now().UTC()
	res := s.DB.WithContext(ctx).Model(&Order{}).
		Where("id = ? AND tenant_id = ? AND assigned_warehouse_id IS NULL", o.ID, o.TenantID).
		Updates(map[string]any{
			"assigned_warehouse_id":       plan.WarehouseID,
			"assigned_warehouse_name":     plan.WarehouseName,
			"assigned_warehouse_strategy": strategy,
			"warehouse_assigned_at":       now,
		})
	if res.Error != nil {
		return AutomationLogFailed, "", res.Error
	}
	if res.RowsAffected == 0 {
		return AutomationLogSkipped, "订单已分配发货仓，保留现有分配", nil
	}
	wh := plan.WarehouseID
	o.AssignedWarehouseID = &wh
	o.AssignedWarehouseName = plan.WarehouseName
	o.AssignedWarehouseStrategy = strategy
	o.WarehouseAssignedAt = &now
	return AutomationLogSuccess, fmt.Sprintf("已自动分仓：%s（策略：%s，后续扣减默认从该仓出库）", plan.WarehouseName, automationWarehouseStrategyLabel(strategy)), nil
}

func automationWarehouseStrategyLabel(strategy string) string {
	switch strategy {
	case AutomationWarehouseStrategyStockFirst:
		return "库存充足优先"
	default:
		return "默认仓"
	}
}

func automationDedupKey(r OrderAutomationRule, orderID uuid.UUID) string {
	return fmt.Sprintf("%d:%s:%s:%s", r.TenantID, r.ID.String(), orderID.String(), r.TriggerEvent)
}

// writeAutomationLog upserts the execution log by dedup key (a retried
// failure updates the same row) and leaves an order timeline audit entry.
func (s *Service) writeAutomationLog(ctx context.Context, r OrderAutomationRule, o *Order, status, reason string, attempts int) {
	row := OrderAutomationLog{
		TenantID:     r.TenantID,
		RuleID:       r.ID,
		RuleName:     r.Name,
		OrderID:      o.ID,
		OrderNo:      o.OrderNo,
		ShopID:       o.ShopID,
		TriggerEvent: r.TriggerEvent,
		Action:       r.Action,
		Status:       status,
		Reason:       reason,
		Attempts:     attempts,
		DedupKey:     automationDedupKey(r, o.ID),
	}
	if err := s.DB.WithContext(ctx).Clauses(clause.OnConflict{
		Columns: []clause.Column{{Name: "dedup_key"}},
		DoUpdates: clause.Assignments(map[string]any{
			"status":   status,
			"reason":   reason,
			"attempts": gorm.Expr("order_automation_logs.attempts + ?", attempts),
		}),
	}).Create(&row).Error; err != nil {
		return
	}
	if s.OpLog != nil {
		_ = s.OpLog.WriteBackground(ctx, operationlog.WriteOpts{
			TenantID:   r.TenantID,
			Action:     "order_automation.execute",
			Resource:   "order",
			ResourceID: o.ID.String(),
			Status:     status,
			Message:    fmt.Sprintf("自动规则：%s（%s → %s）%s", r.Name, r.TriggerEvent, r.Action, reason),
		})
	}
}

// RetryAutomationLog re-executes one failed automation log entry.
func (s *Service) retryAutomationExecution(ctx context.Context, log *OrderAutomationLog) (*OrderAutomationLog, error) {
	var rule OrderAutomationRule
	if err := s.DB.WithContext(ctx).
		First(&rule, "id = ? AND tenant_id = ?", log.RuleID, log.TenantID).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, fmt.Errorf("规则已删除，无法重试")
		}
		return nil, err
	}
	var o Order
	if err := s.DB.WithContext(ctx).
		First(&o, "id = ? AND tenant_id = ?", log.OrderID, log.TenantID).Error; err != nil {
		return nil, fmt.Errorf("订单不存在，无法重试")
	}
	if ReviewBlocked(o.ReviewStatus) {
		s.writeAutomationLog(ctx, rule, &o, AutomationLogSkipped,
			"订单审单待审/挂起，按安全边界跳过自动化", 1)
	} else {
		status, outcome, attempts := s.executeAutomationAction(ctx, rule, &o, "手动重试")
		s.writeAutomationLog(ctx, rule, &o, status, outcome, attempts)
		if status == AutomationLogSuccess && rule.Action == AutomationActionConfirmPayment {
			s.fireOrderEvent(ctx, o.TenantID, o.ID, AutomationEventOrderPaid, 2)
		}
	}
	var updated OrderAutomationLog
	if err := s.DB.WithContext(ctx).
		First(&updated, "id = ? AND tenant_id = ?", log.ID, log.TenantID).Error; err != nil {
		return nil, err
	}
	return &updated, nil
}
