package bannedwords_test

import (
	"context"
	"testing"

	"github.com/trademind-ai/trademind/backend/internal/modules/bannedwords"
)

func TestAIComplianceAdvisorAvoidWordsForbiddenOnly(t *testing.T) {
	db := openTestDB(t)
	svc := &bannedwords.Service{DB: db}
	adv := &bannedwords.AIComplianceAdvisor{Svc: svc}

	if _, err := svc.Create(testCtx(1), bannedwords.CreateBody{Word: "自定义禁词", Level: "forbidden"}, nil); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.Create(testCtx(1), bannedwords.CreateBody{Word: "自定义警告词", Level: "warning"}, nil); err != nil {
		t.Fatal(err)
	}

	words, err := adv.AvoidWords(context.Background(), 1)
	if err != nil {
		t.Fatal(err)
	}
	hasForbidden, hasWarning := false, false
	for _, w := range words {
		if w == "自定义禁词" {
			hasForbidden = true
		}
		if w == "自定义警告词" {
			hasWarning = true
		}
	}
	if !hasForbidden {
		t.Fatal("expected forbidden custom word in avoid list")
	}
	if hasWarning {
		t.Fatal("warning-level word must not be in avoid list")
	}

	// Tenant isolation: tenant 2 does not see tenant 1's custom word.
	words2, err := adv.AvoidWords(context.Background(), 2)
	if err != nil {
		t.Fatal(err)
	}
	for _, w := range words2 {
		if w == "自定义禁词" {
			t.Fatal("tenant 2 must not see tenant 1 custom word")
		}
	}
}

func TestAIComplianceAdvisorCheckText(t *testing.T) {
	db := openTestDB(t)
	svc := &bannedwords.Service{DB: db}
	adv := &bannedwords.AIComplianceAdvisor{Svc: svc}

	if _, err := svc.Create(testCtx(1), bannedwords.CreateBody{Word: "秒杀神器", Level: "forbidden", Suggestion: "改为中性描述"}, nil); err != nil {
		t.Fatal(err)
	}

	hits, err := adv.CheckText(context.Background(), 1, "全新秒杀神器限量发售")
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, h := range hits {
		if h.Word == "秒杀神器" {
			found = true
			if h.Level != bannedwords.LevelForbidden {
				t.Fatalf("expected forbidden level, got %s", h.Level)
			}
			if h.Suggestion != "改为中性描述" {
				t.Fatalf("expected suggestion carried over, got %q", h.Suggestion)
			}
		}
	}
	if !found {
		t.Fatal("expected hit for 秒杀神器")
	}

	clean, err := adv.CheckText(context.Background(), 1, "普通合规文案")
	if err != nil {
		t.Fatal(err)
	}
	if len(clean) != 0 {
		t.Fatalf("expected no hits for clean text, got %d", len(clean))
	}
}
