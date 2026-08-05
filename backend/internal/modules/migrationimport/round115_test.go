package migrationimport_test

import (
	"testing"

	"github.com/google/uuid"
	"gorm.io/gorm"

	"github.com/trademind-ai/trademind/backend/internal/modules/inventory"
	"github.com/trademind-ai/trademind/backend/internal/modules/migrationimport"
	"github.com/trademind-ai/trademind/backend/internal/modules/order"
	"github.com/trademind-ai/trademind/backend/internal/modules/product"
	"github.com/trademind-ai/trademind/backend/internal/modules/sourcing"
)

func migrateRound115(t *testing.T, db *gorm.DB) {
	t.Helper()
	if err := db.AutoMigrate(
		&migrationimport.ImportMappingPreset{},
		&inventory.Warehouse{}, &inventory.WarehouseStock{}, &inventory.InventoryChangeLog{},
		&sourcing.Supplier{}, &sourcing.ProductSource{}, &sourcing.ProductSourceSKU{}, &sourcing.SourcePriceHistory{},
	); err != nil {
		t.Fatal(err)
	}
}

func newSvc115(db *gorm.DB) *migrationimport.Service {
	return &migrationimport.Service{
		DB:       db,
		Products: &product.Service{DB: db},
		Orders:   &order.Service{DB: db},
		Sourcing: &sourcing.Service{DB: db},
	}
}

func seedSKU(t *testing.T, db *gorm.DB, tenantID int64, skuCode string) product.ProductSKU {
	t.Helper()
	p := &product.Product{TenantID: tenantID, Source: "manual", Title: "测试商品 " + skuCode, Status: "draft", Currency: "CNY"}
	if err := db.Create(p).Error; err != nil {
		t.Fatal(err)
	}
	stock := 0
	sku := product.ProductSKU{ProductID: p.ID, SKUCode: skuCode, SKUName: "默认规格", Stock: &stock}
	if err := db.Create(&sku).Error; err != nil {
		t.Fatal(err)
	}
	return sku
}

func inventoryBody(hash string) migrationimport.WizardBody {
	return migrationimport.WizardBody{
		Kind:    migrationimport.KindInventory,
		Columns: []string{"SKU编码", "仓库编码", "期初数量", "参考进价"},
		Rows: [][]string{
			{"INV-SKU-1", "", "12", "4.50"},
			{"INV-SKU-MISSING", "", "3", ""},
		},
		Mapping:      map[string]int{"skuCode": 0, "warehouseCode": 1, "quantity": 2, "costPrice": 3},
		FileName:     "inventory.csv",
		FileHash:     hash,
		SourceFormat: migrationimport.SourceCustom,
	}
}

func TestInventoryOpeningImport(t *testing.T) {
	db := openTestDB(t)
	migrateRound115(t, db)
	svc := newSvc115(db)
	c := testCtx(1)
	sku := seedSKU(t, db, 1, "INV-SKU-1")

	out, err := svc.Validate(c, inventoryBody("hash-inv-1"))
	if err != nil {
		t.Fatal(err)
	}
	if out.TotalRows != 2 || out.ErrorRows != 0 {
		t.Fatalf("validate: %+v", out)
	}

	res, err := svc.Commit(c, inventoryBody("hash-inv-1"), nil)
	if err != nil {
		t.Fatal(err)
	}
	if res.SuccessRows != 1 || res.FailedRows != 1 {
		t.Fatalf("commit: %+v", res)
	}
	var got product.ProductSKU
	if err := db.First(&got, "id = ?", sku.ID).Error; err != nil {
		t.Fatal(err)
	}
	if got.Stock == nil || *got.Stock != 12 {
		t.Fatalf("stock not applied: %+v", got.Stock)
	}
	if got.CostPrice == nil || *got.CostPrice != 4.5 {
		t.Fatalf("cost price not applied: %+v", got.CostPrice)
	}
	var logs int64
	if err := db.Model(&inventory.InventoryChangeLog{}).
		Where("product_sku_id = ? AND change_type = ?", sku.ID, inventory.ChangeImportOpening).
		Count(&logs).Error; err != nil {
		t.Fatal(err)
	}
	if logs != 1 {
		t.Fatalf("change logs: %d", logs)
	}

	// A different file carrying the same SKU is a duplicate, not a re-add.
	res2, err := svc.Commit(c, inventoryBody("hash-inv-2"), nil)
	if err != nil {
		t.Fatal(err)
	}
	if res2.DuplicateRows != 1 || res2.SuccessRows != 0 {
		t.Fatalf("second commit: %+v", res2)
	}
	if err := db.First(&got, "id = ?", sku.ID).Error; err != nil {
		t.Fatal(err)
	}
	if got.Stock == nil || *got.Stock != 12 {
		t.Fatalf("stock changed on duplicate: %+v", got.Stock)
	}
}

func TestInventoryOpeningImportNonDefaultWarehouse(t *testing.T) {
	db := openTestDB(t)
	migrateRound115(t, db)
	svc := newSvc115(db)
	c := testCtx(1)
	sku := seedSKU(t, db, 1, "INV-SKU-WH")
	def := inventory.Warehouse{TenantID: 1, Code: "WH-DEF", Name: "默认仓", IsDefault: true, Enabled: true}
	sub := inventory.Warehouse{TenantID: 1, Code: "WH-2", Name: "二号仓", Enabled: true}
	if err := db.Create(&def).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Create(&sub).Error; err != nil {
		t.Fatal(err)
	}

	body := migrationimport.WizardBody{
		Kind:         migrationimport.KindInventory,
		Columns:      []string{"SKU编码", "仓库编码", "期初数量"},
		Rows:         [][]string{{"INV-SKU-WH", "WH-2", "7"}, {"INV-SKU-WH", "WH-NOPE", "1"}},
		Mapping:      map[string]int{"skuCode": 0, "warehouseCode": 1, "quantity": 2},
		FileName:     "inventory-wh.csv",
		FileHash:     "hash-inv-wh",
		SourceFormat: migrationimport.SourceCustom,
	}
	res, err := svc.Commit(c, body, nil)
	if err != nil {
		t.Fatal(err)
	}
	if res.SuccessRows != 1 || res.FailedRows != 1 {
		t.Fatalf("commit: %+v", res)
	}
	var ws inventory.WarehouseStock
	if err := db.First(&ws, "warehouse_id = ? AND product_sku_id = ?", sub.ID, sku.ID).Error; err != nil {
		t.Fatal(err)
	}
	if ws.Stock != 7 {
		t.Fatalf("warehouse stock: %d", ws.Stock)
	}
	var got product.ProductSKU
	if err := db.First(&got, "id = ?", sku.ID).Error; err != nil {
		t.Fatal(err)
	}
	if got.Stock == nil || *got.Stock != 7 {
		t.Fatalf("total stock: %+v", got.Stock)
	}
}

func sourceBody(hash string) migrationimport.WizardBody {
	return migrationimport.WizardBody{
		Kind:    migrationimport.KindSource,
		Columns: []string{"供应商名称", "SKU编码", "货源链接", "参考价", "货源SKU"},
		Rows: [][]string{
			{"华强北电子", "SRC-SKU-1", "https://detail.1688.com/offer/123.html", "3.80", "EXT-1"},
			{"华强北电子", "SRC-SKU-NOPE", "", "1.00", ""},
		},
		Mapping:      map[string]int{"supplierName": 0, "skuCode": 1, "sourceLink": 2, "refPrice": 3, "externalSkuId": 4},
		FileName:     "sources.csv",
		FileHash:     hash,
		SourceFormat: migrationimport.SourceCustom,
	}
}

func TestSourceArchiveImport(t *testing.T) {
	db := openTestDB(t)
	migrateRound115(t, db)
	svc := newSvc115(db)
	c := testCtx(1)
	sku := seedSKU(t, db, 1, "SRC-SKU-1")

	res, err := svc.Commit(c, sourceBody("hash-src-1"), nil)
	if err != nil {
		t.Fatal(err)
	}
	if res.SuccessRows != 1 || res.FailedRows != 1 {
		t.Fatalf("commit: %+v", res)
	}
	var sup sourcing.Supplier
	if err := db.First(&sup, "tenant_id = ? AND name = ?", int64(1), "华强北电子").Error; err != nil {
		t.Fatal(err)
	}
	var src sourcing.ProductSource
	if err := db.First(&src, "supplier_id = ? AND product_id = ?", sup.ID, sku.ProductID).Error; err != nil {
		t.Fatal(err)
	}
	var m sourcing.ProductSourceSKU
	if err := db.First(&m, "product_source_id = ? AND local_sku_id = ?", src.ID, sku.ID).Error; err != nil {
		t.Fatal(err)
	}
	if m.CurrentPrice == nil || *m.CurrentPrice != 3.8 || m.ExternalSKUID != "EXT-1" {
		t.Fatalf("mapping: %+v", m)
	}

	// Re-import via a different file: existing mapping rows become duplicates.
	res2, err := svc.Commit(c, sourceBody("hash-src-2"), nil)
	if err != nil {
		t.Fatal(err)
	}
	if res2.DuplicateRows != 1 {
		t.Fatalf("second commit: %+v", res2)
	}
}

func TestMappingPresetsCRUD(t *testing.T) {
	db := openTestDB(t)
	migrateRound115(t, db)
	svc := newSvc115(db)
	c := testCtx(1)

	body := migrationimport.MappingPresetBody{
		Kind:    migrationimport.KindInventory,
		Name:    "店小秘库存模板",
		Columns: []string{"SKU", "仓库", "数量"},
		Mapping: map[string]int{"skuCode": 0, "warehouseCode": 1, "quantity": 2},
	}
	p1, err := svc.SaveMappingPreset(c, body, nil)
	if err != nil {
		t.Fatal(err)
	}
	// Same name overwrites instead of duplicating.
	body.Mapping = map[string]int{"skuCode": 1, "warehouseCode": 0, "quantity": 2}
	p2, err := svc.SaveMappingPreset(c, body, nil)
	if err != nil {
		t.Fatal(err)
	}
	if p1.ID != p2.ID {
		t.Fatalf("expected upsert, got new row")
	}
	list, err := svc.ListMappingPresets(c, migrationimport.KindInventory)
	if err != nil {
		t.Fatal(err)
	}
	if len(list) != 1 {
		t.Fatalf("list: %d", len(list))
	}
	// Tenant isolation.
	other, err := svc.ListMappingPresets(testCtx(2), migrationimport.KindInventory)
	if err != nil {
		t.Fatal(err)
	}
	if len(other) != 0 {
		t.Fatalf("tenant leak: %d", len(other))
	}
	if err := svc.DeleteMappingPreset(c, p1.ID); err != nil {
		t.Fatal(err)
	}
	if err := svc.DeleteMappingPreset(c, uuid.New()); err == nil {
		t.Fatalf("expected not found")
	}
}

func TestFieldsForNewKinds(t *testing.T) {
	inv := migrationimport.FieldsForKind(migrationimport.KindInventory)
	if len(inv) != 4 || inv[0].Key != "skuCode" || !inv[0].Required {
		t.Fatalf("inventory fields: %+v", inv)
	}
	src := migrationimport.FieldsForKind(migrationimport.KindSource)
	if len(src) != 5 || src[0].Key != "supplierName" || !src[0].Required {
		t.Fatalf("source fields: %+v", src)
	}
}
