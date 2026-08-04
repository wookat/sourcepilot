// Package bannedwords manages tenant-scoped listing banned-word libraries
// (违禁词库) and scans product draft copy (title / selling points / detail)
// before publishing. Presets cover 广告法极限词 and common prohibited terms;
// tenants may add custom words and enable/disable whole categories.
package bannedwords

import (
	"github.com/trademind-ai/trademind/backend/internal/pkg/model"
)

// Word severity levels.
const (
	LevelForbidden = "forbidden" // 禁止级：命中阻断刊登
	LevelWarning   = "warning"   // 警告级：提示但不阻断
)

// Built-in categories.
const (
	CategoryAdExtreme    = "ad_extreme"   // 广告法极限词
	CategoryGeneral      = "general"      // 通用违禁词
	CategoryMedical      = "medical"      // 医疗功效词
	CategoryInfringement = "infringement" // 品牌侵权词
)

// BannedWord is one banned word entry in a tenant's library.
type BannedWord struct {
	model.HardDeleteBase
	TenantID int64  `gorm:"default:0;uniqueIndex:ux_banned_words_tenant_word" json:"tenantId"`
	Word     string `gorm:"size:128;not null;uniqueIndex:ux_banned_words_tenant_word" json:"word"`
	Category string `gorm:"size:32;index;not null" json:"category"`
	// Level is forbidden (blocks publishing) or warning (hint only).
	Level string `gorm:"size:16;not null" json:"level"`
	// IsPreset marks built-in words seeded per tenant; presets can be
	// disabled but not edited or deleted.
	IsPreset bool `gorm:"default:false" json:"isPreset"`
	Enabled  bool `gorm:"default:true" json:"enabled"`
	// Suggestion is the recommended replacement or fix hint shown on hits.
	Suggestion string `gorm:"size:512" json:"suggestion,omitempty"`
}

// TableName maps to banned_words.
func (BannedWord) TableName() string { return "banned_words" }

// BannedWordCategoryState stores a tenant's enable/disable switch per category.
type BannedWordCategoryState struct {
	model.HardDeleteBase
	TenantID int64  `gorm:"default:0;uniqueIndex:ux_banned_word_categories_tenant" json:"tenantId"`
	Category string `gorm:"size:32;not null;uniqueIndex:ux_banned_word_categories_tenant" json:"category"`
	Enabled  bool   `gorm:"default:true" json:"enabled"`
}

// TableName maps to banned_word_category_states.
func (BannedWordCategoryState) TableName() string { return "banned_word_category_states" }
