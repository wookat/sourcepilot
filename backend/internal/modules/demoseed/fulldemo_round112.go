package demoseed

import (
	"fmt"
	"time"

	"gorm.io/gorm"

	"github.com/trademind-ai/trademind/backend/internal/modules/inventory"
	"github.com/trademind-ai/trademind/backend/internal/modules/product"
)

// Round 112 demo warehouse names/codes (all cleanup-safe: DEMO- prefixed).
const (
	demoWarehouseCode = "DEMO-WH-2"
	demoWarehouseName = "DEMO-华南仓"
)

// seedRound112Warehouses adds the second warehouse plus sample multi-warehouse
// data: warehouse stock rows, one transfer ledger pair (out/in) and one
// warehouse-tagged purchase inbound ledger row, so the multi-warehouse flows
// (center filter, transfer history, per-warehouse report) demo out of the box.
func (s *FullDemoSeeder) seedRound112Warehouses(tx *gorm.DB, res *FullDemoResult, now time.Time, skus []product.ProductSKU) error {
	count := func(table string, n int64) { res.Counts[table] += n }
	if len(skus) < 2 {
		return nil
	}

	// Default warehouse: reuse the tenant's row or create it (same shape as
	// the round 112 migration backfill).
	var def inventory.Warehouse
	err := tx.Where("tenant_id = ? AND is_default = ?", s.TenantID, true).First(&def).Error
	if err == gorm.ErrRecordNotFound {
		def = inventory.Warehouse{TenantID: s.TenantID,
			Code: inventory.DefaultWarehouseCode, Name: inventory.DefaultWarehouseName,
			IsDefault: true, Enabled: true}
		// Not counted: the default warehouse is real tenant data (kept by cleanup).
		if err := tx.Create(&def).Error; err != nil {
			return fmt.Errorf("demoseed: default warehouse: %w", err)
		}
	} else if err != nil {
		return fmt.Errorf("demoseed: default warehouse: %w", err)
	}

	wh := inventory.Warehouse{TenantID: s.TenantID, Code: demoWarehouseCode,
		Name: demoWarehouseName, Enabled: true, Priority: 10,
		Remark: "DEMO- 演示第二仓（种子数据）"}
	if err := tx.Create(&wh).Error; err != nil {
		return fmt.Errorf("demoseed: demo warehouse: %w", err)
	}
	count("warehouses", 1)

	// Transfer sample: move 6 units of skus[0] default → demo warehouse
	// (two ledger rows, SKU total unchanged).
	skuA := skus[0]
	totalA := 0
	if skuA.Stock != nil {
		totalA = *skuA.Stock
	}
	transferQty := 6
	outLog := inventory.InventoryChangeLog{TenantID: s.TenantID,
		ProductID: skuA.ProductID, ProductSKUID: skuA.ID,
		ChangeType: inventory.ChangeTransferOut, WarehouseID: &def.ID,
		BeforeStock: totalA, AfterStock: totalA - transferQty, Delta: -transferQty,
		Reason: "warehouse_transfer", Remark: "DEMO- 调拨出库：默认仓 → " + demoWarehouseName + "(种子数据)",
		BusinessEventKey: "DEMO-EVT-TRANSFER-1:out"}
	inLog := inventory.InventoryChangeLog{TenantID: s.TenantID,
		ProductID: skuA.ProductID, ProductSKUID: skuA.ID,
		ChangeType: inventory.ChangeTransferIn, WarehouseID: &wh.ID,
		BeforeStock: 0, AfterStock: transferQty, Delta: transferQty,
		Reason: "warehouse_transfer", Remark: "DEMO- 调拨入库：默认仓 → " + demoWarehouseName + "(种子数据)",
		BusinessEventKey: "DEMO-EVT-TRANSFER-1:in"}
	if err := tx.Create(&outLog).Error; err != nil {
		return fmt.Errorf("demoseed: transfer out log: %w", err)
	}
	if err := tx.Create(&inLog).Error; err != nil {
		return fmt.Errorf("demoseed: transfer in log: %w", err)
	}
	count("inventory_change_logs", 2)
	if err := tx.Create(&inventory.WarehouseStock{TenantID: s.TenantID,
		WarehouseID: wh.ID, ProductID: skuA.ProductID, ProductSKUID: skuA.ID,
		Stock: transferQty}).Error; err != nil {
		return fmt.Errorf("demoseed: warehouse stock A: %w", err)
	}
	count("warehouse_stocks", 1)

	// Inbound sample: 4 units of skus[1] received straight into the demo
	// warehouse (SKU total grows by 4).
	skuB := skus[1]
	totalB := 0
	if skuB.Stock != nil {
		totalB = *skuB.Stock
	}
	inboundQty := 4
	inbound := inventory.InventoryChangeLog{TenantID: s.TenantID,
		ProductID: skuB.ProductID, ProductSKUID: skuB.ID,
		ChangeType: inventory.ChangePurchaseInbound, WarehouseID: &wh.ID,
		BeforeStock: totalB, AfterStock: totalB + inboundQty, Delta: inboundQty,
		Reason: "purchase_delivered", Remark: "DEMO- 采购签收入库到 " + demoWarehouseName + "（种子数据）",
		BusinessEventKey: "DEMO-EVT-WH-INBOUND-1"}
	if err := tx.Create(&inbound).Error; err != nil {
		return fmt.Errorf("demoseed: warehouse inbound log: %w", err)
	}
	count("inventory_change_logs", 1)
	if err := tx.Create(&inventory.WarehouseStock{TenantID: s.TenantID,
		WarehouseID: wh.ID, ProductID: skuB.ProductID, ProductSKUID: skuB.ID,
		Stock: inboundQty}).Error; err != nil {
		return fmt.Errorf("demoseed: warehouse stock B: %w", err)
	}
	count("warehouse_stocks", 1)
	if err := tx.Model(&product.ProductSKU{}).Where("id = ?", skuB.ID).
		Update("stock", totalB+inboundQty).Error; err != nil {
		return fmt.Errorf("demoseed: warehouse inbound stock: %w", err)
	}
	return nil
}
