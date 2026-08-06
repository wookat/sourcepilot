package customerchat

import (
	"time"

	"github.com/google/uuid"
	"github.com/trademind-ai/trademind/backend/internal/pkg/model"
	"gorm.io/datatypes"
)

// Buyer auto-message order nodes (订单节点).
const (
	BuyerMsgNodePaid               = "paid"
	BuyerMsgNodeShipped            = "shipped"
	BuyerMsgNodeDelivered          = "delivered"
	BuyerMsgNodeLogisticsException = "logistics_exception"
	BuyerMsgNodeRefunded           = "refunded"
)

// BuyerMsgNodes lists all valid buyer auto-message node keys.
var BuyerMsgNodes = []string{
	BuyerMsgNodePaid,
	BuyerMsgNodeShipped,
	BuyerMsgNodeDelivered,
	BuyerMsgNodeLogisticsException,
	BuyerMsgNodeRefunded,
}

// IsValidBuyerMsgNode reports whether n is a known node key.
func IsValidBuyerMsgNode(n string) bool {
	for _, k := range BuyerMsgNodes {
		if k == n {
			return true
		}
	}
	return false
}

// Buyer message draft statuses.
const (
	BuyerMsgDraftPending = "pending"
	BuyerMsgDraftSent    = "sent"
	BuyerMsgDraftIgnored = "ignored"
)

// BuyerMsgDraftStatuses lists all valid draft statuses.
var BuyerMsgDraftStatuses = []string{BuyerMsgDraftPending, BuyerMsgDraftSent, BuyerMsgDraftIgnored}

// IsValidBuyerMsgDraftStatus reports whether st is a known draft status.
func IsValidBuyerMsgDraftStatus(st string) bool {
	for _, k := range BuyerMsgDraftStatuses {
		if k == st {
			return true
		}
	}
	return false
}

// BuyerMessageRule maps one order node to a reply template (tenant scoped).
// When enabled, matching orders get a pending draft generated automatically;
// drafts are NEVER sent to any platform by the system — a human copies the
// content into the platform seller console and marks the draft as sent.
type BuyerMessageRule struct {
	model.Base
	TenantID   int64     `gorm:"not null;default:0;index" json:"tenantId"`
	Name       string    `gorm:"size:255;not null" json:"name"`
	Node       string    `gorm:"size:32;index;not null" json:"node"`
	TemplateID uuid.UUID `gorm:"type:char(36);index;not null" json:"templateId"`
	// 无 default tag：带 default 时 GORM Create 会跳过零值 false 导致落库为 true；
	// 默认启用语义由 service 层保证。
	Enabled bool `gorm:"not null;index" json:"enabled"`
	// EffectiveFrom 限定草稿生成范围：仅对该时刻之后发生的订单节点事件生成
	// 草稿。nil 表示回溯全部存量订单（显式开启「回溯存量」，或该字段引入前的
	// 历史规则）。创建/启用规则时由 service 层写入当前时间。
	EffectiveFrom *time.Time `gorm:"index" json:"effectiveFrom,omitempty"`
	// Platforms / ShopIDs are optional JSON string arrays; empty means all.
	Platforms datatypes.JSON `gorm:"type:jsonb" json:"platforms,omitempty"`
	ShopIDs   datatypes.JSON `gorm:"type:jsonb" json:"shopIds,omitempty"`
	CreatedBy *uuid.UUID     `gorm:"type:char(36);index" json:"createdBy,omitempty"`
}

func (BuyerMessageRule) TableName() string { return "buyer_message_rules" }

// BuyerMessageDraft is one pending outbound buyer message (人工确认后在平台
// 后台发送，系统只记录回执，绝不自动外发). One draft per (tenant, order, node).
type BuyerMessageDraft struct {
	model.Base
	TenantID       int64          `gorm:"not null;default:0;index;uniqueIndex:idx_buyer_msg_drafts_tenant_order_node,priority:1" json:"tenantId"`
	OrderID        uuid.UUID      `gorm:"type:char(36);not null;index;uniqueIndex:idx_buyer_msg_drafts_tenant_order_node,priority:2" json:"orderId"`
	Node           string         `gorm:"size:32;not null;index;uniqueIndex:idx_buyer_msg_drafts_tenant_order_node,priority:3" json:"node"`
	RuleID         uuid.UUID      `gorm:"type:char(36);index;not null" json:"ruleId"`
	TemplateID     uuid.UUID      `gorm:"type:char(36);index;not null" json:"templateId"`
	TemplateName   string         `gorm:"size:255" json:"templateName"`
	Platform       string         `gorm:"size:64;index" json:"platform"`
	ShopID         *uuid.UUID     `gorm:"type:char(36);index" json:"shopId,omitempty"`
	OrderNo        string         `gorm:"size:128;index" json:"orderNo"`
	CustomerName   string         `gorm:"size:255" json:"customerName"`
	Content        string         `gorm:"type:text;not null" json:"content"`
	MissingVars    datatypes.JSON `gorm:"type:jsonb" json:"missingVars,omitempty"`
	Status         string         `gorm:"size:32;index;not null" json:"status"`
	ConversationID *uuid.UUID     `gorm:"type:char(36);index" json:"conversationId,omitempty"`
	SentAt         *time.Time     `json:"sentAt,omitempty"`
	SentBy         *uuid.UUID     `gorm:"type:char(36)" json:"sentBy,omitempty"`
	IgnoredAt      *time.Time     `json:"ignoredAt,omitempty"`
}

func (BuyerMessageDraft) TableName() string { return "buyer_message_drafts" }
