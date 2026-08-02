package demoseed

import (
	"context"
	"strings"
	"testing"

	"github.com/trademind-ai/trademind/backend/internal/modules/customerchat"
	"github.com/trademind-ai/trademind/backend/internal/modules/customersync"
	"github.com/trademind-ai/trademind/backend/internal/modules/selection"
)

func TestDemoSelectionTaskPlansUseModuleStatuses(t *testing.T) {
	valid := map[string]bool{
		selection.StatusPending: true, selection.StatusRunning: true,
		selection.StatusSuccess: true, selection.StatusFailed: true, selection.StatusPartial: true,
	}
	plans := demoSelectionTaskPlans()
	if len(plans) != 5 {
		t.Fatalf("expected 5 selection task plans, got %d", len(plans))
	}
	seen := map[string]bool{}
	for _, p := range plans {
		if !valid[p.status] {
			t.Errorf("plan %s: invalid status %q", p.suffix, p.status)
		}
		if !strings.HasPrefix(p.name, DemoPrefix) {
			t.Errorf("plan %s: name %q missing %s prefix", p.suffix, p.name, DemoPrefix)
		}
		if p.errorMessage != "" && !strings.HasPrefix(p.errorMessage, DemoPrefix) {
			t.Errorf("plan %s: error message missing %s prefix", p.suffix, DemoPrefix)
		}
		seen[p.status] = true
	}
	for st := range valid {
		if !seen[st] {
			t.Errorf("missing selection task status %s", st)
		}
	}
}

func TestDemoSelectionCandidatePlansCoverDecisions(t *testing.T) {
	seen := map[string]bool{}
	hasDraft := false
	for _, p := range demoSelectionCandidatePlans() {
		if !strings.HasPrefix(p.title, DemoPrefix) {
			t.Errorf("candidate %q missing %s prefix", p.title, DemoPrefix)
		}
		if p.status != selection.CandidateScored {
			t.Errorf("candidate %q: expected scored status, got %q", p.title, p.status)
		}
		seen[p.decision] = true
		if p.withDraft {
			hasDraft = true
			if p.decision != selection.DecisionApproved {
				t.Errorf("draft-converted candidate must be approved, got %q", p.decision)
			}
		}
	}
	for _, d := range []string{selection.DecisionPending, selection.DecisionApproved, selection.DecisionRejected} {
		if !seen[d] {
			t.Errorf("missing evaluation decision %s", d)
		}
	}
	if !hasDraft {
		t.Error("missing approved-and-converted-to-draft candidate sample")
	}
}

func TestDemoCustomerConversationPlansCoverStatuses(t *testing.T) {
	plans := demoCustomerConversationPlans()
	seenStatus := map[string]bool{}
	seenSuggestion := map[string]bool{}
	for _, p := range plans {
		if !strings.HasPrefix(p.customerName, DemoPrefix) {
			t.Errorf("conversation %q missing %s prefix", p.customerName, DemoPrefix)
		}
		if len(p.messages) == 0 {
			t.Errorf("conversation %q has no message timeline", p.customerName)
		}
		for _, m := range p.messages {
			if !strings.HasPrefix(m.content, DemoPrefix) {
				t.Errorf("conversation %q message missing %s prefix", p.customerName, DemoPrefix)
			}
		}
		seenStatus[p.status] = true
		if p.suggestion != nil {
			seenSuggestion[p.suggestion.status] = true
			if !strings.HasPrefix(p.suggestion.suggestedReply, DemoPrefix) {
				t.Errorf("conversation %q suggestion missing %s prefix", p.customerName, DemoPrefix)
			}
		}
	}
	for _, st := range []string{customerchat.StatusPendingReply, customerchat.StatusReplied, customerchat.StatusClosed} {
		if !seenStatus[st] {
			t.Errorf("missing conversation status %s", st)
		}
	}
	for _, st := range []string{customerchat.SuggestionGenerated, customerchat.SuggestionAccepted, customerchat.SuggestionRejected} {
		if !seenSuggestion[st] {
			t.Errorf("missing suggestion status %s", st)
		}
	}
}

// Round 68: seed must populate selection center + customer service samples on
// the seeder tenant (never tenant 0), be idempotent, and clean/verify must
// leave zero residual rows in the new tables.
func TestFullDemoSeedSelectionAndCustomerService(t *testing.T) {
	db := openFullDemoTestDB(t)
	if db == nil {
		return
	}
	s := &FullDemoSeeder{DB: db, TenantID: 7, AppEnv: "development"}
	ctx := context.Background()

	res, err := s.Seed(ctx)
	if err != nil {
		t.Fatalf("seed: %v", err)
	}
	for _, table := range []string{
		"selection_tasks", "selection_candidates", "selection_source_matches", "selection_evaluations",
		"customer_conversations", "customer_messages", "customer_reply_suggestions", "customer_message_sync_tasks",
	} {
		if res.Counts[table] == 0 {
			t.Errorf("seed produced no rows for %s", table)
		}
	}

	// tenant stamping: no seeded row may land on tenant 0
	var n int64
	if err := db.Model(&customerchat.CustomerConversation{}).
		Where("customer_name LIKE ? AND tenant_id <> ?", "DEMO-%", s.TenantID).Count(&n).Error; err != nil {
		t.Fatal(err)
	}
	if n != 0 {
		t.Fatalf("demo conversations on wrong tenant: %d", n)
	}
	if err := db.Model(&selection.SelectionTask{}).
		Where("name LIKE ? AND tenant_id <> ?", "DEMO-%", s.TenantID).Count(&n).Error; err != nil {
		t.Fatal(err)
	}
	if n != 0 {
		t.Fatalf("demo selection tasks on wrong tenant: %d", n)
	}
	if err := db.Model(&customersync.CustomerMessageSyncTask{}).
		Where("cursor LIKE ? AND tenant_id <> ?", "DEMO-%", s.TenantID).Count(&n).Error; err != nil {
		t.Fatal(err)
	}
	if n != 0 {
		t.Fatalf("demo sync tasks on wrong tenant: %d", n)
	}

	// sync tasks: one success + one failed
	if err := db.Model(&customersync.CustomerMessageSyncTask{}).
		Where("cursor LIKE ? AND status = ?", "DEMO-%", customersync.StatusSuccess).Count(&n).Error; err != nil {
		t.Fatal(err)
	}
	if n != 1 {
		t.Fatalf("expected 1 success sync task, got %d", n)
	}
	if err := db.Model(&customersync.CustomerMessageSyncTask{}).
		Where("cursor LIKE ? AND status = ?", "DEMO-%", customersync.StatusFailed).Count(&n).Error; err != nil {
		t.Fatal(err)
	}
	if n != 1 {
		t.Fatalf("expected 1 failed sync task, got %d", n)
	}

	// idempotency: reseeding yields identical counts
	res2, err := s.Seed(ctx)
	if err != nil {
		t.Fatalf("reseed: %v", err)
	}
	for table, c := range res.Counts {
		if res2.Counts[table] != c {
			t.Errorf("reseed count mismatch for %s: %d != %d", table, res2.Counts[table], c)
		}
	}

	if _, err := s.Cleanup(ctx); err != nil {
		t.Fatalf("cleanup: %v", err)
	}
	vres, err := s.VerifyClean(ctx)
	if err != nil {
		t.Fatalf("verify: %v", err)
	}
	for _, table := range []string{
		"selection_tasks", "selection_candidates", "selection_source_matches", "selection_evaluations",
		"customer_conversations", "customer_messages", "customer_reply_suggestions", "customer_message_sync_tasks",
	} {
		if _, ok := vres.Counts[table]; !ok {
			t.Errorf("verify does not cover %s", table)
		}
	}
	for table, c := range vres.Counts {
		if c != 0 {
			t.Errorf("residual demo rows in %s: %d", table, c)
		}
	}
	var evals int64
	if err := db.Model(&selection.SelectionEvaluation{}).Count(&evals).Error; err != nil {
		t.Fatal(err)
	}
	if evals != 0 {
		t.Fatalf("selection evaluations must be fully cleaned, got %d", evals)
	}
}
