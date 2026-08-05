package inventory

import (
	"context"
	"testing"

	"github.com/google/uuid"
)

// 自动分仓联动：an order's assigned warehouse pins the deduction unless the
// caller explicitly picks another warehouse (manual override wins).
func TestDeductPinsToAssignedWarehouse(t *testing.T) {
	svc, orderID, skuID := setupCycleFixture(t)
	db := svc.DB

	// Second warehouse holding the single unit; assign it to the order.
	var sku struct{ ProductID uuid.UUID }
	if err := db.Table("product_skus").Select("product_id").Where("id = ?", skuID).Scan(&sku).Error; err != nil {
		t.Fatal(err)
	}
	wh := Warehouse{TenantID: 0, Code: "WH-A", Name: "分仓A", Enabled: true, Priority: 10}
	if err := db.Create(&wh).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Create(&WarehouseStock{WarehouseID: wh.ID,
		ProductID: sku.ProductID, ProductSKUID: skuID, Stock: 1}).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Model(&orderMirror{}).Where("id = ?", orderID).
		Update("assigned_warehouse_id", wh.ID).Error; err != nil {
		t.Fatal(err)
	}

	if _, err := svc.DeductInventoryForOrder(context.Background(), orderID, OrderInventoryOptions{
		Reason: "manual_api",
	}); err != nil {
		t.Fatal(err)
	}
	var logs []InventoryChangeLog
	if err := db.Where("product_sku_id = ? AND delta < 0", skuID).Find(&logs).Error; err != nil {
		t.Fatal(err)
	}
	if len(logs) == 0 {
		t.Fatal("expected deduction change logs")
	}
	for _, l := range logs {
		if l.WarehouseID == nil || *l.WarehouseID != wh.ID {
			t.Fatalf("deduction must pin to assigned warehouse, got %+v", l)
		}
	}
}
