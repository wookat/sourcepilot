package backup

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"
)

// fakePgDump writes a script that behaves like pg_dump: it creates the
// --file output so the backup flow can checksum and upload it.
func fakePgDump(t *testing.T) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "fake-pg-dump")
	script := `#!/bin/sh
out=""
while [ $# -gt 0 ]; do
  if [ "$1" = "--file" ]; then out="$2"; shift; fi
  shift
done
printf 'fake dump payload' > "$out"
`
	if err := os.WriteFile(path, []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	return path
}

func newSchedulerTestService(t *testing.T) *Service {
	t.Helper()
	svc := newTestService(t)
	svc.Cfg.Backup.Enabled = true
	svc.Cfg.Backup.Mode = "hybrid"
	svc.Cfg.Backup.StorageProvider = "local"
	svc.Cfg.DB.Name = "trademind_sched_test"
	svc.Cfg.PostgresBackup.PGDumpPath = fakePgDump(t)
	return svc
}

func TestRunScheduledBackupCreatesJobWithUploadAndRetention(t *testing.T) {
	svc := newSchedulerTestService(t)
	store := newMemStore()
	svc.Store = store
	svc.Cfg.Backup.StoragePrefix = "backups/test"
	svc.Cfg.Backup.ObjectRetentionCount = 5

	fire := time.Date(2026, 8, 6, 3, 0, 0, 0, time.UTC)
	job, created, err := svc.RunScheduledBackup(context.Background(), fire)
	if err != nil {
		t.Fatal(err)
	}
	if !created {
		t.Fatal("first firing must create a job")
	}
	if job.Status != StatusCompleted {
		t.Fatalf("expected completed, got %q (%s)", job.Status, job.ErrorSummary)
	}
	if job.TriggerSource != TriggerScheduled {
		t.Fatalf("expected scheduled trigger source, got %q", job.TriggerSource)
	}
	if job.ScheduleKey == nil || *job.ScheduleKey != "sched_202608060300" {
		t.Fatalf("unexpected schedule key %v", job.ScheduleKey)
	}
	if job.UploadStatus != UploadUploaded {
		t.Fatalf("scheduled backup must run the S3 upload path, got %q (%s)", job.UploadStatus, job.UploadError)
	}
	if len(store.objects) == 0 {
		t.Fatal("artifact must be uploaded to object storage")
	}
}

func TestRunScheduledBackupIsIdempotentPerSlot(t *testing.T) {
	svc := newSchedulerTestService(t)
	fire := time.Date(2026, 8, 6, 3, 0, 0, 0, time.UTC)

	first, created, err := svc.RunScheduledBackup(context.Background(), fire)
	if err != nil || !created {
		t.Fatalf("first firing: created=%v err=%v", created, err)
	}
	second, created, err := svc.RunScheduledBackup(context.Background(), fire)
	if err != nil {
		t.Fatal(err)
	}
	if created {
		t.Fatal("duplicate firing of the same slot must not create a second backup")
	}
	if second.BackupID != first.BackupID {
		t.Fatalf("duplicate firing must return the existing job, got %s vs %s", second.BackupID, first.BackupID)
	}
	var count int64
	if err := svc.DB.Model(&Job{}).Count(&count).Error; err != nil {
		t.Fatal(err)
	}
	if count != 1 {
		t.Fatalf("expected exactly 1 job, got %d", count)
	}

	// A different slot creates a new job.
	_, created, err = svc.RunScheduledBackup(context.Background(), fire.Add(24*time.Hour))
	if err != nil || !created {
		t.Fatalf("next slot: created=%v err=%v", created, err)
	}
}

func TestRunScheduledBackupFailurePersistsVisibleStatus(t *testing.T) {
	svc := newSchedulerTestService(t)
	svc.Cfg.PostgresBackup.PGDumpPath = filepath.Join(t.TempDir(), "missing-pg-dump")

	fire := time.Date(2026, 8, 6, 3, 0, 0, 0, time.UTC)
	job, created, err := svc.RunScheduledBackup(context.Background(), fire)
	if err == nil {
		t.Fatal("expected scheduled backup failure")
	}
	if !created || job == nil {
		t.Fatalf("failed run must still persist a job row: created=%v job=%v", created, job)
	}
	var row Job
	if err := svc.DB.Where("backup_id = ?", job.BackupID).First(&row).Error; err != nil {
		t.Fatal(err)
	}
	if row.Status != StatusFailed || row.ErrorSummary == "" {
		t.Fatalf("failure must be visible on the job row: status=%q summary=%q", row.Status, row.ErrorSummary)
	}
	if row.TriggerSource != TriggerScheduled {
		t.Fatalf("expected scheduled trigger source, got %q", row.TriggerSource)
	}

	// The failed slot stays claimed: a duplicate firing does not rerun it.
	dup, created, err := svc.RunScheduledBackup(context.Background(), fire)
	if err != nil || created {
		t.Fatalf("duplicate firing after failure: created=%v err=%v", created, err)
	}
	if dup.BackupID != job.BackupID {
		t.Fatalf("expected existing failed job, got %s", dup.BackupID)
	}
}

func TestManualBackupHasNoScheduleKey(t *testing.T) {
	svc := newSchedulerTestService(t)
	job, err := svc.CreateDatabaseBackup(context.Background(), CreateRequest{Reason: "manual"}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if job.TriggerSource != TriggerManual || job.ScheduleKey != nil {
		t.Fatalf("manual backup must not carry a schedule key: %q %v", job.TriggerSource, job.ScheduleKey)
	}
}
