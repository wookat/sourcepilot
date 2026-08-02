package demoseed

import (
	"context"
	"testing"

	"github.com/trademind-ai/trademind/backend/internal/modules/admin"
	"github.com/trademind-ai/trademind/backend/internal/modules/product"
)

// A seeded demo product renamed from the UI (title no longer DEMO-) must
// still be fully cleaned via its DEMO- SKUs, leaving zero residual rows.
func TestFullDemoCleanupRenamedProduct(t *testing.T) {
	db := openFullDemoTestDB(t)
	if db == nil {
		return
	}
	s := &FullDemoSeeder{DB: db, TenantID: 7, AppEnv: "development"}
	if _, err := s.Seed(context.Background()); err != nil {
		t.Fatalf("seed: %v", err)
	}

	var renamed product.Product
	if err := db.Where("title LIKE ?", "DEMO-%").First(&renamed).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Model(&product.Product{}).Where("id = ?", renamed.ID).
		Update("title", "改名后的普通商品").Error; err != nil {
		t.Fatal(err)
	}

	real := product.Product{TenantID: 7, Source: "manual", Title: "真实商品", Currency: "CNY", Status: product.StatusDraft}
	if err := db.Create(&real).Error; err != nil {
		t.Fatal(err)
	}
	realStock := 5
	realSKU := product.ProductSKU{ProductID: real.ID, SKUCode: "REAL-SKU-1", SKUName: "默认", Stock: &realStock}
	if err := db.Create(&realSKU).Error; err != nil {
		t.Fatal(err)
	}

	if _, err := s.Cleanup(context.Background()); err != nil {
		t.Fatalf("cleanup: %v", err)
	}

	res, err := s.VerifyClean(context.Background())
	if err != nil {
		t.Fatalf("verify: %v", err)
	}
	for table, n := range res.Counts {
		if n != 0 {
			t.Fatalf("residual demo rows in %s: %d", table, n)
		}
	}
	var n int64
	if err := db.Model(&product.Product{}).Where("id = ?", renamed.ID).Count(&n).Error; err != nil {
		t.Fatal(err)
	}
	if n != 0 {
		t.Fatalf("renamed demo product must be cleaned, got %d rows", n)
	}
	if err := db.Model(&product.ProductSKU{}).Where("sku_code LIKE ?", "DEMO-%").Count(&n).Error; err != nil {
		t.Fatal(err)
	}
	if n != 0 {
		t.Fatalf("DEMO- SKUs must be cleaned, got %d rows", n)
	}
	if err := db.Model(&product.Product{}).Where("id = ?", real.ID).Count(&n).Error; err != nil {
		t.Fatal(err)
	}
	if n != 1 {
		t.Fatal("real product must survive cleanup")
	}
}

// Seed grants an ungranted readonly user view scope on the DEMO shop so the
// permission boundary (read-visible / write-denied) is verifiable out of the
// box; cleanup removes it with the shop.
func TestFullDemoSeedReadonlyStoreGrant(t *testing.T) {
	db := openFullDemoTestDB(t)
	if db == nil {
		return
	}
	ro := admin.AdminUser{TenantID: 7, Username: admin.NewInternalUsername(),
		Email: "demo_readonly@trademind.local", PasswordHash: "x", Role: "readonly"}
	if err := db.Create(&ro).Error; err != nil {
		t.Fatal(err)
	}

	s := &FullDemoSeeder{DB: db, TenantID: 7, AppEnv: "development"}
	if _, err := s.Seed(context.Background()); err != nil {
		t.Fatalf("seed: %v", err)
	}

	var grants []admin.UserStorePermission
	if err := db.Where("user_id = ?", ro.ID).Find(&grants).Error; err != nil {
		t.Fatal(err)
	}
	if len(grants) != 1 {
		t.Fatalf("expected 1 demo grant for readonly user, got %d", len(grants))
	}
	if grants[0].PermissionScope != admin.StorePermScopeView {
		t.Fatalf("expected view scope for readonly, got %s", grants[0].PermissionScope)
	}

	if _, err := s.Cleanup(context.Background()); err != nil {
		t.Fatalf("cleanup: %v", err)
	}
	var n int64
	if err := db.Model(&admin.UserStorePermission{}).Where("user_id = ?", ro.ID).Count(&n).Error; err != nil {
		t.Fatal(err)
	}
	if n != 0 {
		t.Fatalf("readonly demo grant must be cleaned, got %d", n)
	}
}
