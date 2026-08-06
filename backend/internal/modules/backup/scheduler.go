package backup

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"sync"
	"time"

	"github.com/trademind-ai/trademind/backend/internal/config"
	"github.com/trademind-ai/trademind/backend/internal/pkg/backupsched"
	"gorm.io/gorm"
)

// scheduleKeyFor identifies one schedule slot (minute precision). Jobs carry
// it in a unique column so a slot never produces more than one backup even if
// the timer fires twice or two processes race.
func scheduleKeyFor(fireTime time.Time) string {
	return "sched_" + fireTime.UTC().Format("200601021504")
}

// RunScheduledBackup executes one scheduler firing. It is idempotent per
// schedule slot: when a job for the slot already exists (from an earlier
// duplicate firing or a concurrent process) it returns that job with
// created=false and does not run a second backup. Failures are persisted on
// the job row (status=failed + errorSummary) so they stay visible in the Ops
// backups list.
func (s *Service) RunScheduledBackup(ctx context.Context, fireTime time.Time) (*Job, bool, error) {
	if s.DB == nil || s.Cfg == nil {
		return nil, false, fmt.Errorf("backup service unavailable")
	}
	key := scheduleKeyFor(fireTime)
	var existing Job
	err := s.DB.WithContext(ctx).Where("schedule_key = ?", key).First(&existing).Error
	if err == nil {
		return &existing, false, nil
	}
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, false, err
	}
	row, err := s.createBackup(ctx, CreateRequest{Reason: "scheduled backup " + key}, nil, TriggerScheduled, &key)
	if err != nil && isDuplicateScheduleKey(err) {
		if lookupErr := s.DB.WithContext(ctx).Where("schedule_key = ?", key).First(&existing).Error; lookupErr == nil {
			return &existing, false, nil
		}
	}
	return row, true, err
}

// isDuplicateScheduleKey detects a unique-index violation on schedule_key
// from a concurrent firing of the same slot (PostgreSQL 23505 / sqlite
// UNIQUE constraint wording).
func isDuplicateScheduleKey(err error) bool {
	if errors.Is(err, gorm.ErrDuplicatedKey) {
		return true
	}
	msg := strings.ToLower(err.Error())
	return strings.Contains(msg, "duplicate key") || strings.Contains(msg, "unique constraint")
}

// StartScheduler runs the built-in backup scheduler when
// BACKUP_SCHEDULE_ENABLED=true. Each fire runs the full backup flow
// (pg_dump, optional encryption, S3 upload and object retention pruning).
// When disabled nothing runs and the host crontab path stays the alternative
// trigger.
func StartScheduler(ctx context.Context, wg *sync.WaitGroup, log *slog.Logger, svc *Service, cfg *config.Config) {
	if cfg == nil || !cfg.Backup.ScheduleEnabled {
		return
	}
	sched, err := backupsched.Parse(cfg.Backup.Schedule)
	if err != nil {
		// Config validation rejects this at startup; keep a guard anyway.
		log.Error("backup_scheduler_invalid_schedule", "error", err)
		return
	}
	log.Info("backup_scheduler_started", "schedule", sched.String())
	wg.Add(1)
	go func() {
		defer wg.Done()
		for {
			next := sched.Next(time.Now())
			if next.IsZero() {
				log.Error("backup_scheduler_stopped", "reason", "schedule has no future fire time")
				return
			}
			timer := time.NewTimer(time.Until(next))
			select {
			case <-ctx.Done():
				timer.Stop()
				log.Info("backup_scheduler_stopped", "reason", "shutdown")
				return
			case <-timer.C:
			}
			job, created, err := svc.RunScheduledBackup(ctx, next)
			switch {
			case err != nil && job != nil:
				log.Error("backup_scheduled_run_failed", "backupId", job.BackupID, "scheduleKey", scheduleKeyFor(next), "error", err)
			case err != nil:
				log.Error("backup_scheduled_run_failed", "scheduleKey", scheduleKeyFor(next), "error", err)
			case !created:
				log.Info("backup_scheduled_run_skipped_duplicate", "scheduleKey", scheduleKeyFor(next), "backupId", job.BackupID)
			default:
				log.Info("backup_scheduled_run_completed", "backupId", job.BackupID, "scheduleKey", scheduleKeyFor(next), "status", job.Status)
			}
		}
	}()
}
