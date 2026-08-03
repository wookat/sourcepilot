package demoseed

import (
	"context"
	"testing"

	"github.com/trademind-ai/trademind/backend/internal/modules/productpublish"
)

// Round 77: the publish task page's two tabs must be non-empty out of the
// box — one multi-product DEMO publish batch plus child tasks covering
// success / failed-retryable / pending. Seed stays idempotent and
// clean/verify leaves zero DEMO- residual rows.
func TestFullDemoSeedPublishBatchWithTasks(t *testing.T) {
	db := openFullDemoTestDB(t)
	if db == nil {
		return
	}
	seeder := &FullDemoSeeder{DB: db, TenantID: 11, AppEnv: "development"}
	ctx := context.Background()

	for run := 0; run < 2; run++ { // twice: idempotency
		if _, err := seeder.Seed(ctx); err != nil {
			t.Fatalf("seed run %d: %v", run+1, err)
		}
	}

	var batches []productpublish.ProductPublishBatch
	if err := db.Where("name LIKE ?", DemoPrefix+"%").Find(&batches).Error; err != nil {
		t.Fatal(err)
	}
	if len(batches) != 1 {
		t.Fatalf("expected exactly 1 demo publish batch, got %d", len(batches))
	}
	batch := batches[0]
	if batch.BatchType != productpublish.BatchTypeMultiProduct {
		t.Errorf("batch must be multi_product to show in the batch list, got %s", batch.BatchType)
	}
	if batch.Status != productpublish.BatchPartialSuccess || batch.FinishedAt == nil {
		t.Errorf("batch misconfigured: status=%s finishedAt=%v", batch.Status, batch.FinishedAt)
	}

	var tasks []productpublish.ProductPublishTask
	if err := db.Where("batch_id = ?", batch.ID).Find(&tasks).Error; err != nil {
		t.Fatal(err)
	}
	if len(tasks) != 3 {
		t.Fatalf("expected 3 demo publish tasks, got %d", len(tasks))
	}
	statuses := map[string]bool{}
	for _, task := range tasks {
		statuses[task.Status] = true
		if task.TenantID != seeder.TenantID {
			t.Errorf("task tenant mismatch: %d", task.TenantID)
		}
		if task.TaskType != productpublish.TaskTypeLocalDraftCreate {
			t.Errorf("unexpected task type: %s", task.TaskType)
		}
		if task.Status == productpublish.TaskFailed && !task.Retryable {
			t.Error("failed demo task must be retryable")
		}
	}
	for _, want := range []string{productpublish.TaskSuccess, productpublish.TaskFailed, productpublish.TaskPending} {
		if !statuses[want] {
			t.Errorf("missing demo task status sample: %s", want)
		}
	}

	// Clean leaves zero DEMO residuals.
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
	var n int64
	if err := db.Model(&productpublish.ProductPublishTask{}).Where("batch_id = ?", batch.ID).Count(&n).Error; err != nil {
		t.Fatal(err)
	}
	if n != 0 {
		t.Errorf("residual demo publish tasks after clean: %d", n)
	}
}
