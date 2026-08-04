// Package waybill manages tenant-scoped waybill print templates (自定义打印
// 模板，非电子面单) and shipping rules (按条件推荐物流商). Templates control
// how /orders/print renders (size, sections, header/footer); rules recommend
// a carrier for shipping flows without forcing it. No real waybill / 电子面单
// platform API is involved; carrier booking stays manual until credentials
// land.
package waybill

import (
	"github.com/trademind-ai/trademind/backend/internal/pkg/model"
	"gorm.io/datatypes"
)

// Template size codes (打印纸张规格).
const (
	Size100x180 = "100x180" // 100×180mm 标准面单
	Size100x150 = "100x150" // 100×150mm 小号面单
	SizeA4List  = "a4_list" // A4 一联单（拣货/发货清单式）
)

// ValidSizes lists accepted template size codes.
func ValidSizes() []string { return []string{Size100x180, Size100x150, SizeA4List} }

// Template is one tenant waybill print template.
type Template struct {
	model.HardDeleteBase
	TenantID int64  `gorm:"default:0;index:idx_waybill_templates_tenant" json:"tenantId"`
	Name     string `gorm:"size:128;not null" json:"name"`
	// SizeCode is one of ValidSizes().
	SizeCode string `gorm:"size:32;not null" json:"sizeCode"`
	// Section toggles (显示字段勾选).
	ShowRecipient   bool `gorm:"default:true" json:"showRecipient"`
	ShowSender      bool `gorm:"default:true" json:"showSender"`
	ShowItems       bool `gorm:"default:true" json:"showItems"`
	ShowRemark      bool `gorm:"default:true" json:"showRemark"`
	ShowCarrierLogo bool `gorm:"default:false" json:"showCarrierLogo"`
	// HeaderText / FooterText are custom print texts (页眉 / 页脚).
	HeaderText string `gorm:"size:512" json:"headerText,omitempty"`
	FooterText string `gorm:"size:512" json:"footerText,omitempty"`
	// IsDefault marks the tenant's default template for print flows.
	IsDefault bool `gorm:"default:false" json:"isDefault"`
	// IsPreset marks built-in templates seeded per tenant; presets can be
	// edited but not deleted.
	IsPreset  bool `gorm:"default:false" json:"isPreset"`
	SortOrder int  `gorm:"default:0" json:"sortOrder"`
}

// TableName maps to waybill_templates.
func (Template) TableName() string { return "waybill_templates" }

// ShippingRule recommends a carrier when its conditions match an order.
// Empty condition fields mean "any"; a rule with a condition the order's
// known attributes cannot satisfy does not match.
type ShippingRule struct {
	model.HardDeleteBase
	TenantID int64  `gorm:"default:0;index:idx_shipping_rules_tenant" json:"tenantId"`
	Name     string `gorm:"size:128;not null" json:"name"`
	// Priority orders evaluation (smaller first).
	Priority int  `gorm:"default:0;index" json:"priority"`
	Enabled  bool `gorm:"default:true" json:"enabled"`
	// Provinces is a JSON array of destination province names (目的省份).
	Provinces datatypes.JSON `gorm:"type:jsonb" json:"provinces,omitempty"`
	// Platforms is a JSON array of order platforms (douyin_shop / manual…).
	Platforms datatypes.JSON `gorm:"type:jsonb" json:"platforms,omitempty"`
	// Weight range in kg (重量段), nil = unbounded.
	MinWeightKg *float64 `gorm:"type:decimal(10,3)" json:"minWeightKg,omitempty"`
	MaxWeightKg *float64 `gorm:"type:decimal(10,3)" json:"maxWeightKg,omitempty"`
	// Order amount range (金额段, order currency), nil = unbounded.
	MinAmount *float64 `gorm:"type:decimal(18,4)" json:"minAmount,omitempty"`
	MaxAmount *float64 `gorm:"type:decimal(18,4)" json:"maxAmount,omitempty"`
	// CarrierCode is the tenant carrier recommended on match.
	CarrierCode string `gorm:"size:64;not null" json:"carrierCode"`
}

// TableName maps to shipping_rules.
func (ShippingRule) TableName() string { return "shipping_rules" }
