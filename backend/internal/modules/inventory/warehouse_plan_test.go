package inventory

import (
	"context"
	"errors"
	"testing"
)

func TestPlanOrderWarehouseDefaultStrategy(t *testing.T) {
	db := openWarehouseTestDB(t)
	svc := &Service{DB: db}
	ctx := context.Background()
	sku := createWarehouseTestSKU(t, db, 1, 10)

	plan, err := svc.PlanOrderWarehouse(ctx, 1, "", []WarehouseDemand{
		{ProductSKUID: sku.ID, SKUCode: sku.SKUCode, Quantity: 3},
	})
	if err != nil {
		t.Fatal(err)
	}
	if !plan.IsDefault || plan.WarehouseName == "" {
		t.Fatalf("expected default warehouse plan, got %+v", plan)
	}

	// Insufficient stock: visible error, wraps ErrPlanInsufficientStock.
	_, err = svc.PlanOrderWarehouse(ctx, 1, WarehousePlanStrategyDefault, []WarehouseDemand{
		{ProductSKUID: sku.ID, SKUCode: sku.SKUCode, Quantity: 99},
	})
	if !errors.Is(err, ErrPlanInsufficientStock) {
		t.Fatalf("expected insufficient stock error, got %v", err)
	}
}

func TestPlanOrderWarehouseStockFirst(t *testing.T) {
	db := openWarehouseTestDB(t)
	svc := &Service{DB: db}
	ctx := context.Background()
	sku := createWarehouseTestSKU(t, db, 1, 10)
	def, err := svc.EnsureDefaultWarehouse(ctx, 1)
	if err != nil {
		t.Fatal(err)
	}
	// Second warehouse holds 8 of the 10 units → default only has 2 available.
	wh := Warehouse{TenantID: 1, Code: "WH-2", Name: "华南仓", Enabled: true, Priority: 10}
	if err := db.Create(&wh).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Create(&WarehouseStock{TenantID: 1, WarehouseID: wh.ID,
		ProductID: sku.ProductID, ProductSKUID: sku.ID, Stock: 8}).Error; err != nil {
		t.Fatal(err)
	}

	// Needs 5: default (avail 2) cannot cover, stock_first falls through to WH-2.
	plan, err := svc.PlanOrderWarehouse(ctx, 1, WarehousePlanStrategyStockFirst, []WarehouseDemand{
		{ProductSKUID: sku.ID, SKUCode: sku.SKUCode, Quantity: 5},
	})
	if err != nil {
		t.Fatal(err)
	}
	if plan.WarehouseID != wh.ID {
		t.Fatalf("expected stock-first pick WH-2, got %+v (default %s)", plan, def.ID)
	}

	// Needs 2: default warehouse (higher priority) covers it and wins.
	plan, err = svc.PlanOrderWarehouse(ctx, 1, WarehousePlanStrategyStockFirst, []WarehouseDemand{
		{ProductSKUID: sku.ID, SKUCode: sku.SKUCode, Quantity: 2},
	})
	if err != nil {
		t.Fatal(err)
	}
	if plan.WarehouseID != def.ID {
		t.Fatalf("expected default warehouse pick, got %+v", plan)
	}

	// Needs 9: no single warehouse covers → visible insufficient-stock error.
	_, err = svc.PlanOrderWarehouse(ctx, 1, WarehousePlanStrategyStockFirst, []WarehouseDemand{
		{ProductSKUID: sku.ID, SKUCode: sku.SKUCode, Quantity: 9},
	})
	if !errors.Is(err, ErrPlanInsufficientStock) {
		t.Fatalf("expected insufficient stock error, got %v", err)
	}
}

func TestPlanOrderWarehouseTenantScope(t *testing.T) {
	db := openWarehouseTestDB(t)
	svc := &Service{DB: db}
	ctx := context.Background()
	sku := createWarehouseTestSKU(t, db, 2, 10)

	// Tenant 1 must not plan against tenant 2's SKU.
	if _, err := svc.PlanOrderWarehouse(ctx, 1, "", []WarehouseDemand{
		{ProductSKUID: sku.ID, SKUCode: sku.SKUCode, Quantity: 1},
	}); err == nil {
		t.Fatal("expected error planning another tenant's SKU")
	}

	// Unknown strategy is rejected.
	if _, err := svc.PlanOrderWarehouse(ctx, 2, "bogus", []WarehouseDemand{
		{ProductSKUID: sku.ID, SKUCode: sku.SKUCode, Quantity: 1},
	}); err == nil {
		t.Fatal("expected error for unknown strategy")
	}

	// Empty demand is rejected.
	if _, err := svc.PlanOrderWarehouse(ctx, 2, "", nil); err == nil {
		t.Fatal("expected error for empty demand")
	}
}
