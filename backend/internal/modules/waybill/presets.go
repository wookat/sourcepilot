package waybill

import (
	"context"
	"fmt"

	"gorm.io/gorm"
)

// TemplatePreset is one built-in print template seeded for every tenant.
type TemplatePreset struct {
	Name       string
	SizeCode   string
	ShowLogo   bool
	FooterText string
	IsDefault  bool
	SortOrder  int
}

// TemplatePresets lists built-in templates covering the three supported sizes.
func TemplatePresets() []TemplatePreset {
	return []TemplatePreset{
		{Name: "标准面单 100×180", SizeCode: Size100x180, ShowLogo: true, IsDefault: true, SortOrder: 10},
		{Name: "小号面单 100×150", SizeCode: Size100x150, ShowLogo: true, SortOrder: 20},
		{Name: "A4 一联单", SizeCode: SizeA4List, SortOrder: 30},
	}
}

// EnsureTemplatePresets idempotently seeds built-in templates for one tenant.
// Matching is by (tenant, preset name): presets remain editable, so only
// missing names are inserted.
func EnsureTemplatePresets(ctx context.Context, db *gorm.DB, tenantID int64) error {
	if db == nil {
		return fmt.Errorf("waybill: no db")
	}
	for _, p := range TemplatePresets() {
		var exists int64
		if err := db.WithContext(ctx).Model(&Template{}).
			Where("tenant_id = ? AND is_preset = ? AND name = ?", tenantID, true, p.Name).
			Count(&exists).Error; err != nil {
			return err
		}
		if exists > 0 {
			continue
		}
		row := Template{
			TenantID:        tenantID,
			Name:            p.Name,
			SizeCode:        p.SizeCode,
			ShowRecipient:   true,
			ShowSender:      true,
			ShowItems:       true,
			ShowRemark:      true,
			ShowCarrierLogo: p.ShowLogo,
			FooterText:      p.FooterText,
			IsDefault:       false,
			IsPreset:        true,
			SortOrder:       p.SortOrder,
		}
		if err := db.WithContext(ctx).Create(&row).Error; err != nil {
			return err
		}
		if p.IsDefault {
			// Only claim default when the tenant has none yet.
			var defaults int64
			if err := db.WithContext(ctx).Model(&Template{}).
				Where("tenant_id = ? AND is_default = ?", tenantID, true).
				Count(&defaults).Error; err != nil {
				return err
			}
			if defaults == 0 {
				if err := db.WithContext(ctx).Model(&Template{}).
					Where("id = ?", row.ID).Update("is_default", true).Error; err != nil {
					return err
				}
			}
		}
	}
	return nil
}
