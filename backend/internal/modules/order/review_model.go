package order

import (
	"github.com/google/uuid"
	"github.com/trademind-ai/trademind/backend/internal/pkg/model"
	"gorm.io/datatypes"
)

// Order review statuses (审单状态). Empty means the order never entered the
// review flow (no rule matched, or created before review rules existed);
// it is treated like auto_passed for downstream flows.
const (
	ReviewStatusNone       = ""
	ReviewStatusAutoPassed = "auto_passed"
	ReviewStatusPending    = "pending_review"
	ReviewStatusHeld       = "held"
	ReviewStatusApproved   = "approved"
	ReviewStatusRejected   = "rejected"
)

// Review rule actions (命中后的动作).
const (
	ReviewActionPass   = "pass"   // 自动通过
	ReviewActionReview = "review" // 打标待人工审核
	ReviewActionHold   = "hold"   // 挂起拦截
)

// ValidReviewActions lists accepted rule actions.
func ValidReviewActions() []string {
	return []string{ReviewActionPass, ReviewActionReview, ReviewActionHold}
}

// reviewBlocked reports whether a review status blocks procurement / shipping.
func reviewBlocked(status string) bool {
	return status == ReviewStatusPending || status == ReviewStatusHeld
}

// ReviewBlocked exposes the blocking check to other modules (procurement).
func ReviewBlocked(status string) bool { return reviewBlocked(status) }

// OrderReviewRule is one tenant-configurable审单规则. Empty / nil condition
// fields mean "any"; all non-empty conditions must match (AND semantics).
// Rules are evaluated by ascending priority; the first match decides the
// action, remaining matches are still recorded as hits for visibility.
type OrderReviewRule struct {
	model.HardDeleteBase
	TenantID int64  `gorm:"default:0;index:idx_order_review_rules_tenant" json:"tenantId"`
	Name     string `gorm:"size:128;not null" json:"name"`
	// Priority orders evaluation (smaller first).
	Priority int  `gorm:"default:0;index" json:"priority"`
	Enabled  bool `gorm:"default:true" json:"enabled"`
	// Action is one of ValidReviewActions().
	Action string `gorm:"size:16;not null" json:"action"`
	// Amount threshold (订单金额区间), nil = unbounded.
	MinAmount *float64 `gorm:"type:decimal(18,4)" json:"minAmount,omitempty"`
	MaxAmount *float64 `gorm:"type:decimal(18,4)" json:"maxAmount,omitempty"`
	// AddressKeywords: JSON array; matches when收货地址文本 contains any keyword
	// (黑名单地区/关键词). Orders without known address text never match.
	AddressKeywords datatypes.JSON `gorm:"type:jsonb" json:"addressKeywords,omitempty"`
	// RemarkKeywords: JSON array; matches when买家备注 contains any keyword.
	RemarkKeywords datatypes.JSON `gorm:"type:jsonb" json:"remarkKeywords,omitempty"`
	// Platforms / ShopIDs: JSON arrays limiting the rule to指定平台/店铺.
	Platforms datatypes.JSON `gorm:"type:jsonb" json:"platforms,omitempty"`
	ShopIDs   datatypes.JSON `gorm:"type:jsonb" json:"shopIds,omitempty"`
	// MaxTotalQuantity: matches when订单商品总数量 > threshold.
	MaxTotalQuantity *int `json:"maxTotalQuantity,omitempty"`
	// MaxSKUQuantity: matches when任一单 SKU 行数量 > threshold.
	MaxSKUQuantity *int `gorm:"column:max_sku_quantity" json:"maxSkuQuantity,omitempty"`
	// RepeatReceiverMinOrders: matches when同收件人（姓名+电话）在窗口期内
	// 非取消订单数（含本单）达到该值. Nil disables the condition.
	RepeatReceiverMinOrders *int `json:"repeatReceiverMinOrders,omitempty"`
	// RepeatReceiverWindowDays is the lookback window (default 7 when the
	// repeat-receiver condition is enabled).
	RepeatReceiverWindowDays *int `json:"repeatReceiverWindowDays,omitempty"`
}

// TableName maps to order_review_rules.
func (OrderReviewRule) TableName() string { return "order_review_rules" }

// OrderReviewHit records one rule matched against one order (审核原因可见).
type OrderReviewHit struct {
	model.HardDeleteBase
	TenantID int64     `gorm:"default:0;index:idx_order_review_hits_tenant" json:"tenantId"`
	OrderID  uuid.UUID `gorm:"type:char(36);index;not null" json:"orderId"`
	// RuleID references order_review_rules (rule may be deleted later; the
	// snapshot fields keep the hit readable).
	RuleID   uuid.UUID `gorm:"type:char(36);index" json:"ruleId"`
	RuleName string    `gorm:"size:128;not null" json:"ruleName"`
	Action   string    `gorm:"size:16;not null" json:"action"`
	// Reason is a human-readable中文 explanation of what matched.
	Reason string `gorm:"size:512" json:"reason"`
	// Decisive marks the rule that decided the final action (first by priority).
	Decisive bool `gorm:"default:false" json:"decisive"`
}

// TableName maps to order_review_hits.
func (OrderReviewHit) TableName() string { return "order_review_hits" }
