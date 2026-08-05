package inventory

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"testing"

	"github.com/glebarez/sqlite"
	"github.com/google/uuid"
	"gorm.io/gorm"

	"github.com/trademind-ai/trademind/backend/internal/modules/product"
)

func openWarehouseTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	dsn := filepath.Join(t.TempDir(), "warehouse.db")
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Skipf("sqlite unavailable: %v", err)
	}
	if err := db.AutoMigrate(
		&product.Product{}, &product.ProductSKU{},
		&Warehouse{}, &WarehouseStock{}, &InventoryChangeLog{},
	); err != nil {
		t.Fatal(err)
	}
	return db
}

func createWarehouseTestSKU(t *testing.T, db *gorm.DB, tenantID int64, stock int) product.ProductSKU {
	t.Helper()
	p := product.Product{TenantID: tenantID, Title: fmt.Sprintf("wh product %d", tenantID), Status: "draft"}
	if err := db.Create(&p).Error; err != nil {
		t.Fatal(err)
	}
	sku := product.ProductSKU{ProductID: p.ID, SKUCode: uuid.NewString()[:12], Stock: &stock}
	if err := db.Create(&sku).Error; err != nil {
		t.Fatal(err)
	}
	return sku
}

// Warehouse lifecycle: default warehouse is auto-created, locked against
// delete/disable; non-default warehouses are tenant CRUD-able.
func TestWarehouseLifecycleAndDefaultLock(t *testing.T) {
	db := openWarehouseTestDB(t)
	svc := &Service{DB: db}
	ctx := context.Background()

	rows, err := svc.ListWarehouses(ctx, 1)
	if err != nil || len(rows) != 1 || !rows[0].IsDefault {
		t.Fatalf("default warehouse: %v %+v", err, rows)
	}
	def := rows[0]

	if err := svc.DeleteWarehouse(ctx, 1, def.ID); !errors.Is(err, ErrDefaultWarehouseLocked) {
		t.Fatalf("delete default: want locked, got %v", err)
	}
	off := false
	if _, err := svc.UpdateWarehouse(ctx, 1, def.ID, UpdateWarehouseBody{Enabled: &off}); !errors.Is(err, ErrDefaultWarehouseLocked) {
		t.Fatalf("disable default: want locked, got %v", err)
	}

	wh, err := svc.CreateWarehouse(ctx, 1, CreateWarehouseBody{Code: "south", Name: "华南仓", Priority: 10})
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	if _, err := svc.CreateWarehouse(ctx, 1, CreateWarehouseBody{Code: "south", Name: "重复编码"}); !errors.Is(err, ErrWarehouseCodeConflict) {
		t.Fatalf("dup code: want conflict, got %v", err)
	}
	if _, err := svc.CreateWarehouse(ctx, 1, CreateWarehouseBody{Code: DefaultWarehouseCode, Name: "假默认仓"}); !errors.Is(err, ErrWarehouseCodeConflict) {
		t.Fatalf("reserved code: want conflict, got %v", err)
	}
	name := "华南一号仓"
	upd, err := svc.UpdateWarehouse(ctx, 1, wh.ID, UpdateWarehouseBody{Name: &name, Enabled: &off})
	if err != nil || upd.Name != name || upd.Enabled {
		t.Fatalf("update: %v %+v", err, upd)
	}
	if err := svc.DeleteWarehouse(ctx, 1, wh.ID); err != nil {
		t.Fatalf("delete: %v", err)
	}
	if _, err := svc.GetWarehouse(ctx, 1, wh.ID); !errors.Is(err, ErrWarehouseNotFound) {
		t.Fatalf("deleted get: want not found, got %v", err)
	}
}

// Tenant isolation: warehouses of tenant 1 must be invisible (404 semantics)
// to tenant 2 on read, update, delete and transfer.
func TestWarehouseTenantIsolation(t *testing.T) {
	db := openWarehouseTestDB(t)
	svc := &Service{DB: db}
	ctx := context.Background()

	wh, err := svc.CreateWarehouse(ctx, 1, CreateWarehouseBody{Code: "east", Name: "华东仓"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := svc.GetWarehouse(ctx, 2, wh.ID); !errors.Is(err, ErrWarehouseNotFound) {
		t.Fatalf("cross-tenant get: want not found, got %v", err)
	}
	name := "越权改名"
	if _, err := svc.UpdateWarehouse(ctx, 2, wh.ID, UpdateWarehouseBody{Name: &name}); !errors.Is(err, ErrWarehouseNotFound) {
		t.Fatalf("cross-tenant update: want not found, got %v", err)
	}
	if err := svc.DeleteWarehouse(ctx, 2, wh.ID); !errors.Is(err, ErrWarehouseNotFound) {
		t.Fatalf("cross-tenant delete: want not found, got %v", err)
	}

	rows, err := svc.ListWarehouses(ctx, 2)
	if err != nil {
		t.Fatal(err)
	}
	for _, r := range rows {
		if r.ID == wh.ID {
			t.Fatalf("cross-tenant list leaked warehouse %s", wh.ID)
		}
	}

	// Cross-tenant SKU: transfer must fail with not-found semantics.
	sku := createWarehouseTestSKU(t, db, 1, 10)
	def2, err := svc.EnsureDefaultWarehouse(ctx, 2)
	if err != nil {
		t.Fatal(err)
	}
	wh2, err := svc.CreateWarehouse(ctx, 2, CreateWarehouseBody{Code: "t2", Name: "租户2仓"})
	if err != nil {
		t.Fatal(err)
	}
	_, err = svc.TransferStock(ctx, 2, TransferStockBody{
		ProductSKUID:    sku.ID.String(),
		FromWarehouseID: def2.ID.String(),
		ToWarehouseID:   wh2.ID.String(),
		Quantity:        1,
	}, nil)
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		t.Fatalf("cross-tenant sku transfer: want record not found, got %v", err)
	}
}

// Transfer happy path: source decreases, target increases, SKU total
// unchanged, exactly two ledger rows in one transaction.
func TestTransferStockAtomicity(t *testing.T) {
	db := openWarehouseTestDB(t)
	svc := &Service{DB: db}
	ctx := context.Background()

	sku := createWarehouseTestSKU(t, db, 1, 20)
	def, err := svc.EnsureDefaultWarehouse(ctx, 1)
	if err != nil {
		t.Fatal(err)
	}
	wh, err := svc.CreateWarehouse(ctx, 1, CreateWarehouseBody{Code: "south", Name: "华南仓"})
	if err != nil {
		t.Fatal(err)
	}

	res, err := svc.TransferStock(ctx, 1, TransferStockBody{
		ProductSKUID:    sku.ID.String(),
		FromWarehouseID: def.ID.String(),
		ToWarehouseID:   wh.ID.String(),
		Quantity:        6,
	}, nil)
	if err != nil {
		t.Fatalf("transfer: %v", err)
	}
	if res.FromBefore != 20 || res.FromAfter != 14 || res.ToBefore != 0 || res.ToAfter != 6 {
		t.Fatalf("transfer amounts: %+v", res)
	}

	var logs int64
	if err := db.Model(&InventoryChangeLog{}).
		Where("business_event_key LIKE ?", "warehouse_transfer:"+res.TransferID.String()+"%").
		Count(&logs).Error; err != nil || logs != 2 {
		t.Fatalf("ledger rows: %v %d", err, logs)
	}
	var skuRow product.ProductSKU
	if err := db.First(&skuRow, "id = ?", sku.ID).Error; err != nil || derefStock(skuRow.Stock) != 20 {
		t.Fatalf("sku total changed: %v %v", err, skuRow.Stock)
	}
	entries, err := svc.WarehouseStocksForSKU(ctx, 1, sku.ID, 20)
	if err != nil {
		t.Fatal(err)
	}
	got := map[uuid.UUID]int{}
	for _, e := range entries {
		got[e.WarehouseID] = e.Stock
	}
	if got[def.ID] != 14 || got[wh.ID] != 6 {
		t.Fatalf("breakdown: %+v", entries)
	}

	// Insufficient source stock: whole transfer rolls back, no ledger rows.
	var before int64
	if err := db.Model(&InventoryChangeLog{}).Count(&before).Error; err != nil {
		t.Fatal(err)
	}
	if _, err := svc.TransferStock(ctx, 1, TransferStockBody{
		ProductSKUID:    sku.ID.String(),
		FromWarehouseID: wh.ID.String(),
		ToWarehouseID:   def.ID.String(),
		Quantity:        100,
	}, nil); !errors.Is(err, ErrInsufficientWarehouse) {
		t.Fatalf("insufficient: want error, got %v", err)
	}
	var after int64
	if err := db.Model(&InventoryChangeLog{}).Count(&after).Error; err != nil || after != before {
		t.Fatalf("rollback wrote ledger rows: %v %d != %d", err, after, before)
	}
	entries, err = svc.WarehouseStocksForSKU(ctx, 1, sku.ID, 20)
	if err != nil {
		t.Fatal(err)
	}
	for _, e := range entries {
		if e.WarehouseID == wh.ID && e.Stock != 6 {
			t.Fatalf("rollback changed stock: %+v", entries)
		}
	}

	// Guard rails: same warehouse / non-positive quantity / disabled target.
	if _, err := svc.TransferStock(ctx, 1, TransferStockBody{
		ProductSKUID: sku.ID.String(), FromWarehouseID: wh.ID.String(),
		ToWarehouseID: wh.ID.String(), Quantity: 1}, nil); !errors.Is(err, ErrTransferSameWarehouse) {
		t.Fatalf("same warehouse: got %v", err)
	}
	if _, err := svc.TransferStock(ctx, 1, TransferStockBody{
		ProductSKUID: sku.ID.String(), FromWarehouseID: def.ID.String(),
		ToWarehouseID: wh.ID.String(), Quantity: 0}, nil); !errors.Is(err, ErrTransferInvalidQuantity) {
		t.Fatalf("zero qty: got %v", err)
	}
	off := false
	if _, err := svc.UpdateWarehouse(ctx, 1, wh.ID, UpdateWarehouseBody{Enabled: &off}); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.TransferStock(ctx, 1, TransferStockBody{
		ProductSKUID: sku.ID.String(), FromWarehouseID: def.ID.String(),
		ToWarehouseID: wh.ID.String(), Quantity: 1}, nil); !errors.Is(err, ErrWarehouseDisabled) {
		t.Fatalf("disabled target: got %v", err)
	}
}

// Migration precheck: legacy stock stays consistent (derived default) and the
// preview flags broken invariants.
func TestWarehouseMigrationPreview(t *testing.T) {
	db := openWarehouseTestDB(t)
	svc := &Service{DB: db}
	ctx := context.Background()

	sku := createWarehouseTestSKU(t, db, 1, 15)
	if _, err := svc.EnsureDefaultWarehouse(ctx, 1); err != nil {
		t.Fatal(err)
	}
	prev, err := svc.PreviewWarehouseMigration(ctx, 1)
	if err != nil {
		t.Fatal(err)
	}
	if !prev.DefaultWarehouseExists || prev.TotalStock != 15 || prev.DefaultDerivedStock != 15 || !prev.Consistent {
		t.Fatalf("preview: %+v", prev)
	}

	// Over-allocated non-default stock must be flagged as inconsistent.
	wh, err := svc.CreateWarehouse(ctx, 1, CreateWarehouseBody{Code: "x", Name: "X仓"})
	if err != nil {
		t.Fatal(err)
	}
	if err := db.Create(&WarehouseStock{TenantID: 1, WarehouseID: wh.ID,
		ProductID: sku.ProductID, ProductSKUID: sku.ID, Stock: 99}).Error; err != nil {
		t.Fatal(err)
	}
	prev, err = svc.PreviewWarehouseMigration(ctx, 1)
	if err != nil {
		t.Fatal(err)
	}
	if prev.NegativeDerivedSKUs != 1 || prev.Consistent {
		t.Fatalf("inconsistent preview: %+v", prev)
	}
}
