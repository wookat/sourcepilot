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
	Enabled bool `gorm:"not null;index" json:"enabled"`
	// DefaultLanguage 是 Content 字段所用语言（历史模板默认 zh-CN）；其余语言
	// 内容存放在 customer_reply_template_variants，占位符口径与 Content 一致。
	DefaultLanguage string     `gorm:"size:16;not null;default:zh-CN" json:"defaultLanguage"`
	CreatedBy       *uuid.UUID `gorm:"type:char(36);index" json:"createdBy,omitempty"`
}

func (CustomerReplyTemplate) TableName() string { return "customer_reply_templates" }

// CustomerReplyTemplateVariant is one extra-language content for a template.
// 同一模板同一语言最多一条；默认语言内容仍存在模板 Content 上，不入本表。
type CustomerReplyTemplateVariant struct {
	model.Base
	TenantID   int64     `gorm:"not null;default:0;index;uniqueIndex:idx_reply_tpl_variant_lang,priority:1" json:"tenantId"`
	TemplateID uuid.UUID `gorm:"type:char(36);not null;index;uniqueIndex:idx_reply_tpl_variant_lang,priority:2" json:"templateId"`
	Language   string    `gorm:"size:16;not null;uniqueIndex:idx_reply_tpl_variant_lang,priority:3" json:"language"`
	Content    string    `gorm:"type:text;not null" json:"content"`
}

func (CustomerReplyTemplateVariant) TableName() string { return "customer_reply_template_variants" }

// TemplateVariantRow is the API shape for one language variant.
type TemplateVariantRow struct {
	Language string `json:"language"`
	Content  string `json:"content"`
}

// TemplateRow is the API shape for one template.
type TemplateRow struct {
	ID        uuid.UUID `json:"id"`
	GroupKey  string    `json:"groupKey"`
	Name      string    `json:"name"`
	Content   string    `json:"content"`
	SortOrder int       `json:"sortOrder"`
	Enabled   bool      `json:"enabled"`
	// DefaultLanguage + Variants 支撑多语言模板；Variants 不含默认语言内容。
	DefaultLanguage string               `json:"defaultLanguage"`
	Variants        []TemplateVariantRow `json:"variants"`
	CreatedAt       time.Time            `json:"createdAt"`
	UpdatedAt       time.Time            `json:"updatedAt"`
}

func toTemplateRow(t CustomerReplyTemplate, variants []CustomerReplyTemplateVariant) TemplateRow {
	dl := t.DefaultLanguage
	if dl == "" {
		dl = TemplateDefaultLanguage
	}
	vrows := make([]TemplateVariantRow, 0, len(variants))
	for _, v := range variants {
		vrows = append(vrows, TemplateVariantRow{Language: v.Language, Content: v.Content})
	}
	return TemplateRow{
		ID:              t.ID,
		GroupKey:        t.GroupKey,
		Name:            t.Name,
		Content:         t.Content,
		SortOrder:       t.SortOrder,
		Enabled:         t.Enabled,
		DefaultLanguage: dl,
		Variants:        vrows,
		CreatedAt:       t.CreatedAt,
		UpdatedAt:       t.UpdatedAt,
	}
}
