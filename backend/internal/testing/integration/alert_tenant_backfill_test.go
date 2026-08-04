package integration

import (
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"github.com/trademind-ai/trademind/backend/internal/database"
	"github.com/trademind-ai/trademind/backend/internal/modules/collect"
	"github.com/trademind-ai/trademind/backend/internal/modules/platformtenant"
	"github.com/trademind-ai/trademind/backend/internal/modules/taskcenter"
	"github.com/trademind-ai/trademind/backend/internal/testing/safeenv"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

// Regression (R105): legacy tenant-0 task_alerts are backfilled with the
// owning tenant of their source task row; alerts whose source row is gone
// stay in the tenant-0 bucket; the backfill is idempotent.
func TestRound105AlertTenantBackfill(t *testing.T) {
	t.Setenv("APP_ENV", "test")
	cfg, ok, err := safeenv.TestDatabaseURLFromEnv()
	require.NoError(t, err)
	if !ok {
		t.Skip("TEST_DATABASE_URL is not set; skipping PostgreSQL integration migration test")
	}

	db, err := gorm.Open(postgres.Open(cfg.URL), &gorm.Config{})
	require.NoError(t, err)
	sqlDB, err := db.DB()
	require.NoError(t, err)
	t.Cleanup(func() { sqlDB.Close() })

	require.NoError(t, database.AutoMigrate(db))

	cleanup := func() {
		db.Exec("DELETE FROM task_alerts WHERE failure_category = ?", "r105-test")
		db.Exec("DELETE FROM collect_tasks WHERE source_url LIKE ?", "https://r105.example%")
		db.Exec("DELETE FROM tenants WHERE name = ?", "r105-tenant")
	}
	cleanup()
	t.Cleanup(cleanup)

	tn := &platformtenant.Tenant{Name: "r105-tenant", Status: "active"}
	require.NoError(t, db.Create(tn).Error)

	ct := collect.CollectTask{TenantID: tn.ID, SourceURL: "https://r105.example/p", Status: "failed"}
	require.NoError(t, db.Create(&ct).Error)

	now := time.Now().UTC()
	mk := func(sourceID string) taskcenter.TaskAlert {
		return taskcenter.TaskAlert{
			ID:              uuid.New(),
			TenantID:        0,
			TaskType:        taskcenter.TaskTypeCollect,
			SourceID:        sourceID,
			SourceTable:     taskcenter.SourceTableCollectTasks,
			FailureCategory: "r105-test",
			Severity:        "high",
			Title:           "t",
			Status:          taskcenter.TaskAlertStatusOpen,
			AlertCount:      1,
			FirstSeenAt:     now,
			LastSeenAt:      now,
			CreatedAt:       now,
			UpdatedAt:       now,
		}
	}
	linked := mk(ct.ID.String())
	orphan := mk(uuid.NewString())
	require.NoError(t, db.Create(&linked).Error)
	require.NoError(t, db.Create(&orphan).Error)

	// AutoMigrate runs the round105 backfill; run twice to prove idempotency.
	require.NoError(t, database.AutoMigrate(db))
	require.NoError(t, database.AutoMigrate(db))

	var got taskcenter.TaskAlert
	require.NoError(t, db.First(&got, "id = ?", linked.ID).Error)
	require.Equal(t, tn.ID, got.TenantID, "alert with a live source row must gain its tenant")

	var gotOrphan taskcenter.TaskAlert
	require.NoError(t, db.First(&gotOrphan, "id = ?", orphan.ID).Error)
	require.EqualValues(t, 0, gotOrphan.TenantID, "orphan alert must stay in the tenant-0 bucket")
}
