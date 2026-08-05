package inventory

import (
	"context"
	"fmt"
	"testing"

	"github.com/glebarez/sqlite"
	"github.com/google/uuid"
	"github.com/trademind-ai/trademind/backend/internal/modules/idempotency"
	"github.com/trademind-ai/trademind/backend/internal/modules/product"
	"gorm.io/gorm"
)

func setupCycleFixture(t *testing.T) (*Service, uuid.UUID, uuid.UUID) {
	t.Helper()
	dsn := fmt.Sprintf("file:cycle_%s?mode=memory&cache=shared", uuid.NewString())
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Skipf("sqlite unavailable: %v", err)
	}
	if err := db.AutoMigrate(
		&orderMirror{}, &orderLineMirror{},
		&product.Product{}, &product.ProductSKU{},
		&InventoryChangeLog{}, &OrderInventoryEffect{},
		&Warehouse{}, &WarehouseStock{},
		&idempotency.Record{},
	); err != nil {
		t.Fatal(err)
	}

	p := product.Product{Title: "cycle product"}
	if err := db.Create(&p).Error; err != nil {
		t.Fatal(err)
	}
	stock := 1
	sku := product.ProductSKU{ProductID: p.ID, SKUCode: "CYC-1", Stock: &stock}
	if err := db.Create(&sku).Error; err != nil {
		t.Fatal(err)
	}

	o := orderMirror{OrderNo: "CYC-ORDER-1", Status: "paid", PaymentStatus: "paid"}
	if err := db.Create(&o).Error; err != nil {
		t.Fatal(err)
	}
	item := orderLineMirror{OrderID: o.ID, ProductID: &p.ID, ProductSKUID: &sku.ID, Quantity: 1}
	if err := db.Create(&item).Error; err != nil {
		t.Fatal(err)
	}

	svc := &Service{DB: db, Idempotency: &idempotency.Service{DB: db}}
	return svc, o.ID, sku.ID
}

func skuStock(t *testing.T, db *gorm.DB, skuID uuid.UUID) int {
	t.Helper()
	var sku product.ProductSKU
	if err := db.First(&sku, "id = ?", skuID).Error; err != nil {
		t.Fatal(err)
	}
	return derefStock(sku.Stock)
}

// Regression: deduct → restore → deduct must complete. Previously the deduct
// idempotency key stayed succeeded after the first round, so a re-deduct after
// restore failed with INVENTORY_DEDUCT_KEY_CONFLICT.
func TestDeductRestoreDeductCycle(t *testing.T) {
	svc, orderID, skuID := setupCycleFixture(t)
	ctx := context.Background()

	// Round 1: deduct 1→0.
	sum, err := svc.DeductInventoryForOrder(ctx, orderID, OrderInventoryOptions{Reason: "manual"})
	if err != nil {
		t.Fatal(err)
	}
	if sum.LinesSynced != 1 || skuStock(t, svc.DB, skuID) != 0 {
		t.Fatalf("round1 deduct: %+v stock=%d", sum, skuStock(t, svc.DB, skuID))
	}

	// Repeat deduct: idempotent skip, no double deduct.
	sum, err = svc.DeductInventoryForOrder(ctx, orderID, OrderInventoryOptions{Reason: "manual"})
	if err != nil {
		t.Fatal(err)
	}
	if sum.LinesSynced != 0 || skuStock(t, svc.DB, skuID) != 0 {
		t.Fatalf("repeat deduct not idempotent: %+v stock=%d", sum, skuStock(t, svc.DB, skuID))
	}

	// Restore 0→1.
	rsum, err := svc.RestoreInventoryForOrder(ctx, orderID, OrderInventoryOptions{Reason: "manual_ui"})
	if err != nil {
		t.Fatal(err)
	}
	if rsum.LinesSynced != 1 || skuStock(t, svc.DB, skuID) != 1 {
		t.Fatalf("restore: %+v stock=%d", rsum, skuStock(t, svc.DB, skuID))
	}

	// Repeat restore: idempotent skip.
	rsum, err = svc.RestoreInventoryForOrder(ctx, orderID, OrderInventoryOptions{Reason: "manual_ui"})
	if err != nil {
		t.Fatal(err)
	}
	if rsum.LinesSynced != 0 || skuStock(t, svc.DB, skuID) != 1 {
		t.Fatalf("repeat restore not idempotent: %+v stock=%d", rsum, skuStock(t, svc.DB, skuID))
	}

	// Round 2: deduct after restore must succeed again (was 400 KEY_CONFLICT).
	sum, err = svc.DeductInventoryForOrder(ctx, orderID, OrderInventoryOptions{Reason: "manual_round2"})
	if err != nil {
		t.Fatal(err)
	}
	if sum.LinesSynced != 1 || skuStock(t, svc.DB, skuID) != 0 {
		t.Fatalf("round2 deduct after restore: %+v stock=%d", sum, skuStock(t, svc.DB, skuID))
	}

	// Round 2 restore also works (restore count vs deduct count, not one-shot).
	rsum, err = svc.RestoreInventoryForOrder(ctx, orderID, OrderInventoryOptions{Reason: "manual_ui"})
	if err != nil {
		t.Fatal(err)
	}
	if rsum.LinesSynced != 1 || skuStock(t, svc.DB, skuID) != 1 {
		t.Fatalf("round2 restore: %+v stock=%d", rsum, skuStock(t, svc.DB, skuID))
	}

	// Effect rows hold latest state only: one deduct + one restore success row.
	var deductRows, restoreRows int64
	if err := svc.DB.Model(&OrderInventoryEffect{}).
		Where("order_id = ? AND effect_type = ? AND status = ?", orderID, EffectTypeDeduct, InventoryEffectSuccess).
		Count(&deductRows).Error; err != nil {
		t.Fatal(err)
	}
	if err := svc.DB.Model(&OrderInventoryEffect{}).
		Where("order_id = ? AND effect_type = ? AND status = ?", orderID, EffectTypeRestore, InventoryEffectSuccess).
		Count(&restoreRows).Error; err != nil {
		t.Fatal(err)
	}
	if deductRows != 1 || restoreRows != 1 {
		t.Fatalf("effect rows deduct=%d restore=%d, want 1/1", deductRows, restoreRows)
	}

	// Full audit history stays in inventory_change_logs: two deduct + two restore logs.
	var deductLogs, restoreLogs int64
	if err := svc.DB.Model(&InventoryChangeLog{}).
		Where("ref_order_id = ? AND change_type = ?", orderID, ChangeOrderDeduct).
		Count(&deductLogs).Error; err != nil {
		t.Fatal(err)
	}
	if err := svc.DB.Model(&InventoryChangeLog{}).
		Where("ref_order_id = ? AND change_type = ?", orderID, ChangeOrderCancel).
		Count(&restoreLogs).Error; err != nil {
		t.Fatal(err)
	}
	if deductLogs != 2 || restoreLogs != 2 {
		t.Fatalf("change logs deduct=%d restore=%d, want 2/2", deductLogs, restoreLogs)
	}
}
