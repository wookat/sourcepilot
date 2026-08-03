package demoseed

import (
	"context"
	"testing"

	"github.com/trademind-ai/trademind/backend/internal/modules/operationtask"
	"github.com/trademind-ai/trademind/backend/internal/modules/productpublish"
	"github.com/trademind-ai/trademind/backend/internal/modules/settings"
	"github.com/trademind-ai/trademind/backend/internal/modules/shop"
)

// Round 75: seed provides the publish-link demo samples — a TikTok DEMO shop
// with the degraded (local_draft_only) platform_tiktok config preset, at
// least two pending-review operation tasks for batch approve/reject, and one
// bound douyin publication with SKU binding rows. Seed stays idempotent,
// never overwrites operator-managed settings, and clean/verify leaves zero
// DEMO- residual rows.
func TestFullDemoSeedPublishSamples(t *testing.T) {
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

	// TikTok DEMO shop present and authorized.
	var tkShop shop.Shop
	if err := db.Where("shop_code = ?", "DEMO-SHOP-3").First(&tkShop).Error; err != nil {
		t.Fatalf("tiktok demo shop: %v", err)
	}
	if tkShop.Platform != "tiktok" || tkShop.AuthStatus != "authorized" || tkShop.Status != "active" {
		t.Errorf("tiktok demo shop misconfigured: %+v", tkShop)
	}

	// platform_tiktok preset present exactly once with DEMO remark; values
	// carry no real credential.
	var presetRows []settings.Setting
	if err := db.Where("tenant_id = 0 AND group_key = ?", "platform_tiktok").Find(&presetRows).Error; err != nil {
		t.Fatal(err)
	}
	if len(presetRows) != len(demoTikTokPublishPreset()) {
		t.Fatalf("expected %d preset rows, got %d", len(demoTikTokPublishPreset()), len(presetRows))
	}
	for _, row := range presetRows {
		if row.Remark != demoSettingRemark {
			t.Errorf("preset row %s missing demo remark: %q", row.ItemKey, row.Remark)
		}
	}

	// Seed must not overwrite existing operator-managed configuration.
	if err := db.Where("group_key = ?", "platform_tiktok").Delete(&settings.Setting{}).Error; err != nil {
		t.Fatal(err)
	}
	real := settings.Setting{TenantID: 0, GroupKey: "platform_tiktok", ItemKey: "app_key",
		ItemValue: "real-operator-key", ValueType: "string"}
	if err := db.Create(&real).Error; err != nil {
		t.Fatal(err)
	}
	if _, err := seeder.Seed(ctx); err != nil {
		t.Fatalf("re-seed with real config: %v", err)
	}
	var n int64
	if err := db.Model(&settings.Setting{}).Where("group_key = ?", "platform_tiktok").Count(&n).Error; err != nil {
		t.Fatal(err)
	}
	if n != 1 {
		t.Errorf("seed must skip preset when group already configured, got %d rows", n)
	}

	// ≥2 pending-review operation tasks for the batch approve/reject flow.
	var pending int64
	if err := db.Model(&operationtask.OperationTask{}).
		Where("status = ? AND title LIKE ?", operationtask.OperationTaskStatusPendingReview, DemoPrefix+"%").
		Count(&pending).Error; err != nil {
		t.Fatal(err)
	}
	if pending < 2 {
		t.Errorf("expected >=2 pending-review demo tasks, got %d", pending)
	}

	// One bound douyin publication with SKU binding rows (bound + unmatched).
	var pub productpublish.ProductPublication
	if err := db.Where("external_product_id = ?", "DEMO-DY-3502001").First(&pub).Error; err != nil {
		t.Fatalf("douyin publication sample: %v", err)
	}
	if pub.Platform != "douyin_shop" || pub.Status != productpublish.StatusPublishedRecord {
		t.Errorf("douyin publication misconfigured: platform=%s status=%s", pub.Platform, pub.Status)
	}
	var bindRows []productpublish.ProductPublicationSKU
	if err := db.Where("publication_id = ?", pub.ID).Find(&bindRows).Error; err != nil {
		t.Fatal(err)
	}
	if len(bindRows) != 2 {
		t.Fatalf("expected 2 publication sku rows, got %d", len(bindRows))
	}
	statuses := map[string]bool{}
	for _, r := range bindRows {
		statuses[r.BindStatus] = true
	}
	if !statuses[productpublish.BindStatusBound] || !statuses[productpublish.BindStatusUnmatched] {
		t.Errorf("expected bound + unmatched sku binding samples, got %v", statuses)
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
	if err := db.Model(&settings.Setting{}).Where("group_key = ?", "platform_tiktok").Count(&n).Error; err != nil {
		t.Fatal(err)
	}
	if n != 1 {
		t.Errorf("cleanup must keep operator-managed settings, got %d rows", n)
	}
}
