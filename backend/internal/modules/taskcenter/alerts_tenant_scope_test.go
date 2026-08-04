package taskcenter

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/glebarez/sqlite"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

// 任务告警列表必须按调用方租户过滤，业务租户不得看到其它租户/平台的告警。
func TestListAlertsTenantScope(t *testing.T) {
	dsn := fmt.Sprintf("file:alerts_%s?mode=memory&cache=shared", uuid.NewString())
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Skipf("sqlite unavailable: %v", err)
	}
	if err := db.AutoMigrate(&TaskAlert{}, &TaskAlertNotification{}); err != nil {
		t.Fatal(err)
	}
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
	}
	s := &Service{DB: db}

	res, err := s.ListAlerts(context.Background(), ListAlertsParams{TenantID: 2})
	if err != nil {
		t.Fatal(err)
	}
	if res.Total != 1 {
		t.Fatalf("tenant 2 should only see its own alerts, got total=%d", res.Total)
	}

	res0, err := s.ListAlerts(context.Background(), ListAlertsParams{TenantID: 0})
	if err != nil {
		t.Fatal(err)
	}
	if res0.Total != 1 {
		t.Fatalf("tenant 0 should only see tenant-0 alerts, got total=%d", res0.Total)
	}
}
