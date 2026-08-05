package productpublish

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
)

// Round112 regression: retrying a failed local_draft_create task used to be
// routed into the generic platform publish worker, which failed on the empty
// input snapshot with a raw English "empty task input". The worker must
// complete local draft tasks by rebuilding the draft from the product.
func TestProcessQueuedTaskCompletesLocalDraftRetry(t *testing.T) {
	db := newBatchIntegrationDB(t)
	svc := newBatchTestService(db)
	pid, sid := seedBatchProduct(t, db)

	task := ProductPublishTask{
		ProductID:     pid,
		ShopID:        sid,
		TargetStoreID: sid,
		Platform:      "shopee",
		TaskType:      TaskTypeLocalDraftCreate,
		Status:        TaskPending,
		PublishStatus: StatusReady,
		Mode:          PublishModeSaveAsPlatformDraft,
		PublishMode:   PublishModeSaveAsPlatformDraft,
	}
	if err := db.Create(&task).Error; err != nil {
		t.Fatal(err)
	}

	if err := svc.ProcessQueuedTask(context.Background(), task.ID, "worker-test"); err != nil {
		t.Fatalf("ProcessQueuedTask: %v", err)
	}

	var got ProductPublishTask
	if err := db.First(&got, "id = ?", task.ID).Error; err != nil {
		t.Fatal(err)
	}
	if got.Status != TaskSuccess {
		t.Fatalf("status = %s (error=%q), want %s", got.Status, got.ErrorMessage, TaskSuccess)
	}
	if got.PublishStatus != StatusDraftCreated {
		t.Fatalf("publish_status = %s, want %s", got.PublishStatus, StatusDraftCreated)
	}
	if got.ErrorMessage != "" || got.ErrorCode != "" {
		t.Fatalf("error fields should be cleared, got code=%q msg=%q", got.ErrorCode, got.ErrorMessage)
	}
	var pub ProductPublication
	if err := db.First(&pub, "publish_task_id = ?", task.ID).Error; err != nil {
		t.Fatalf("publication should be created: %v", err)
	}
	if pub.PublishStatus != StatusDraftCreated {
		t.Fatalf("publication publish_status = %s, want %s", pub.PublishStatus, StatusDraftCreated)
	}
}

// A local draft task whose product is gone must fail with Chinese user
// copy instead of a raw English worker error.
func TestProcessQueuedTaskLocalDraftMissingProductFailsInChinese(t *testing.T) {
	db := newBatchIntegrationDB(t)
	svc := newBatchTestService(db)

	task := ProductPublishTask{
		ProductID:     uuid.New(),
		ShopID:        uuid.New(),
		TargetStoreID: uuid.New(),
		Platform:      "shopee",
		TaskType:      TaskTypeLocalDraftCreate,
		Status:        TaskPending,
		PublishStatus: StatusReady,
		Mode:          PublishModeSaveAsPlatformDraft,
		PublishMode:   PublishModeSaveAsPlatformDraft,
	}
	if err := db.Create(&task).Error; err != nil {
		t.Fatal(err)
	}

	_ = svc.ProcessQueuedTask(context.Background(), task.ID, "worker-test")

	var got ProductPublishTask
	if err := db.First(&got, "id = ?", task.ID).Error; err != nil {
		t.Fatal(err)
	}
	if got.Status != TaskFailed {
		t.Fatalf("status = %s, want %s", got.Status, TaskFailed)
	}
	if !containsCJK(got.ErrorMessage) {
		t.Fatalf("error_message should be Chinese user copy, got %q", got.ErrorMessage)
	}
}

// Generic worker failures stored as user-visible error_message must have a
// Chinese fallback ("empty task input" style raw English must not leak).
func TestGenericWorkerEmptyInputFailsInChinese(t *testing.T) {
	db := newBatchIntegrationDB(t)
	svc := newBatchTestService(db)
	pid, sid := seedBatchProduct(t, db)

	task := ProductPublishTask{
		ProductID:     pid,
		ShopID:        sid,
		TargetStoreID: sid,
		Platform:      "shopee",
		Status:        TaskPending,
		PublishStatus: StatusReady,
		Mode:          ModeManual,
	}
	if err := db.Create(&task).Error; err != nil {
		t.Fatal(err)
	}

	_ = svc.ProcessQueuedTask(context.Background(), task.ID, "worker-test")

	var got ProductPublishTask
	if err := db.First(&got, "id = ?", task.ID).Error; err != nil {
		t.Fatal(err)
	}
	if got.Status != TaskFailed {
		t.Fatalf("status = %s, want %s", got.Status, TaskFailed)
	}
	if strings.Contains(got.ErrorMessage, "empty task input") {
		t.Fatalf("raw English error leaked: %q", got.ErrorMessage)
	}
	if !containsCJK(got.ErrorMessage) {
		t.Fatalf("error_message should be Chinese user copy, got %q", got.ErrorMessage)
	}
	if got.FinishedAt == nil || time.Since(*got.FinishedAt) > time.Minute {
		t.Fatalf("finished_at not set correctly: %v", got.FinishedAt)
	}
}

func containsCJK(s string) bool {
	for _, r := range s {
		if r > 0x2E7F {
			return true
		}
	}
	return false
}
