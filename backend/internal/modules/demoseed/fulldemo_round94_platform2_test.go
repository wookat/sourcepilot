package demoseed

import (
	"context"
	"testing"

	"github.com/trademind-ai/trademind/backend/internal/modules/settings"
	"github.com/trademind-ai/trademind/backend/internal/modules/shop"
)

// Round 94: seed provides the second-platform (Shopee) publish demo samples —
// a Shopee DEMO shop plus the degraded (local_draft_only) platform_shopee
// config preset. Seed stays idempotent, never overwrites operator-managed
// settings, and clean/verify leaves zero DEMO- residual rows.
func TestFullDemoSeedShopeePublishSamples(t *testing.T) {
	db := openFullDemoTestDB(t)
	if db == nil {
		return
	}
	seeder := &FullDemoSeeder{DB: db, TenantID: 9, AppEnv: "development"}
	ctx := context.Background()

	for run := 0; run < 2; run++ { // twice: idempotency
		if _, err := seeder.Seed(ctx); err != nil {
			t.Fatalf("seed run %d: %v", run+1, err)
		}
	}

	// Shopee DEMO shop present and authorized.
	var spShop shop.Shop
	if err := db.Where("shop_code = ?", "DEMO-SHOP-4").First(&spShop).Error; err != nil {
		t.Fatalf("shopee demo shop: %v", err)
	}
	if spShop.Platform != "shopee" || spShop.AuthStatus != "authorized" || spShop.Status != "active" {
		t.Errorf("shopee demo shop misconfigured: %+v", spShop)
	}

	// platform_shopee preset present exactly once with DEMO remark; values
	// carry no real credential.
	var presetRows []settings.Setting
	if err := db.Where("tenant_id = 0 AND group_key = ?", "platform_shopee").Find(&presetRows).Error; err != nil {
		t.Fatal(err)
	}
	if len(presetRows) != len(demoPublishPresetSettings("platform_shopee")) {
		t.Fatalf("expected %d preset rows, got %d", len(demoPublishPresetSettings("platform_shopee")), len(presetRows))
	}
	for _, row := range presetRows {
		if row.Remark != demoSettingRemark {
			t.Errorf("preset row %s missing demo remark: %q", row.ItemKey, row.Remark)
		}
	}

	// Seed must not overwrite existing operator-managed configuration.
	if err := db.Where("group_key = ?", "platform_shopee").Delete(&settings.Setting{}).Error; err != nil {
		t.Fatal(err)
	}
	real := settings.Setting{TenantID: 0, GroupKey: "platform_shopee", ItemKey: "partner_id",
		ItemValue: "2000001", ValueType: "string"}
	if err := db.Create(&real).Error; err != nil {
		t.Fatal(err)
	}
	if _, err := seeder.Seed(ctx); err != nil {
		t.Fatalf("re-seed with real config: %v", err)
	}
	var n int64
	if err := db.Model(&settings.Setting{}).Where("group_key = ?", "platform_shopee").Count(&n).Error; err != nil {
		t.Fatal(err)
	}
	if n != 1 {
		t.Errorf("seed must skip preset when group already configured, got %d rows", n)
	}

	// Clean leaves zero DEMO residuals (the operator-managed row stays).
	if _, err := seeder.Cleanup(ctx); err != nil {
		t.Fatalf("cleanup: %v", err)
	}
	verify, err := seeder.VerifyClean(ctx)
	if err != nil {
		t.Fatalf("verify: %v", err)
	}
	for table, cnt := range verify.Counts {
		if cnt != 0 {
			t.Errorf("residual demo rows in %s: %d", table, cnt)
		}
	}
	if err := db.Model(&settings.Setting{}).Where("group_key = ?", "platform_shopee").Count(&n).Error; err != nil {
		t.Fatal(err)
	}
	if n != 1 {
		t.Errorf("cleanup must keep operator-managed settings, got %d rows", n)
	}
}
