package demoseed

import (
	"context"
	"testing"

	"github.com/trademind-ai/trademind/backend/internal/modules/collect"
	"github.com/trademind-ai/trademind/backend/internal/modules/selection"
	"github.com/trademind-ai/trademind/backend/internal/modules/sourcing"
)

// Round 120: repeated collect captures per demo candidate source URL (price
// trend samples) plus same-category product link; cleanup / verify leave zero
// residue.
func TestFullDemoSeedSourcingInsights(t *testing.T) {
	db := openFullDemoTestDB(t)
	s := &FullDemoSeeder{DB: db, TenantID: 1, AppEnv: "development"}

	res, err := s.Seed(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if res.Counts["collect_tasks"] != 6 {
		t.Fatalf("expected 6 demo collect captures, got %d", res.Counts["collect_tasks"])
	}

	var tasks []collect.CollectTask
	if err := db.Where("source_url LIKE ?", demoMarketURLPrefix+"%").Find(&tasks).Error; err != nil {
		t.Fatal(err)
	}
	if len(tasks) != 6 {
		t.Fatalf("expected 6 captures in db, got %d", len(tasks))
	}
	perURL := map[string]int{}
	for _, task := range tasks {
		if task.TenantID != 1 {
			t.Fatalf("capture %s wrong tenant %d", task.ID, task.TenantID)
		}
		if task.Status != collect.StatusSuccess || len(task.RawResult) == 0 || task.FinishedAt == nil {
			t.Fatalf("capture %s must be a finished success with raw result", task.ID)
		}
		perURL[task.SourceURL]++
	}
	if perURL[demoMarketURLPrefix+"item/DEMO-SEL-1"] != 4 || perURL[demoMarketURLPrefix+"item/DEMO-SEL-3"] != 2 {
		t.Fatalf("capture distribution wrong: %v", perURL)
	}
	if perURL[demoMarketURLPrefix+"item/DEMO-SEL-2"] != 0 {
		t.Fatalf("candidate 2 must stay without history (trend empty state): %v", perURL)
	}

	var linked int64
	if err := db.Model(&selection.SelectionCandidate{}).
		Where("title LIKE ? AND product_id IS NOT NULL", "DEMO-候选%").Count(&linked).Error; err != nil {
		t.Fatal(err)
	}
	if linked < 1 {
		t.Fatalf("expected at least one demo candidate linked to a product, got %d", linked)
	}

	var readySources int64
	if err := db.Model(&sourcing.ProductSource{}).
		Where("source_offer_id = ?", "DEMO-SEL-OFFER-1").Count(&readySources).Error; err != nil {
		t.Fatal(err)
	}
	if readySources != 1 {
		t.Fatalf("expected one supply-ready demo source matching DEMO-SEL-OFFER-1, got %d", readySources)
	}

	if _, err := s.Cleanup(context.Background()); err != nil {
		t.Fatal(err)
	}
	verify, err := s.VerifyClean(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if verify.Counts["collect_tasks"] != 0 {
		t.Fatalf("expected zero demo collect captures after cleanup, got %d", verify.Counts["collect_tasks"])
	}
	if err := db.Model(&sourcing.ProductSource{}).
		Where("source_offer_id = ?", "DEMO-SEL-OFFER-1").Count(&readySources).Error; err != nil {
		t.Fatal(err)
	}
	if readySources != 0 {
		t.Fatalf("expected supply-ready demo source removed after cleanup, got %d", readySources)
	}
}
