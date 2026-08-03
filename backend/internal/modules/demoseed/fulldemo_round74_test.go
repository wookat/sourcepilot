package demoseed

import (
	"context"
	"testing"

	"github.com/trademind-ai/trademind/backend/internal/modules/productpublish"
	"github.com/trademind-ai/trademind/backend/internal/modules/shop"
	"github.com/trademind-ai/trademind/backend/internal/modules/sourcing"
)

// Round 74: seed links every demo product to the granted manual DEMO shop so
// operator/readonly demo accounts see non-empty lists, and provides at least
// one price-increase and one out-of-stock source alert with an open,
// adoptable switch suggestion each. Seed stays idempotent and clean leaves
// zero DEMO- residual rows.
func TestFullDemoSeedShopLinksAndSourcingAlerts(t *testing.T) {
	db := openFullDemoTestDB(t)
	if db == nil {
		return
	}
	seeder := &FullDemoSeeder{DB: db, TenantID: 7, AppEnv: "development"}
	ctx := context.Background()

	for run := 0; run < 2; run++ { // twice: idempotency
		if _, err := seeder.Seed(ctx); err != nil {
			t.Fatalf("seed run %d: %v", run+1, err)
		}
	}

	var grantedShop shop.Shop
	if err := db.Where("shop_code = ?", "DEMO-SHOP-2").First(&grantedShop).Error; err != nil {
		t.Fatalf("granted demo shop: %v", err)
	}

	var pubCount int64
	if err := db.Model(&productpublish.ProductPublication{}).
		Where("shop_id = ? AND title LIKE ?", grantedShop.ID, DemoPrefix+"%").
		Count(&pubCount).Error; err != nil {
		t.Fatal(err)
	}
	var prodCount int64
	if err := db.Table("products").Where("title LIKE ? AND deleted_at IS NULL", DemoPrefix+"%").
		Count(&prodCount).Error; err != nil {
		t.Fatal(err)
	}
	if pubCount == 0 || pubCount != prodCount {
		t.Fatalf("expected every demo product linked to granted shop, got %d publications for %d products", pubCount, prodCount)
	}

	for _, status := range []string{sourcing.SourceStatusPriceAlert, sourcing.SourceStatusOutOfStock} {
		var n int64
		if err := db.Model(&sourcing.ProductSource{}).
			Where("status = ? AND last_checked_at IS NOT NULL", status).
			Count(&n).Error; err != nil {
			t.Fatal(err)
		}
		if n < 1 {
			t.Errorf("expected at least one %s source with last_checked_at, got %d", status, n)
		}
	}

	var suggestions []sourcing.SourceSwitchEvent
	if err := db.Where("mode = ? AND status = ?", sourcing.SwitchModeSuggested, sourcing.SuggestionOpen).
		Find(&suggestions).Error; err != nil {
		t.Fatal(err)
	}
	if len(suggestions) < 2 {
		t.Fatalf("expected at least 2 open switch suggestions, got %d", len(suggestions))
	}
	reasons := map[string]bool{}
	for _, ev := range suggestions {
		reasons[ev.Reason] = true
		var backup sourcing.ProductSource
		if err := db.Where("id = ?", ev.ToSourceID).First(&backup).Error; err != nil {
			t.Errorf("suggestion %s: backup source missing: %v", ev.ID, err)
			continue
		}
		if backup.IsPrimary || backup.Status != sourcing.SourceStatusActive {
			t.Errorf("suggestion %s: backup source must be a non-primary active source", ev.ID)
		}
	}
	if !reasons[sourcing.SwitchReasonPriceIncrease] || !reasons[sourcing.SwitchReasonOutOfStock] {
		t.Errorf("expected open suggestions for both price_increase and out_of_stock, got %v", reasons)
	}

	if _, err := seeder.Cleanup(ctx); err != nil {
		t.Fatalf("cleanup: %v", err)
	}
	verify, err := seeder.VerifyClean(ctx)
	if err != nil {
		t.Fatalf("verify: %v", err)
	}
	for table, n := range verify.Counts {
		if n != 0 {
			t.Errorf("residual %s rows after cleanup: %d", table, n)
		}
	}
	var leftPubs, leftEvents int64
	if err := db.Model(&productpublish.ProductPublication{}).Unscoped().Count(&leftPubs).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Model(&sourcing.SourceSwitchEvent{}).Unscoped().Count(&leftEvents).Error; err != nil {
		t.Fatal(err)
	}
	if leftPubs != 0 || leftEvents != 0 {
		t.Errorf("cleanup left %d publications / %d switch events", leftPubs, leftEvents)
	}
}
