package demoseed

import (
	"context"
	"testing"

	"github.com/trademind-ai/trademind/backend/internal/modules/order"
)

// Round 114: demo审单规则 + 命中样本 seed / cleanup / verify leave zero residue.
func TestFullDemoSeedOrderReviewSamples(t *testing.T) {
	db := openFullDemoTestDB(t)
	s := &FullDemoSeeder{DB: db, TenantID: 1, AppEnv: "development"}

	if _, err := s.Seed(context.Background()); err != nil {
		t.Fatal(err)
	}

	var rules []order.OrderReviewRule
	if err := db.Find(&rules).Error; err != nil {
		t.Fatal(err)
	}
	if len(rules) != 4 {
		t.Fatalf("expected 4 demo review rules, got %d", len(rules))
	}
	var disabled int
	for _, r := range rules {
		if r.TenantID != 1 {
			t.Fatalf("rule %s wrong tenant %d", r.Name, r.TenantID)
		}
		if !r.Enabled {
			disabled++
		}
	}
	if disabled != 1 {
		t.Fatalf("expected exactly one disabled demo rule, got %d", disabled)
	}

	var held, pending int64
	db.Model(&order.Order{}).Where("review_status = ?", order.ReviewStatusHeld).Count(&held)
	db.Model(&order.Order{}).Where("review_status = ?", order.ReviewStatusPending).Count(&pending)
	if held < 1 || pending < 2 {
		t.Fatalf("expected held>=1 pending>=2 samples, got held=%d pending=%d", held, pending)
	}
	var hits int64
	db.Model(&order.OrderReviewHit{}).Count(&hits)
	if hits != 3 {
		t.Fatalf("expected 3 demo review hits, got %d", hits)
	}

	if _, err := s.Cleanup(context.Background()); err != nil {
		t.Fatal(err)
	}
	verify, err := s.VerifyClean(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if verify.Counts["order_review_rules"] != 0 {
		t.Fatalf("expected zero demo review rules after cleanup, got %d", verify.Counts["order_review_rules"])
	}
	if verify.Counts["order_review_hits"] != 0 {
		t.Fatalf("expected zero demo review hits after cleanup, got %d", verify.Counts["order_review_hits"])
	}
	var residue int64
	db.Model(&order.OrderReviewRule{}).Count(&residue)
	if residue != 0 {
		t.Fatalf("review rules residue: %d", residue)
	}
	db.Model(&order.OrderReviewHit{}).Count(&residue)
	if residue != 0 {
		t.Fatalf("review hits residue: %d", residue)
	}
}
