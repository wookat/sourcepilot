package taskcenter

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/glebarez/sqlite"
	"github.com/google/uuid"
	"github.com/trademind-ai/trademind/backend/internal/modules/collect"
	"github.com/trademind-ai/trademind/backend/internal/modules/taskcenter/failureclassifier"
	"gorm.io/gorm"
)

func newAlertTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	dsn := fmt.Sprintf("file:alerts_src_%s?mode=memory&cache=shared", uuid.NewString())
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Skipf("sqlite unavailable: %v", err)
	}
	if err := db.AutoMigrate(&TaskAlert{}, &TaskAlertNotification{}, &collect.CollectTask{}); err != nil {
		t.Fatal(err)
	}
	return db
}

// 告警 Upsert 必须以来源任务行的租户作为告警 tenant_id（#221 P2-1 闭环）。
func TestUpsertAlertForFailureUsesSourceTenant(t *testing.T) {
	db := newAlertTestDB(t)
	ct := collect.CollectTask{TenantID: 2, SourceURL: "https://example.com/p", Status: "failed"}
	if err := db.Create(&ct).Error; err != nil {
		t.Fatal(err)
	}
	s := &Service{DB: db}
	now := time.Now().UTC()
	dto := UnifiedTaskDTO{
		TaskType:    TaskTypeCollect,
		SourceID:    ct.ID.String(),
		SourceTable: SourceTableCollectTasks,
	}
	class := failureclassifier.Result{Category: "network", Severity: "high", Reason: "boom"}
	gen, _, err := s.UpsertAlertForFailure(context.Background(), dto, class, now, true, nil)
	if err != nil {
		t.Fatal(err)
	}
	if !gen {
		t.Fatal("expected alert generated")
	}
	var a TaskAlert
	if err := db.Where("task_type = ? AND source_id = ?", TaskTypeCollect, ct.ID.String()).First(&a).Error; err != nil {
		t.Fatal(err)
	}
	if a.TenantID != 2 {
		t.Fatalf("alert must carry the source task tenant, got %d", a.TenantID)
	}

	// Bumping an existing legacy tenant-0 row self-heals its tenant_id.
	if err := db.Model(&TaskAlert{}).Where("id = ?", a.ID).Update("tenant_id", 0).Error; err != nil {
		t.Fatal(err)
	}
	if _, _, err := s.UpsertAlertForFailure(context.Background(), dto, class, now.Add(time.Minute), true, nil); err != nil {
		t.Fatal(err)
	}
	var a2 TaskAlert
	if err := db.First(&a2, "id = ?", a.ID).Error; err != nil {
		t.Fatal(err)
	}
	if a2.TenantID != 2 {
		t.Fatalf("bump must backfill tenant_id from source, got %d", a2.TenantID)
	}
}

// 通知审计列表必须按告警归属租户过滤，业务租户不得看到其它租户/平台的通知记录。
func TestListAlertNotificationsTenantScope(t *testing.T) {
	db := newAlertTestDB(t)
	now := time.Now().UTC()
	for _, tid := range []int64{0, 2} {
		a := TaskAlert{
			ID:              uuid.New(),
			TenantID:        tid,
			TaskType:        TaskTypeCollect,
			SourceID:        fmt.Sprintf("src-%d", tid),
			FailureCategory: "network",
			Severity:        "high",
			Title:           "t",
			Status:          TaskAlertStatusOpen,
			AlertCount:      1,
			FirstSeenAt:     now,
			LastSeenAt:      now,
			CreatedAt:       now,
			UpdatedAt:       now,
		}
		if err := db.Create(&a).Error; err != nil {
			t.Fatal(err)
		}
		n := TaskAlertNotification{
			ID:        uuid.New(),
			AlertID:   a.ID,
			Channel:   "mail",
			Status:    TaskAlertNotifStatusSuccess,
			Target:    fmt.Sprintf("t%d@example.com", tid),
			CreatedAt: now,
		}
		if err := db.Create(&n).Error; err != nil {
			t.Fatal(err)
		}
	}
	s := &Service{DB: db}
	res, err := s.ListAlertNotifications(context.Background(), ListAlertNotificationsParams{TenantID: 2})
	if err != nil {
		t.Fatal(err)
	}
	if res.Total != 1 || len(res.List) != 1 || res.List[0].Target != "t2@example.com" {
		t.Fatalf("tenant 2 should only see its own notifications, got total=%d list=%v", res.Total, res.List)
	}
	res0, err := s.ListAlertNotifications(context.Background(), ListAlertNotificationsParams{TenantID: 0})
	if err != nil {
		t.Fatal(err)
	}
	if res0.Total != 1 || res0.List[0].Target != "t0@example.com" {
		t.Fatalf("tenant 0 should only see tenant-0 notifications, got total=%d", res0.Total)
	}
}
