package inventory

import (
	"fmt"
	"strings"

	"github.com/google/uuid"
	"github.com/trademind-ai/trademind/backend/internal/pkg/model"
)

// DefaultWarehouseCode is the reserved code of each tenant's default warehouse.
const DefaultWarehouseCode = "default"

// DefaultWarehouseName is the display name used when auto-creating the default warehouse.
const DefaultWarehouseName = "默认仓"

// Warehouse is a tenant-scoped lightweight warehouse (no bins / waves / pick paths).
// Every tenant has exactly one default warehouse (code "default") that cannot be
// deleted or disabled; its stock is derived as
// product_skus.stock - SUM(non-default warehouse_stocks), so legacy single-warehouse
// data belongs to the default warehouse with zero data movement.
type Warehouse struct {
	model.Base
	TenantID  int64  `gorm:"not null;default:0;index:idx_warehouses_tenant_code" json:"tenantId"`
	Code      string `gorm:"size:64;not null;index:idx_warehouses_tenant_code" json:"code"`
	Name      string `gorm:"size:128;not null" json:"name"`
	IsDefault bool   `gorm:"not null;default:false" json:"isDefault"`
	Enabled   bool   `gorm:"not null;default:true" json:"enabled"`
	// Priority orders deduction allocation (smaller first; default warehouse ties first).
	Priority int    `gorm:"not null;default:0" json:"priority"`
	Remark   string `gorm:"size:255" json:"remark,omitempty"`
}

func (Warehouse) TableName() string { return "warehouses" }

// WarehouseStock stores per-SKU stock for NON-default warehouses only; the
// default warehouse quantity is derived from the SKU total. Rows are unique
// per (warehouse_id, product_sku_id).
type WarehouseStock struct {
	model.HardDeleteBase
	TenantID     int64     `gorm:"not null;default:0;index" json:"tenantId"`
	WarehouseID  uuid.UUID `gorm:"type:char(36);not null;uniqueIndex:idx_warehouse_stocks_wh_sku" json:"warehouseId"`
	ProductID    uuid.UUID `gorm:"type:char(36);index;not null" json:"productId"`
	ProductSKUID uuid.UUID `gorm:"column:product_sku_id;type:char(36);not null;uniqueIndex:idx_warehouse_stocks_wh_sku;index" json:"productSkuId"`
	Stock        int       `gorm:"not null;default:0" json:"stock"`
}

func (WarehouseStock) TableName() string { return "warehouse_stocks" }

// WarehouseStockEntry is one warehouse line in a SKU's stock breakdown.
type WarehouseStockEntry struct {
	WarehouseID   uuid.UUID `json:"warehouseId"`
	WarehouseName string    `json:"warehouseName"`
	IsDefault     bool      `json:"isDefault"`
	Enabled       bool      `json:"enabled"`
	Stock         int       `json:"stock"`
}

// LowStockAlertCopy renders low-stock alert text carrying warehouse names.
// 阈值口径：预警/安全阈值配置在 SKU 级，按全仓总量判定是否告警；文案点名各仓库存，
// 使补货动作可以直接定位到具体仓库。
func LowStockAlertCopy(stocks []WarehouseStockEntry) string {
	if len(stocks) == 0 {
		return "低库存"
	}
	parts := make([]string, 0, len(stocks))
	for _, e := range stocks {
		parts = append(parts, fmt.Sprintf("%s %d", e.WarehouseName, e.Stock))
	}
	return "低库存（" + strings.Join(parts, " / ") + "）"
}
