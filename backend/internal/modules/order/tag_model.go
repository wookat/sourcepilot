package order

import (
	"github.com/google/uuid"
	"github.com/trademind-ai/trademind/backend/internal/pkg/model"
)

// OrderTag is one tenant-level订单标签 (名称 + 颜色). Tags label orders for
// filtering / batch operations; they never change order business state.
type OrderTag struct {
	model.HardDeleteBase
	TenantID int64  `gorm:"not null;default:0;uniqueIndex:idx_order_tags_tenant_name" json:"tenantId"`
	Name     string `gorm:"size:64;not null;uniqueIndex:idx_order_tags_tenant_name" json:"name"`
	// Color is an antd tag color token (blue / green / red / ...) or a hex
	// value; the admin UI renders it as-is.
	Color string `gorm:"size:32;not null;default:blue" json:"color"`
}

// TableName maps to order_tags.
func (OrderTag) TableName() string { return "order_tags" }

// OrderTagLink attaches one tag to one order. The (order_id, tag_id) unique
// index makes打标 idempotent (重复打标 / 批量重复提交不产生重复行).
type OrderTagLink struct {
	model.HardDeleteBase
	TenantID int64     `gorm:"not null;default:0;index" json:"tenantId"`
	OrderID  uuid.UUID `gorm:"type:char(36);not null;uniqueIndex:idx_order_tag_links_order_tag" json:"orderId"`
	TagID    uuid.UUID `gorm:"type:char(36);not null;uniqueIndex:idx_order_tag_links_order_tag;index" json:"tagId"`
	// Source records who attached the tag: manual (手工/批量) or automation.
	Source string `gorm:"size:16;not null;default:manual" json:"source"`
}

// TableName maps to order_tag_links.
func (OrderTagLink) TableName() string { return "order_tag_links" }

// Tag link sources.
const (
	TagLinkSourceManual     = "manual"
	TagLinkSourceAutomation = "automation"
)

// OrderTagBrief is the compact tag shape embedded in order list rows.
type OrderTagBrief struct {
	ID    uuid.UUID `json:"id"`
	Name  string    `json:"name"`
	Color string    `json:"color"`
}
