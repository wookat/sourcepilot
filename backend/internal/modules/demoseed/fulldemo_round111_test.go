package demoseed

import (
	"context"
	"testing"

	"github.com/trademind-ai/trademind/backend/internal/modules/waybill"
)

// Round 111: demo seed covers waybill templates + shipping rules, and
// cleanup/verify leave zero DEMO- residue while keeping tenant presets.
func TestFullDemoSeedWaybillTemplatesAndShippingRules(t *testing.T) {
	db := openFullDemoTestDB(t)
	s := &FullDemoSeeder{DB: db, TenantID: 1, AppEnv: "development"}

	res, err := s.Seed(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if res.Counts["waybill_templates"] != int64(len(waybill.TemplatePresets())) {
		t.Fatalf("expected %d waybill templates, got %d", len(waybill.TemplatePresets()), res.Counts["waybill_templates"])
	}
	if res.Counts["shipping_rules"] == 0 {
		t.Fatal("expected demo shipping rules to be seeded")
	}

	// Seed is idempotent: re-seed keeps the same counts (no duplicates).
	res2, err := s.Seed(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if res2.Counts["waybill_templates"] != res.Counts["waybill_templates"] || res2.Counts["shipping_rules"] != res.Counts["shipping_rules"] {
		t.Fatalf("expected idempotent counts, got %+v vs %+v", res2.Counts, res.Counts)
	}

	if _, err := s.Cleanup(context.Background()); err != nil {
		t.Fatal(err)
	}
	verify, err := s.VerifyClean(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if verify.Counts["shipping_rules"] != 0 {
		t.Fatalf("expected zero DEMO- shipping rules after cleanup, got %d", verify.Counts["shipping_rules"])
	}
	if verify.Counts["waybill_templates"] != 0 {
		t.Fatalf("expected zero DEMO- custom waybill templates after cleanup, got %d", verify.Counts["waybill_templates"])
	}

	// Presets remain as the tenant's baseline (same policy as carriers).
	var presets int64
	if err := db.Model(&waybill.Template{}).Where("tenant_id = 1 AND is_preset = ?", true).Count(&presets).Error; err != nil {
		t.Fatal(err)
	}
	if presets != int64(len(waybill.TemplatePresets())) {
		t.Fatalf("expected %d preset templates to survive cleanup, got %d", len(waybill.TemplatePresets()), presets)
	}
}
