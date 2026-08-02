package collect

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/glebarez/sqlite"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"github.com/trademind-ai/trademind/backend/internal/pkg/model"
	"gorm.io/gorm"
)

func openProcessingTimeoutTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	dsn := fmt.Sprintf("file:collect_timeout_%s?mode=memory&cache=shared", uuid.New().String())
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&CollectTask{}, &CollectTaskEvent{}))
	return db
}

func newProcessingTask(t *testing.T, db *gorm.DB, status string, queuedAgo time.Duration) uuid.UUID {
	t.Helper()
	id := uuid.New()
	queuedAt := time.Now().UTC().Add(-queuedAgo)
	require.NoError(t, db.Create(&CollectTask{
		HardDeleteBase: model.HardDeleteBase{ID: id},
		Source:         "1688",
		SourceURL:      "https://detail.1688.com/offer/" + id.String() + ".html",
		Status:         status,
		QueuedAt:       &queuedAt,
	}).Error)
	return id
}

func TestReapProcessingTimeoutsFailsStuckTasks(t *testing.T) {
	db := openProcessingTimeoutTestDB(t)
	svc := &Service{DB: db}
	ctx := context.Background()

	pendingID := newProcessingTask(t, db, StatusPending, 2*defaultProcessingTimeoutSeconds*time.Second)
	runningID := newProcessingTask(t, db, StatusRunning, 2*defaultProcessingTimeoutSeconds*time.Second)
	retryingID := newProcessingTask(t, db, StatusRetrying, 2*defaultProcessingTimeoutSeconds*time.Second)

	n := svc.ReapProcessingTimeouts(ctx)
	require.Equal(t, 3, n)

	for _, id := range []uuid.UUID{pendingID, runningID, retryingID} {
		var row CollectTask
		require.NoError(t, db.First(&row, "id = ?", id).Error)
		require.Equal(t, StatusFailed, row.Status)
		require.Contains(t, row.ErrorMessage, "任务超时")
		require.NotNil(t, row.FinishedAt)
		require.Nil(t, row.NextRetryAt)
		require.Nil(t, row.LockedBy)
		require.Nil(t, row.LockedUntil)

		var event CollectTaskEvent
		require.NoError(t, db.First(&event, "task_id = ? AND event_type = ?", id, EventTaskProcessingTimeout).Error)
		require.NotNil(t, event.ToStatus)
		require.Equal(t, StatusFailed, *event.ToStatus)
	}
}

func TestReapProcessingTimeoutsKeepsFreshAndTerminalTasks(t *testing.T) {
	db := openProcessingTimeoutTestDB(t)
	svc := &Service{DB: db}
	ctx := context.Background()

	freshID := newProcessingTask(t, db, StatusRunning, 10*time.Second)
	successID := newProcessingTask(t, db, StatusSuccess, 2*defaultProcessingTimeoutSeconds*time.Second)
	failedID := newProcessingTask(t, db, StatusFailed, 2*defaultProcessingTimeoutSeconds*time.Second)

	n := svc.ReapProcessingTimeouts(ctx)
	require.Equal(t, 0, n)

	for id, want := range map[uuid.UUID]string{
		freshID:   StatusRunning,
		successID: StatusSuccess,
		failedID:  StatusFailed,
	} {
		var row CollectTask
		require.NoError(t, db.First(&row, "id = ?", id).Error)
		require.Equal(t, want, row.Status)
	}
}

func TestReapProcessingTimeoutsRestartsClockAfterRetry(t *testing.T) {
	db := openProcessingTimeoutTestDB(t)
	svc := &Service{DB: db}
	ctx := context.Background()

	// Task created long ago, then manually retried just now: queued_at was reset,
	// so the timeout clock restarts and the task must NOT be reaped.
	id := uuid.New()
	created := time.Now().UTC().Add(-3 * defaultProcessingTimeoutSeconds * time.Second)
	requeued := time.Now().UTC()
	require.NoError(t, db.Create(&CollectTask{
		HardDeleteBase: model.HardDeleteBase{ID: id, CreatedAt: created, UpdatedAt: created},
		Source:         "1688",
		SourceURL:      "https://detail.1688.com/offer/retry.html",
		Status:         StatusRetrying,
		QueuedAt:       &requeued,
	}).Error)
	require.NoError(t, db.Model(&CollectTask{}).Where("id = ?", id).
		Update("created_at", created).Error)

	n := svc.ReapProcessingTimeouts(ctx)
	require.Equal(t, 0, n)

	var row CollectTask
	require.NoError(t, db.First(&row, "id = ?", id).Error)
	require.Equal(t, StatusRetrying, row.Status)
}

func TestReapProcessingTimeoutsFallsBackToCreatedAtForLegacyRows(t *testing.T) {
	db := openProcessingTimeoutTestDB(t)
	svc := &Service{DB: db}
	ctx := context.Background()

	// Legacy row without queued_at: created_at drives the timeout.
	id := uuid.New()
	created := time.Now().UTC().Add(-2 * defaultProcessingTimeoutSeconds * time.Second)
	require.NoError(t, db.Create(&CollectTask{
		HardDeleteBase: model.HardDeleteBase{ID: id},
		Source:         "1688",
		SourceURL:      "https://detail.1688.com/offer/legacy.html",
		Status:         StatusPending,
	}).Error)
	require.NoError(t, db.Model(&CollectTask{}).Where("id = ?", id).
		Update("created_at", created).Error)

	n := svc.ReapProcessingTimeouts(ctx)
	require.Equal(t, 1, n)

	var row CollectTask
	require.NoError(t, db.First(&row, "id = ?", id).Error)
	require.Equal(t, StatusFailed, row.Status)
	require.Contains(t, row.ErrorMessage, "任务超时")
}

func TestProcessingTimeoutSecondsDefault(t *testing.T) {
	svc := &Service{}
	require.Equal(t, defaultProcessingTimeoutSeconds, svc.ProcessingTimeoutSeconds(context.Background()))
}
