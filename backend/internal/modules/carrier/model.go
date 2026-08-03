// Package carrier manages tenant-scoped logistics carriers (物流商) used by
// order shipments. Presets cover common domestic couriers; tenants may add
// custom carriers and enable/disable any entry.
package carrier

import (
	"github.com/trademind-ai/trademind/backend/internal/pkg/model"
)

// Carrier is one logistics carrier a tenant can ship with.
type Carrier struct {
	model.HardDeleteBase
	TenantID int64  `gorm:"default:0;uniqueIndex:ux_carriers_tenant_code" json:"tenantId"`
	Code     string `gorm:"size:64;not null;uniqueIndex:ux_carriers_tenant_code" json:"code"`
	Name     string `gorm:"size:128;not null" json:"name"`
	Enabled  bool   `gorm:"default:true" json:"enabled"`
	// IsPreset marks built-in carriers seeded per tenant; presets can be
	// disabled but not deleted or re-coded.
	IsPreset bool `gorm:"default:false" json:"isPreset"`
	// TrackingURLTemplate builds a tracking link with {trackingNo} placeholder.
	TrackingURLTemplate string `gorm:"type:text" json:"trackingUrlTemplate,omitempty"`
	SortOrder           int    `gorm:"default:0" json:"sortOrder"`
}

// TableName maps to carriers.
func (Carrier) TableName() string { return "carriers" }
