package demoseed

import (
	"context"
	"testing"

	"github.com/trademind-ai/trademind/backend/internal/modules/productpublish"
)

// Round 112: every seeded demo publish task carries a real input sample so
// the retry flow is demonstrable (the worker used to fail retried demo tasks
// with a raw "empty task input"), and cleanup/verify still remove them all.
func TestFullDemoSeedPublishTasksHaveInput(t *testing.T) {
	db := openFullDemoTestDB(t)
	s := &FullDemoSeeder{DB: db, TenantID: 1, AppEnv: "development"}

	if _, err := s.Seed(context.Background()); err != nil {
		t.Fatal(err)
	}

	var tasks []productpublish.ProductPublishTask
	if err := db.Find(&tasks).Error; err != nil {
		t.Fatal(err)
	}
	if len(tasks) == 0 {
		t.Fatal("expected demo publish tasks to be seeded")
	}
	var failedRetryable int
	for _, task := range tasks {
		if len(task.Input) == 0 {
			t.Fatalf("task %s (%s) has empty input", task.ID, task.Status)
		}
		if task.Status == productpublish.TaskFailed && task.Retryable {
			failedRetryable++
		}
	}
	if failedRetryable == 0 {
		t.Fatal("expected a retryable failed demo task for the retry flow demo")
	}

	if _, err := s.Cleanup(context.Background()); err != nil {
		t.Fatal(err)
	}
	verify, err := s.VerifyClean(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if verify.Counts["product_publish_tasks"] != 0 {
		t.Fatalf("expected zero demo publish tasks after cleanup, got %d", verify.Counts["product_publish_tasks"])
	}
	if verify.Counts["product_publish_batches"] != 0 {
		t.Fatalf("expected zero demo publish batches after cleanup, got %d", verify.Counts["product_publish_batches"])
	}
}
