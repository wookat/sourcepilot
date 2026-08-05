package demoseed

import (
	"context"
	"testing"

	"github.com/trademind-ai/trademind/backend/internal/modules/order"
	"github.com/trademind-ai/trademind/backend/internal/modules/product"
	"github.com/trademind-ai/trademind/backend/internal/modules/shop"
)

// Round 115 (UX v6 P2-2): soft-deleted prefixed rows are historical residue
// invisible to the app; VerifyClean must not count them as live residual rows
// (they are reported separately in SoftDeleted), while live residue still fails.
func TestVerifyCleanIgnoresSoftDeletedResidue(t *testing.T) {
	db := openFullDemoTestDB(t)
	s := &FullDemoSeeder{DB: db, TenantID: 1, AppEnv: "development"}
	ctx := context.Background()

	sh := shop.Shop{ShopCode: DemoPrefix + "SHOP-SD", ShopName: DemoPrefix + "店铺"}
	if err := db.Create(&sh).Error; err != nil {
		t.Fatal(err)
	}
	p := product.Product{TenantID: 1, Title: DemoPrefix + "商品SD", Status: "draft"}
	if err := db.Create(&p).Error; err != nil {
		t.Fatal(err)
	}
	o := order.Order{Platform: "manual", OrderNo: DemoPrefix + "SO-SD", Status: "paid"}
	if err := db.Create(&o).Error; err != nil {
		t.Fatal(err)
	}
	// Soft delete all three (deleted_at set, rows remain).
	if err := db.Delete(&sh).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Delete(&p).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Delete(&o).Error; err != nil {
		t.Fatal(err)
	}

	res, err := s.VerifyClean(ctx)
	if err != nil {
		t.Fatal(err)
	}
	for table, n := range res.Counts {
		if n != 0 {
			t.Fatalf("soft-deleted residue counted as live in %s: %d", table, n)
		}
	}
	for _, table := range []string{"shops", "products", "orders"} {
		if res.SoftDeleted[table] != 1 {
			t.Fatalf("soft-deleted %s not reported: %+v", table, res.SoftDeleted)
		}
	}

	// A live prefixed row must still be counted as residue.
	live := shop.Shop{ShopCode: DemoPrefix + "SHOP-LIVE", ShopName: DemoPrefix + "在营店铺"}
	if err := db.Create(&live).Error; err != nil {
		t.Fatal(err)
	}
	res, err = s.VerifyClean(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if res.Counts["shops"] != 1 {
		t.Fatalf("live residue missed: %+v", res.Counts)
	}
}
