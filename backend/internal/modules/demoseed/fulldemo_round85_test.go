package demoseed

import (
	"context"
	"testing"

	"github.com/trademind-ai/trademind/backend/internal/modules/product"
	"github.com/trademind-ai/trademind/backend/internal/modules/shop"
)

// Round 85: clean/verify accept a custom prefix (e.g. QA-) while defaulting to
// DEMO-; non-target prefixes stay untouched, runs are idempotent, and seed
// plus production refusal keep the DEMO--only / refuse-production口径.
func TestValidateCleanPrefix(t *testing.T) {
	for _, ok := range []string{"DEMO-", "QA-", "qa-", "R85-TEST-", "A1-"} {
		if err := ValidateCleanPrefix(ok); err != nil {
			t.Errorf("prefix %q should be valid: %v", ok, err)
		}
	}
	for _, bad := range []string{"", "-", "QA", "QA%", "QA_-", "%QA-", " QA-", "测试-", "0123456789012345678901234567890123-"} {
		if err := ValidateCleanPrefix(bad); err == nil {
			t.Errorf("prefix %q should be rejected", bad)
		}
	}
}

func TestCleanupCustomPrefixTargetsOnlyThatPrefix(t *testing.T) {
	db := openFullDemoTestDB(t)
	if db == nil {
		return
	}
	ctx := context.Background()
	demoSeeder := &FullDemoSeeder{DB: db, TenantID: 7, AppEnv: "development"}
	if _, err := demoSeeder.Seed(ctx); err != nil {
		t.Fatalf("seed: %v", err)
	}

	qaRows := []any{
		&shop.Shop{TenantID: 7, Platform: "manual", ShopName: "QA-店铺", ShopCode: "QA-SHOP-1",
			Status: "active", AuthStatus: "authorized", Currency: "CNY"},
		&product.Product{TenantID: 7, Title: "QA-商品草稿", Status: product.StatusReady, Source: "manual"},
		&product.Product{TenantID: 7, Title: "常规商品（非目标前缀）", Status: product.StatusReady, Source: "manual"},
	}
	for _, row := range qaRows {
		if err := db.Create(row).Error; err != nil {
			t.Fatalf("create qa fixture: %v", err)
		}
	}

	qaSeeder := &FullDemoSeeder{DB: db, TenantID: 7, AppEnv: "development", Prefix: "QA-"}
	for run := 0; run < 2; run++ { // twice: idempotency
		if _, err := qaSeeder.Cleanup(ctx); err != nil {
			t.Fatalf("qa cleanup run %d: %v", run+1, err)
		}
	}

	verify, err := qaSeeder.VerifyClean(ctx)
	if err != nil {
		t.Fatalf("qa verify: %v", err)
	}
	for table, cnt := range verify.Counts {
		if cnt != 0 {
			t.Errorf("residual QA- rows in %s: %d", table, cnt)
		}
	}

	// Non-target data survives: DEMO- rows and the plain-title product remain.
	var demoShops, demoProducts, plainProducts int64
	if err := db.Model(&shop.Shop{}).Where("shop_code LIKE ?", DemoPrefix+"%").Count(&demoShops).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Model(&product.Product{}).Where("title LIKE ?", DemoPrefix+"%").Count(&demoProducts).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Model(&product.Product{}).Where("title = ?", "常规商品（非目标前缀）").Count(&plainProducts).Error; err != nil {
		t.Fatal(err)
	}
	if demoShops == 0 || demoProducts == 0 {
		t.Errorf("QA- cleanup must not delete DEMO- rows (shops=%d products=%d)", demoShops, demoProducts)
	}
	if plainProducts != 1 {
		t.Errorf("QA- cleanup must not delete non-target rows, got %d", plainProducts)
	}

	// Default-prefix cleanup still removes DEMO- rows afterwards.
	if _, err := demoSeeder.Cleanup(ctx); err != nil {
		t.Fatalf("demo cleanup: %v", err)
	}
	demoVerify, err := demoSeeder.VerifyClean(ctx)
	if err != nil {
		t.Fatalf("demo verify: %v", err)
	}
	for table, cnt := range demoVerify.Counts {
		if cnt != 0 {
			t.Errorf("residual DEMO- rows in %s: %d", table, cnt)
		}
	}
}

func TestCustomPrefixGuards(t *testing.T) {
	db := openFullDemoTestDB(t)
	if db == nil {
		return
	}
	ctx := context.Background()

	// Seed only supports DEMO-.
	if _, err := (&FullDemoSeeder{DB: db, TenantID: 1, AppEnv: "development", Prefix: "QA-"}).Seed(ctx); err == nil {
		t.Error("seed with custom prefix must fail")
	}
	// Invalid prefixes (LIKE wildcards) are rejected before touching data.
	for _, bad := range []string{"QA%", "QA_-"} {
		if _, err := (&FullDemoSeeder{DB: db, TenantID: 1, AppEnv: "development", Prefix: bad}).Cleanup(ctx); err == nil {
			t.Errorf("cleanup with prefix %q must fail", bad)
		}
		if _, err := (&FullDemoSeeder{DB: db, TenantID: 1, AppEnv: "development", Prefix: bad}).VerifyClean(ctx); err == nil {
			t.Errorf("verify with prefix %q must fail", bad)
		}
	}
	// Production refusal口径 unchanged, custom prefix included.
	prod := &FullDemoSeeder{DB: db, TenantID: 1, AppEnv: "production", Prefix: "QA-"}
	if _, err := prod.Cleanup(ctx); err == nil {
		t.Error("cleanup must refuse production")
	}
	if _, err := prod.VerifyClean(ctx); err == nil {
		t.Error("verify must refuse production")
	}
}
