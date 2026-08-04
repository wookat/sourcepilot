package customerchat

import (
	"time"

	"github.com/google/uuid"
	"github.com/trademind-ai/trademind/backend/internal/pkg/model"
)

// Template groups (customer-service reply template library).
const (
	TemplateGroupPresale   = "presale"
	TemplateGroupAftersale = "aftersale"
	TemplateGroupLogistics = "logistics"
	TemplateGroupRefund    = "refund"
	TemplateGroupOther     = "other"
)

// TemplateGroups lists all valid template group keys.
var TemplateGroups = []string{
	TemplateGroupPresale,
	TemplateGroupAftersale,
	TemplateGroupLogistics,
	TemplateGroupRefund,
	TemplateGroupOther,
}

// IsValidTemplateGroup reports whether g is a known template group key.
func IsValidTemplateGroup(g string) bool {
	for _, k := range TemplateGroups {
		if k == g {
			return true
		}
	}
	return false
}

// CustomerReplyTemplate is one tenant-scoped canned reply. Content may carry
// variable placeholders like {订单号} / {买家昵称}; the Admin UI fills them from
// the current conversation context before inserting into the reply box.
type CustomerReplyTemplate struct {
	model.Base
	TenantID  int64  `gorm:"not null;default:0;index" json:"tenantId"`
	GroupKey  string `gorm:"size:32;index;not null" json:"groupKey"`
	Name      string `gorm:"size:255;not null" json:"name"`
	Content   string `gorm:"type:text;not null" json:"content"`
	SortOrder int    `gorm:"not null;default:0;index" json:"sortOrder"`
	// 无 default tag：带 default 时 GORM 会在 Create 时跳过零值 false 导致落库为 true；
	// 默认启用语义由 service 层（Create 未传 enabled 时置 true）保证。
	Enabled   bool       `gorm:"not null;index" json:"enabled"`
	CreatedBy *uuid.UUID `gorm:"type:char(36);index" json:"createdBy,omitempty"`
}

func (CustomerReplyTemplate) TableName() string { return "customer_reply_templates" }

// TemplateRow is the API shape for one template.
type TemplateRow struct {
	ID        uuid.UUID `json:"id"`
	GroupKey  string    `json:"groupKey"`
	Name      string    `json:"name"`
	Content   string    `json:"content"`
	SortOrder int       `json:"sortOrder"`
	Enabled   bool      `json:"enabled"`
	CreatedAt time.Time `json:"createdAt"`
	UpdatedAt time.Time `json:"updatedAt"`
}

func toTemplateRow(t CustomerReplyTemplate) TemplateRow {
	return TemplateRow{
		ID:        t.ID,
		GroupKey:  t.GroupKey,
		Name:      t.Name,
		Content:   t.Content,
		SortOrder: t.SortOrder,
		Enabled:   t.Enabled,
		CreatedAt: t.CreatedAt,
		UpdatedAt: t.UpdatedAt,
	}
}
