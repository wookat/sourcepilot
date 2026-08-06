package backup

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/trademind-ai/trademind/backend/internal/providers/backupstore"
)

// memStore is an in-memory backupstore.Store for service-level tests.
type memStore struct {
	objects     map[string][]byte
	modTimes    map[string]time.Time
	failUploads int
	uploadCalls int
	deleted     []string
}

func newMemStore() *memStore {
	return &memStore{objects: map[string][]byte{}, modTimes: map[string]time.Time{}}
}

func (m *memStore) Upload(_ context.Context, key, localPath, _ string) error {
	m.uploadCalls++
	if m.failUploads > 0 {
		m.failUploads--
		return fmt.Errorf("injected upload failure")
	}
	data, err := os.ReadFile(localPath)
	if err != nil {
		return err
	}
	m.objects[key] = data
	m.modTimes[key] = time.Now().UTC()
	return nil
}

func (m *memStore) Download(_ context.Context, key, localPath string) error {
	data, ok := m.objects[key]
	if !ok {
		return fmt.Errorf("object not found: %s", key)
	}
	if err := os.MkdirAll(filepath.Dir(localPath), 0o700); err != nil {
		return err
	}
	return os.WriteFile(localPath, data, 0o600)
}

func (m *memStore) List(_ context.Context, prefix string) ([]backupstore.Object, error) {
	var out []backupstore.Object
	for k, v := range m.objects {
		if len(prefix) == 0 || (len(k) >= len(prefix) && k[:len(prefix)] == prefix) {
			out = append(out, backupstore.Object{Key: k, Size: int64(len(v)), LastModified: m.modTimes[k]})
		}
	}
	return out, nil
}

func (m *memStore) Delete(_ context.Context, key string) error {
	delete(m.objects, key)
	m.deleted = append(m.deleted, key)
	return nil
}

func (m *memStore) Target() string { return "s3://test-bucket/backups/test (memstore)" }

func TestUploadArtifactSkippedWithoutStore(t *testing.T) {
	svc := newTestService(t)
	row := seedCompletedBackup(t, svc, false)
	var artifact Artifact
	if err := svc.DB.Where("backup_id = ?", row.BackupID).First(&artifact).Error; err != nil {
		t.Fatal(err)
	}
	svc.uploadArtifact(context.Background(), row, &artifact)
	if row.UploadStatus != UploadSkipped {
		t.Fatalf("expected skipped, got %q", row.UploadStatus)
	}
	if row.UploadTarget != "" {
		t.Fatalf("skipped upload must not set target, got %q", row.UploadTarget)
	}
}

func TestUploadArtifactRetriesThenSucceeds(t *testing.T) {
	svc := newTestService(t)
	store := newMemStore()
	store.failUploads = 2
	svc.Store = store
	svc.Cfg.Backup.UploadMaxAttempts = 3
	svc.Cfg.Backup.StoragePrefix = "backups/test"

	row := seedCompletedBackup(t, svc, false)
	var artifact Artifact
	if err := svc.DB.Where("backup_id = ?", row.BackupID).First(&artifact).Error; err != nil {
		t.Fatal(err)
	}
	svc.uploadArtifact(context.Background(), row, &artifact)
	if row.UploadStatus != UploadUploaded {
		t.Fatalf("expected uploaded after retries, got %q (%s)", row.UploadStatus, row.UploadError)
	}
	if row.UploadAttempts != 3 || store.uploadCalls != 3 {
		t.Fatalf("expected 3 attempts, got %d/%d", row.UploadAttempts, store.uploadCalls)
	}
	if row.UploadedAt == nil || row.UploadTarget == "" {
		t.Fatal("uploadedAt and uploadTarget must be set")
	}
	if artifact.ObjectKey != "backups/test/"+artifact.Name {
		t.Fatalf("unexpected object key %q", artifact.ObjectKey)
	}
	if _, ok := store.objects[artifact.ObjectKey]; !ok {
		t.Fatal("object not stored")
	}
}

func TestUploadArtifactFailureKeepsJobCompleted(t *testing.T) {
	svc := newTestService(t)
	store := newMemStore()
	store.failUploads = 10
	svc.Store = store
	svc.Cfg.Backup.UploadMaxAttempts = 2

	row := seedCompletedBackup(t, svc, false)
	var artifact Artifact
	if err := svc.DB.Where("backup_id = ?", row.BackupID).First(&artifact).Error; err != nil {
		t.Fatal(err)
	}
	svc.uploadArtifact(context.Background(), row, &artifact)
	if row.UploadStatus != UploadFailed {
		t.Fatalf("expected failed, got %q", row.UploadStatus)
	}
	if row.Status != StatusCompleted {
		t.Fatalf("upload failure must not fail the backup job, got %q", row.Status)
	}
	if row.UploadError == "" || row.UploadAttempts != 2 {
		t.Fatalf("expected recorded error and 2 attempts, got %q %d", row.UploadError, row.UploadAttempts)
	}
}

func TestRetryUploadSucceeds(t *testing.T) {
	svc := newTestService(t)
	store := newMemStore()
	svc.Store = store
	svc.Cfg.Backup.StoragePrefix = "backups/test"

	row := seedCompletedBackup(t, svc, false)
	got, err := svc.RetryUpload(context.Background(), row.BackupID)
	if err != nil {
		t.Fatal(err)
	}
	if got.UploadStatus != UploadUploaded {
		t.Fatalf("expected uploaded, got %q", got.UploadStatus)
	}
}

func TestRetryUploadWithoutStoreFails(t *testing.T) {
	svc := newTestService(t)
	row := seedCompletedBackup(t, svc, false)
	if _, err := svc.RetryUpload(context.Background(), row.BackupID); err == nil {
		t.Fatal("expected not-configured error")
	}
}

func TestPruneObjectStoreKeepsNewestAndHeld(t *testing.T) {
	svc := newTestService(t)
	store := newMemStore()
	svc.Store = store
	svc.Cfg.Backup.StoragePrefix = "backups/test"
	svc.Cfg.Backup.ObjectRetentionCount = 2

	base := time.Now().UTC()
	for i := 0; i < 5; i++ {
		key := fmt.Sprintf("backups/test/bk_prune_%03d.dump", i)
		store.objects[key] = []byte("x")
		store.modTimes[key] = base.Add(time.Duration(i) * time.Minute)
	}
	// bk_prune_000 is oldest but under retention hold → must survive.
	if err := svc.DB.Create(&RetentionHold{BackupID: "bk_prune_000", HoldType: HoldManual}).Error; err != nil {
		t.Fatal(err)
	}

	if err := svc.pruneObjectStore(context.Background()); err != nil {
		t.Fatal(err)
	}
	if _, ok := store.objects["backups/test/bk_prune_000.dump"]; !ok {
		t.Fatal("held backup object must not be pruned")
	}
	for _, key := range []string{"backups/test/bk_prune_004.dump", "backups/test/bk_prune_003.dump"} {
		if _, ok := store.objects[key]; !ok {
			t.Fatalf("newest object %s must be kept", key)
		}
	}
	for _, key := range []string{"backups/test/bk_prune_001.dump", "backups/test/bk_prune_002.dump"} {
		if _, ok := store.objects[key]; ok {
			t.Fatalf("old object %s must be pruned", key)
		}
	}
}

func TestPruneDisabledWhenRetentionZero(t *testing.T) {
	svc := newTestService(t)
	store := newMemStore()
	svc.Store = store
	svc.Cfg.Backup.ObjectRetentionCount = 0
	store.objects["backups/test/a.dump"] = []byte("x")
	if err := svc.pruneObjectStore(context.Background()); err != nil {
		t.Fatal(err)
	}
	if len(store.deleted) != 0 {
		t.Fatal("pruning must be disabled when retention count is 0")
	}
}

func TestDownloadFetchesMissingLocalFromObjectStore(t *testing.T) {
	svc := newTestService(t)
	store := newMemStore()
	svc.Store = store
	svc.Cfg.Backup.StoragePrefix = "backups/test"

	row := seedCompletedBackup(t, svc, false)
	row.VerificationStatus = VerificationPassed
	if err := svc.DB.Save(row).Error; err != nil {
		t.Fatal(err)
	}
	var artifact Artifact
	if err := svc.DB.Where("backup_id = ?", row.BackupID).First(&artifact).Error; err != nil {
		t.Fatal(err)
	}
	// Upload first, then simulate container churn removing the local file.
	svc.uploadArtifact(context.Background(), row, &artifact)
	if err := os.Remove(artifact.LocalPath); err != nil {
		t.Fatal(err)
	}

	_, got, err := svc.Download(context.Background(), row.BackupID)
	if err != nil {
		t.Fatalf("download must fetch artifact from object store: %v", err)
	}
	if _, err := os.Stat(got.LocalPath); err != nil {
		t.Fatal("local artifact must be restored from object store")
	}
}

func TestDownloadMissingLocalWithoutObjectCopyFails(t *testing.T) {
	svc := newTestService(t)
	row := seedCompletedBackup(t, svc, false)
	row.VerificationStatus = VerificationPassed
	if err := svc.DB.Save(row).Error; err != nil {
		t.Fatal(err)
	}
	var artifact Artifact
	if err := svc.DB.Where("backup_id = ?", row.BackupID).First(&artifact).Error; err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(artifact.LocalPath); err != nil {
		t.Fatal(err)
	}
	if _, _, err := svc.Download(context.Background(), row.BackupID); err == nil {
		t.Fatal("expected missing artifact error")
	}
}
