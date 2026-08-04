package productpublish

import (
	"context"
	"testing"

	"github.com/google/uuid"
)

// Round108 regression: a queued task rejected by the worker tenant gate must
// be marked failed with a user-visible reason instead of being dropped
// silently (which left it pending forever with no UI feedback).
func TestFailTaskTenantGateMarksTaskFailed(t *testing.T) {
	db := newBatchIntegrationDB(t)
	svc := newBatchTestService(db)
	task := ProductPublishTask{
		TenantID:      0,
		ProductID:     uuid.New(),
		ShopID:        uuid.New(),
		Platform:      "douyin",
		TargetStoreID: uuid.New(),
		TaskType:      TaskTypeDouyinDraftCreate,
		Status:        TaskPending,
		Mode:          ModeManual,
	}
	if err := db.Create(&task).Error; err != nil {
		t.Fatal(err)
	}
	svc.failTaskTenantGate(context.Background(), task.ID, "任务缺少租户上下文，已停止处理")
	var got ProductPublishTask
	if err := db.First(&got, "id = ?", task.ID).Error; err != nil {
		t.Fatal(err)
	}
	if got.Status != TaskFailed {
		t.Fatalf("status = %s, want %s", got.Status, TaskFailed)
	}
	if got.ErrorCode != "task_tenant_missing" {
		t.Fatalf("error_code = %s, want task_tenant_missing", got.ErrorCode)
	}
	if got.ErrorMessage == "" {
		t.Fatal("error_message should carry a user-visible reason")
	}
	if got.FinishedAt == nil {
		t.Fatal("finished_at should be set")
	}
}

// A task that already reached a terminal state must not be overwritten.
func TestFailTaskTenantGateSkipsTerminalTask(t *testing.T) {
	db := newBatchIntegrationDB(t)
	svc := newBatchTestService(db)
	task := ProductPublishTask{
		TenantID:      0,
		ProductID:     uuid.New(),
		ShopID:        uuid.New(),
		Platform:      "douyin",
		TargetStoreID: uuid.New(),
		TaskType:      TaskTypeDouyinDraftCreate,
		Status:        TaskSuccess,
		Mode:          ModeManual,
	}
	if err := db.Create(&task).Error; err != nil {
		t.Fatal(err)
	}
	svc.failTaskTenantGate(context.Background(), task.ID, "reason")
	var got ProductPublishTask
	if err := db.First(&got, "id = ?", task.ID).Error; err != nil {
		t.Fatal(err)
	}
	if got.Status != TaskSuccess {
		t.Fatalf("terminal task overwritten: status = %s", got.Status)
	}
}
