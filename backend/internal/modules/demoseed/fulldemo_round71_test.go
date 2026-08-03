package demoseed

import (
	"context"
	"strings"
	"testing"

	"github.com/trademind-ai/trademind/backend/internal/modules/operationtask"
)

func TestDemoOperationTaskPlansFollowStateMachine(t *testing.T) {
	plans := demoOperationTaskPlans()
	if len(plans) != 6 {
		t.Fatalf("expected 6 operation task plans, got %d", len(plans))
	}
	seenStatus := map[string]bool{}
	seenRetryable := map[bool]bool{}
	for _, p := range plans {
		if err := validateOperationTaskChain(p); err != nil {
			t.Errorf("plan %s: %v", p.suffix, err)
		}
		if !strings.HasPrefix(p.title, DemoPrefix) {
			t.Errorf("plan %s: title %q missing %s prefix", p.suffix, p.title, DemoPrefix)
		}
		if p.errMessage != "" && !strings.HasPrefix(p.errMessage, DemoPrefix) {
			t.Errorf("plan %s: error message missing %s prefix", p.suffix, DemoPrefix)
		}
		status := operationtask.OperationTaskStatusSuggested
		if len(p.statusChain) > 0 {
			status = p.statusChain[len(p.statusChain)-1]
		}
		seenStatus[status] = true
		if p.withAttempt && p.attemptStatus == operationtask.ExecutionAttemptStatusFailed {
			seenRetryable[p.errRetryable] = true
		}
	}
	for _, st := range []string{
		operationtask.OperationTaskStatusSuggested,
		operationtask.OperationTaskStatusPendingReview,
		operationtask.OperationTaskStatusRejected,
		operationtask.OperationTaskStatusDraftWritten,
		operationtask.OperationTaskStatusExecutionFailed,
	} {
		if !seenStatus[st] {
			t.Errorf("missing operation task status %s", st)
		}
	}
	if !seenRetryable[true] || !seenRetryable[false] {
		t.Error("failed task samples must cover retryable=true and retryable=false")
	}
}

// Round 71: seed must populate the operation task center (tasks + drafts +
// approvals + attempts + errors + events) on the seeder tenant, be
// idempotent, and clean/verify must leave zero DEMO- residual rows even with
// the immutable-record triggers installed.
func TestFullDemoSeedOperationTasks(t *testing.T) {
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
	tables := []string{
		"operation_tasks", "platform_drafts", "approval_records",
		"execution_attempts", "execution_errors", "operation_task_events",
	}
	for _, table := range tables {
		if res.Counts[table] == 0 {
			t.Errorf("seed produced no rows for %s", table)
		}
	}

	// tenant stamping: no seeded task lands on another tenant
	var n int64
	if err := db.Model(&operationtask.OperationTask{}).
		Where("title LIKE ? AND tenant_id <> ?", "DEMO-%", s.TenantID).Count(&n).Error; err != nil {
		t.Fatal(err)
	}
	if n != 0 {
		t.Fatalf("demo operation tasks on wrong tenant: %d", n)
	}

	// failed samples: one retryable and one non-retryable execution error
	if err := db.Model(&operationtask.ExecutionError{}).
		Where("safe_message LIKE ? AND retryable = ?", "DEMO-%", true).Count(&n).Error; err != nil {
		t.Fatal(err)
	}
	if n != 1 {
		t.Fatalf("expected 1 retryable failed sample, got %d", n)
	}
	if err := db.Model(&operationtask.ExecutionError{}).
		Where("safe_message LIKE ? AND retryable = ?", "DEMO-%", false).Count(&n).Error; err != nil {
		t.Fatal(err)
	}
	if n != 1 {
		t.Fatalf("expected 1 non-retryable failed sample, got %d", n)
	}

	// immutable guards stay installed after seed: direct event delete aborts
	if err := db.Exec(`DELETE FROM operation_task_events`).Error; err == nil {
		t.Fatal("expected immutable trigger to reject direct event delete")
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
	for _, table := range tables {
		if _, ok := vres.Counts[table]; !ok {
			t.Errorf("verify does not cover %s", table)
		}
	}
	for table, c := range vres.Counts {
		if c != 0 {
			t.Errorf("residual demo rows in %s: %d", table, c)
		}
	}

	// hard zero: operation task tables fully cleaned (no orphans by count)
	for _, m := range []any{
		&operationtask.OperationTask{}, &operationtask.PlatformDraft{},
		&operationtask.ApprovalRecord{}, &operationtask.ExecutionAttempt{},
		&operationtask.ExecutionError{}, &operationtask.OperationTaskEvent{},
	} {
		var c int64
		if err := db.Model(m).Count(&c).Error; err != nil {
			t.Fatal(err)
		}
		if c != 0 {
			t.Errorf("%T not fully cleaned: %d rows", m, c)
		}
	}

	// guards restored after cleanup: append then direct delete still aborts
	if _, err := s.Seed(ctx); err != nil {
		t.Fatalf("seed after cleanup: %v", err)
	}
	if err := db.Exec(`DELETE FROM operation_task_events`).Error; err == nil {
		t.Fatal("expected immutable trigger to be re-installed after cleanup")
	}
}

func TestFullDemoProductionGuardCoversOperationTasks(t *testing.T) {
	s := &FullDemoSeeder{AppEnv: "production"}
	if _, err := s.Seed(context.Background()); err == nil {
		t.Error("production seed must be refused")
	}
	if _, err := s.Cleanup(context.Background()); err == nil {
		t.Error("production cleanup must be refused")
	}
	if _, err := s.VerifyClean(context.Background()); err == nil {
		t.Error("production verify must be refused")
	}
}
