package collect

import (
	"context"
	"fmt"
	"time"

	"github.com/trademind-ai/trademind/backend/internal/modules/operationlog"
)

const (
	// ProcessingTimeoutSettingKey lives in settings group "collector" (seconds).
	ProcessingTimeoutSettingKey = "collect_task_processing_timeout_seconds"

	defaultProcessingTimeoutSeconds = 600
	minProcessingTimeoutSeconds     = 30
	processingTimeoutReapBatchSize  = 50
)

var processingStatuses = []string{StatusPending, StatusRunning, StatusRetrying}

// ProcessingTimeoutSeconds resolves the stuck-task threshold from settings (group collector).
func (s *Service) ProcessingTimeoutSeconds(ctx context.Context) int {
	if s != nil && s.Settings != nil {
		if m, err := s.Settings.PlainByGroup(ctx, 0, "collector"); err == nil {
			if v := settingsInt(m, ProcessingTimeoutSettingKey); v > 0 {
				if v < minProcessingTimeoutSeconds {
					return minProcessingTimeoutSeconds
				}
				return v
			}
		}
	}
	return defaultProcessingTimeoutSeconds
}

func processingTimeoutMessage(thresholdSec int) string {
	return fmt.Sprintf("任务超时：处理超过 %d 秒仍未完成，已自动终止，可点击重试重新采集", thresholdSec)
}

// ReapProcessingTimeouts fails tasks stuck in pending/running/retrying beyond the
// configured threshold, counted from the last (re)queue time. Returns failed count.
func (s *Service) ReapProcessingTimeouts(ctx context.Context) int {
	if s == nil || s.DB == nil {
		return 0
	}
	if ctx == nil {
		ctx = context.Background()
	}
	thresholdSec := s.ProcessingTimeoutSeconds(ctx)
	now := time.Now().UTC()
	cutoff := now.Add(-time.Duration(thresholdSec) * time.Second)

	var stuck []CollectTask
	if err := s.DB.WithContext(ctx).
		Where("status IN ? AND COALESCE(queued_at, created_at) < ?", processingStatuses, cutoff).
		Order("created_at ASC").
		Limit(processingTimeoutReapBatchSize).
		Find(&stuck).Error; err != nil {
		return 0
	}
	n := 0
	for i := range stuck {
		if s.failProcessingTimeout(ctx, &stuck[i], thresholdSec) {
			n++
		}
	}
	return n
}

// failProcessingTimeout moves one stuck task to the failed terminal state (guarded update).
func (s *Service) failProcessingTimeout(ctx context.Context, task *CollectTask, thresholdSec int) bool {
	if s == nil || s.DB == nil || task == nil {
		return false
	}
	fin := time.Now().UTC()
	msg := processingTimeoutMessage(thresholdSec)
	res := s.DB.WithContext(ctx).Model(&CollectTask{}).
		Where("id = ? AND status = ?", task.ID, task.Status).
		Updates(map[string]interface{}{
			"status":            StatusFailed,
			"error_message":     msg,
			"finished_at":       &fin,
			"next_retry_at":     nil,
			"retry_enqueued_at": nil,
			"locked_by":         nil,
			"locked_until":      nil,
			"execution_id":      nil,
			"heartbeat_at":      nil,
			"updated_at":        fin,
		})
	if res.Error != nil || res.RowsAffected == 0 {
		return false
	}

	s.RecordTaskEvent(ctx, task, TaskEventInput{
		EventType:    EventTaskProcessingTimeout,
		FromStatus:   task.Status,
		ToStatus:     StatusFailed,
		Message:      "processing timeout reached",
		ErrorMessage: msg,
		RetryCount:   task.RetryCount,
		MaxRetries:   s.effectiveMaxRetries(task),
		PayloadMap:   map[string]any{"thresholdSeconds": thresholdSec, "retryable": true},
	})

	if s.OpLog != nil {
		_ = s.OpLog.WriteBackground(ctx, operationlog.WriteOpts{
			AdminUserID: task.CreatedBy,
			Action:      "collect.task.processing_timeout",
			Resource:    "collect_task",
			ResourceID:  task.ID.String(),
			Status:      "failed",
			Message:     truncateRunes(msg, 2000),
		})
	}
	if task.BatchID != nil {
		s.reconcileCollectBatchWithTerminalLog(ctx, task.BatchID)
	}
	return true
}
