package order

import (
	"github.com/google/uuid"
	"github.com/trademind-ai/trademind/backend/internal/pkg/model"
	"gorm.io/datatypes"
)

// Automation trigger events (订单状态事件). Rules bind to exactly one event and
// run when that event fires for an order.
const (
	// AutomationEventOrderCreated fires after an order is created (手工新建 / 导入).
	AutomationEventOrderCreated = "order_created"
	// AutomationEventOrderPaid fires when an order becomes已付款并进入待采购
	// (创建即已付款、付款状态改为已付款、或审单放行时已付款).
	AutomationEventOrderPaid = "order_paid"
	// AutomationEventProcurementDelivered fires when a purchase order covering
	// the sales order is signed in (采购签收入库).
	AutomationEventProcurementDelivered = "procurement_delivered"
	// AutomationEventLogisticsCollected fires when采购物流揽收 (tracking filled,
	// package in transit to the warehouse).
	AutomationEventLogisticsCollected = "logistics_collected"
)

// ValidAutomationEvents lists accepted trigger events.
func ValidAutomationEvents() []string {
	return []string{
		AutomationEventOrderCreated,
		AutomationEventOrderPaid,
		AutomationEventProcurementDelivered,
		AutomationEventLogisticsCollected,
	}
}

// Automation actions (命中后执行的站内动作).
const (
	// AutomationActionConfirmPayment marks an unpaid order已付款 (低风险限定：
	// 规则必须配置金额上限，且订单未被审单拦截).
	AutomationActionConfirmPayment = "confirm_payment"
	// AutomationActionGenerateProcurement generates purchase orders for the
	// sales order (same path as手工「生成采购单」).
	AutomationActionGenerateProcurement = "generate_procurement"
	// AutomationActionMarkPrinted stamps打单状态 (waybill_printed_at).
	AutomationActionMarkPrinted = "mark_printed"
	// AutomationActionNotifyShipping通知发货工作台 (ship_ready_notified_at，
	// 打单发货工作台可按此筛选提示).
	AutomationActionNotifyShipping = "notify_shipping"
	// AutomationActionApplyShippingRule evaluates the R111发货规则 and lands
	// the matched carrier onto the order's planned-carrier fields. The mode
	// (仅推荐 / 直接应用) comes from the rule's ShippingApplyMode; the ship
	// flow keeps free carrier choice (人工覆盖).
	AutomationActionApplyShippingRule = "apply_shipping_rule"
	// AutomationActionAssignWarehouse picks a发货仓 per the rule's
	// WarehouseStrategy and stores it on the order; later deductions pin to it
	// (多仓扣减联动). Insufficient stock is a visible failed log.
	AutomationActionAssignWarehouse = "assign_warehouse"
)

// Shipping apply modes for AutomationActionApplyShippingRule.
const (
	// ShippingApplyModeRecommend records the matched carrier as a推荐 only.
	ShippingApplyModeRecommend = "recommend"
	// ShippingApplyModeApply lands the matched carrier as the order's物流商计划.
	ShippingApplyModeApply = "apply"
)

// ValidShippingApplyModes lists accepted apply modes.
func ValidShippingApplyModes() []string {
	return []string{ShippingApplyModeRecommend, ShippingApplyModeApply}
}

// Warehouse strategies for AutomationActionAssignWarehouse (kept in sync with
// inventory.ValidWarehousePlanStrategies via the router-wired hook).
const (
	// AutomationWarehouseStrategyDefault assigns the tenant's默认仓.
	AutomationWarehouseStrategyDefault = "default_warehouse"
	// AutomationWarehouseStrategyStockFirst picks the first warehouse (by
	// deduction priority) whose stock covers every order line.
	AutomationWarehouseStrategyStockFirst = "stock_first"
)

// ValidAutomationWarehouseStrategies lists accepted strategies.
func ValidAutomationWarehouseStrategies() []string {
	return []string{AutomationWarehouseStrategyDefault, AutomationWarehouseStrategyStockFirst}
}

// AutomationEventLabel returns the operator-facing中文 label for a trigger
// event (same wording as admin/src/services/orderAutomation.ts, so操作日志 /
// 订单时间线 never surface raw English enum values).
func AutomationEventLabel(event string) string {
	switch event {
	case AutomationEventOrderCreated:
		return "订单创建"
	case AutomationEventOrderPaid:
		return "进入待采购（已付款）"
	case AutomationEventProcurementDelivered:
		return "采购签收入库"
	case AutomationEventLogisticsCollected:
		return "采购物流揽收"
	default:
		return event
	}
}

// AutomationActionLabel returns the operator-facing中文 label for an action
// (same wording as the admin UI label map).
func AutomationActionLabel(action string) string {
	switch action {
	case AutomationActionConfirmPayment:
		return "自动确认付款"
	case AutomationActionGenerateProcurement:
		return "自动生成采购单"
	case AutomationActionMarkPrinted:
		return "自动标记打单"
	case AutomationActionNotifyShipping:
		return "自动通知发货工作台"
	case AutomationActionApplyShippingRule:
		return "自动应用发货规则"
	case AutomationActionAssignWarehouse:
		return "自动分仓"
	default:
		return action
	}
}

// ShippingApplyModeLabel returns the operator-facing中文 label for an apply
// mode (recommend / apply), matching the admin UI label map.
func ShippingApplyModeLabel(mode string) string {
	switch mode {
	case ShippingApplyModeApply:
		return "直接应用物流商"
	case ShippingApplyModeRecommend:
		return "仅推荐物流商"
	default:
		return mode
	}
}

// ValidAutomationActions lists accepted actions.
func ValidAutomationActions() []string {
	return []string{
		AutomationActionConfirmPayment,
		AutomationActionGenerateProcurement,
		AutomationActionMarkPrinted,
		AutomationActionNotifyShipping,
		AutomationActionApplyShippingRule,
		AutomationActionAssignWarehouse,
	}
}

// automationEventActions maps each trigger event to its allowed actions,
// keeping actions meaningful for the lifecycle stage they run in.
var automationEventActions = map[string][]string{
	AutomationEventOrderCreated:         {AutomationActionConfirmPayment, AutomationActionMarkPrinted, AutomationActionApplyShippingRule},
	AutomationEventOrderPaid:            {AutomationActionGenerateProcurement, AutomationActionMarkPrinted, AutomationActionApplyShippingRule, AutomationActionAssignWarehouse},
	AutomationEventProcurementDelivered: {AutomationActionNotifyShipping, AutomationActionMarkPrinted, AutomationActionApplyShippingRule, AutomationActionAssignWarehouse},
	AutomationEventLogisticsCollected:   {AutomationActionNotifyShipping, AutomationActionApplyShippingRule},
}

// AutomationActionAllowed reports whether the action may bind to the event.
func AutomationActionAllowed(event, action string) bool {
	for _, a := range automationEventActions[event] {
		if a == action {
			return true
		}
	}
	return false
}

// Automation log statuses.
const (
	AutomationLogSuccess = "success"
	AutomationLogFailed  = "failed"
	AutomationLogSkipped = "skipped"
)

// OrderAutomationRule is one tenant-configurable自动化订单规则: trigger event +
// AND conditions -> one station-internal action. Rules run by ascending
// priority; every matching rule executes its action (actions are idempotent
// per rule+order+event via order_automation_logs.dedup_key).
type OrderAutomationRule struct {
	model.HardDeleteBase
	TenantID int64  `gorm:"default:0;index:idx_order_automation_rules_tenant" json:"tenantId"`
	Name     string `gorm:"size:128;not null" json:"name"`
	// Priority orders execution (smaller first).
	Priority int  `gorm:"default:0;index" json:"priority"`
	Enabled  bool `gorm:"default:true" json:"enabled"`
	// TriggerEvent is one of ValidAutomationEvents().
	TriggerEvent string `gorm:"size:32;not null;index" json:"triggerEvent"`
	// Action is one of ValidAutomationActions(); must be allowed for the event.
	Action string `gorm:"size:32;not null" json:"action"`
	// Amount bounds (订单金额区间), nil = unbounded. confirm_payment rules must
	// set MaxAmount (低风险上限).
	MinAmount *float64 `gorm:"type:decimal(18,4)" json:"minAmount,omitempty"`
	MaxAmount *float64 `gorm:"type:decimal(18,4)" json:"maxAmount,omitempty"`
	// Platforms / ShopIDs: JSON arrays limiting the rule to指定平台/店铺.
	Platforms datatypes.JSON `gorm:"type:jsonb" json:"platforms,omitempty"`
	ShopIDs   datatypes.JSON `gorm:"type:jsonb" json:"shopIds,omitempty"`
	// RequireReviewPassed limits the rule to orders whose审单 status is
	// approved / auto_passed (待审/挂起 orders are always excluded regardless).
	RequireReviewPassed bool `gorm:"default:false" json:"requireReviewPassed"`
	// ShippingApplyMode parameterizes apply_shipping_rule：recommend (仅推荐)
	// or apply (直接应用). Empty means recommend.
	ShippingApplyMode string `gorm:"size:16;default:''" json:"shippingApplyMode,omitempty"`
	// WarehouseStrategy parameterizes assign_warehouse：default_warehouse or
	// stock_first. Empty means default_warehouse.
	WarehouseStrategy string `gorm:"size:32;default:''" json:"warehouseStrategy,omitempty"`
}

// TableName maps to order_automation_rules.
func (OrderAutomationRule) TableName() string { return "order_automation_rules" }

// OrderAutomationLog records one automation execution attempt outcome
// (success / failed / skipped with reason). DedupKey (tenant + rule + order +
// event) makes actions idempotent: a success/skipped row blocks re-execution;
// a failed row is retried in place (attempts incremented).
type OrderAutomationLog struct {
	model.HardDeleteBase
	TenantID int64     `gorm:"default:0;index:idx_order_automation_logs_tenant" json:"tenantId"`
	RuleID   uuid.UUID `gorm:"type:char(36);index" json:"ruleId"`
	RuleName string    `gorm:"size:128;not null" json:"ruleName"`
	OrderID  uuid.UUID `gorm:"type:char(36);index;not null" json:"orderId"`
	OrderNo  string    `gorm:"size:128;index" json:"orderNo"`
	// ShopID snapshots the order's shop at execution time so store-scope
	// filtering runs directly on the log table (nil = order without shop,
	// visible to admins only under store scope).
	ShopID *uuid.UUID `gorm:"type:char(36);index" json:"shopId,omitempty"`
	// TriggerEvent / Action snapshot what ran.
	TriggerEvent string `gorm:"size:32;not null;index" json:"triggerEvent"`
	Action       string `gorm:"size:32;not null" json:"action"`
	// Status: success | failed | skipped.
	Status string `gorm:"size:16;not null;index" json:"status"`
	// Reason is a human-readable中文 outcome (成功说明 / 失败原因 / 跳过原因).
	Reason string `gorm:"size:512" json:"reason"`
	// Attempts counts execution attempts (inline retries + manual retries).
	Attempts int `gorm:"default:1" json:"attempts"`
	// DedupKey = tenantID:ruleID:orderID:event (unique).
	DedupKey string `gorm:"size:200;uniqueIndex:idx_order_automation_logs_dedup" json:"-"`
}

// TableName maps to order_automation_logs.
func (OrderAutomationLog) TableName() string { return "order_automation_logs" }
