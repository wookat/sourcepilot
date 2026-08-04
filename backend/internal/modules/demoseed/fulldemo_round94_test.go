package demoseed

import (
	"context"
	"testing"

	"github.com/trademind-ai/trademind/backend/internal/modules/migrationimport"
	"github.com/trademind-ai/trademind/backend/internal/modules/order"
	"github.com/trademind-ai/trademind/backend/internal/modules/product"
	"github.com/trademind-ai/trademind/backend/internal/modules/shop"
	"gorm.io/datatypes"
)

// Round 94: clean/verify cover migration import artifacts (import_jobs /
// import_job_rows plus the imported drafts and orders identified by prefix)
// for the default DEMO- prefix and custom -prefix runs, without touching
// real import history.
func TestCleanupRemovesMigrationImportArtifacts(t *testing.T) {
	db := openFullDemoTestDB(t)
	if db == nil {
		return
	}
	ctx := context.Background()
	seeder := &FullDemoSeeder{DB: db, TenantID: 7, AppEnv: "development"}
	if _, err := seeder.Seed(ctx); err != nil {
		t.Fatalf("seed: %v", err)
	}

	var demoShop shop.Shop
	if err := db.Where("shop_code LIKE ?", DemoPrefix+"%").First(&demoShop).Error; err != nil {
		t.Fatalf("demo shop: %v", err)
	}
	realShop := shop.Shop{TenantID: 7, Platform: "manual", ShopName: "真实店铺", ShopCode: "REAL-SHOP-1",
		Status: "active", AuthStatus: "authorized", Currency: "CNY"}
	if err := db.Create(&realShop).Error; err != nil {
		t.Fatal(err)
	}

	mkJob := func(fileName, batchKey string, sh *shop.Shop) migrationimport.ImportJob {
		job := migrationimport.ImportJob{TenantID: 7, Kind: migrationimport.KindOrder,
			BatchKey: batchKey, SourceFormat: migrationimport.SourceMabang, FileName: fileName,
			Status: migrationimport.JobStatusPartialSuccess, TotalRows: 2, SuccessRows: 1, FailedRows: 1}
		if sh != nil {
			job.ShopID = &sh.ID
		}
		if err := db.Create(&job).Error; err != nil {
			t.Fatalf("create job %s: %v", fileName, err)
		}
		row := migrationimport.ImportJobRow{JobID: job.ID, RowNumber: 2,
			Status: migrationimport.RowStatusFailed, Field: "quantity", Message: "数量需为正整数",
			RawValues: datatypes.JSON([]byte(`{"数量":"x"}`))}
		if err := db.Create(&row).Error; err != nil {
			t.Fatalf("create job row %s: %v", fileName, err)
		}
		return job
	}

	// Demo-shop import (unprefixed sample file) + prefixed-file import must be
	// cleaned; the real-shop unprefixed import must survive.
	demoShopJob := mkJob("mabang-orders.csv", "hash-demo-shop", &demoShop)
	prefixedJob := mkJob("DEMO-products.csv", "hash-prefixed", &realShop)
	realJob := mkJob("real-orders.csv", "hash-real", &realShop)

	// Imported draft/order created by the demo import (prefix-identified).
	importedOrder := order.Order{TenantID: 7, Platform: "manual", ShopID: &demoShop.ID,
		OrderNo: "DEMO-IMP-0001", CustomerName: "Alice", Status: order.StatusPaid,
		PaymentStatus: order.PaymentPaid, FulfillmentStatus: order.FulfillmentUnfulfilled, Currency: "USD"}
	if err := db.Create(&importedOrder).Error; err != nil {
		t.Fatal(err)
	}
	importedProduct := product.Product{TenantID: 7, Title: "无线蓝牙耳机 X100", Source: "migration", Status: product.StatusDraft}
	if err := db.Create(&importedProduct).Error; err != nil {
		t.Fatal(err)
	}
	importedSKU := product.ProductSKU{ProductID: importedProduct.ID, SKUCode: "DEMO-IMP-SKU-1", SKUName: "黑色"}
	if err := db.Create(&importedSKU).Error; err != nil {
		t.Fatal(err)
	}

	for run := 0; run < 2; run++ { // twice: idempotency
		if _, err := seeder.Cleanup(ctx); err != nil {
			t.Fatalf("cleanup run %d: %v", run+1, err)
		}
	}
	verify, err := seeder.VerifyClean(ctx)
	if err != nil {
		t.Fatalf("verify: %v", err)
	}
	for table, cnt := range verify.Counts {
		if cnt != 0 {
			t.Errorf("residual DEMO- rows in %s: %d", table, cnt)
		}
	}
	for _, tbl := range []string{"import_jobs", "import_job_rows"} {
		if _, ok := verify.Counts[tbl]; !ok {
			t.Errorf("verify must report %s", tbl)
		}
	}

	var jobs []migrationimport.ImportJob
	if err := db.Unscoped().Find(&jobs).Error; err != nil {
		t.Fatal(err)
	}
	if len(jobs) != 1 || jobs[0].ID != realJob.ID {
		t.Errorf("only the real import job may survive, got %d jobs", len(jobs))
	}
	var rows []migrationimport.ImportJobRow
	if err := db.Find(&rows).Error; err != nil {
		t.Fatal(err)
	}
	for _, r := range rows {
		if r.JobID == demoShopJob.ID || r.JobID == prefixedJob.ID {
			t.Errorf("import job row of cleaned job %s must be deleted", r.JobID)
		}
	}
	if len(rows) != 1 {
		t.Errorf("only the real job's error row may survive, got %d", len(rows))
	}
	var n int64
	if err := db.Unscoped().Model(&order.Order{}).Where("id = ?", importedOrder.ID).Count(&n).Error; err != nil {
		t.Fatal(err)
	}
	if n != 0 {
		t.Error("imported DEMO- order must be deleted")
	}
	if err := db.Unscoped().Model(&product.Product{}).Where("id = ?", importedProduct.ID).Count(&n).Error; err != nil {
		t.Fatal(err)
	}
	if n != 0 {
		t.Error("imported draft owning DEMO- SKU must be deleted")
	}
}

// Custom -prefix runs (e.g. MB-) clean prefixed import artifacts and imported
// rows without touching DEMO- or real data.
func TestCleanupCustomPrefixCoversMigrationImports(t *testing.T) {
	db := openFullDemoTestDB(t)
	if db == nil {
		return
	}
	ctx := context.Background()
	demoSeeder := &FullDemoSeeder{DB: db, TenantID: 7, AppEnv: "development"}
	if _, err := demoSeeder.Seed(ctx); err != nil {
		t.Fatalf("seed: %v", err)
	}

	mbJob := migrationimport.ImportJob{TenantID: 7, Kind: migrationimport.KindOrder,
		BatchKey: "MB-batch-1", SourceFormat: migrationimport.SourceMabang, FileName: "MB-orders.csv",
		Status: migrationimport.JobStatusSuccess, TotalRows: 1, SuccessRows: 1}
	if err := db.Create(&mbJob).Error; err != nil {
		t.Fatal(err)
	}
	mbOrder := order.Order{TenantID: 7, Platform: "manual", OrderNo: "MB-2026-0001",
		CustomerName: "Bob", Status: order.StatusPaid, PaymentStatus: order.PaymentPaid,
		FulfillmentStatus: order.FulfillmentUnfulfilled, Currency: "USD"}
	if err := db.Create(&mbOrder).Error; err != nil {
		t.Fatal(err)
	}

	mbSeeder := &FullDemoSeeder{DB: db, TenantID: 7, AppEnv: "development", Prefix: "MB-"}
	if _, err := mbSeeder.Cleanup(ctx); err != nil {
		t.Fatalf("mb cleanup: %v", err)
	}
	verify, err := mbSeeder.VerifyClean(ctx)
	if err != nil {
		t.Fatalf("mb verify: %v", err)
	}
	for table, cnt := range verify.Counts {
		if cnt != 0 {
			t.Errorf("residual MB- rows in %s: %d", table, cnt)
		}
	}

	// DEMO- dataset (including demo shops) untouched by the MB- run.
	var demoShops int64
	if err := db.Model(&shop.Shop{}).Where("shop_code LIKE ?", DemoPrefix+"%").Count(&demoShops).Error; err != nil {
		t.Fatal(err)
	}
	if demoShops == 0 {
		t.Error("MB- cleanup must not delete DEMO- shops")
	}
}
