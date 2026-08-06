package backup

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/glebarez/sqlite"
	"github.com/trademind-ai/trademind/backend/internal/config"
	"github.com/trademind-ai/trademind/backend/internal/pkg/backupruntime"
	"gorm.io/datatypes"
	"gorm.io/gorm"
)

func newTestService(t *testing.T) *Service {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Skipf("sqlite unavailable: %v", err)
	}
	if err := db.AutoMigrate(&Job{}, &Artifact{}, &Verification{}, &RetentionHold{}); err != nil {
		t.Fatal(err)
	}
	cfg := &config.Config{AppEnv: config.EnvDevelopment}
	cfg.Backup.CommandTimeoutSeconds = 30
	cfg.PostgresBackup.PGRestorePath = "true" // stand-in binary that always succeeds
	return &Service{DB: db, Cfg: cfg, WorkRoot: t.TempDir()}
}

func seedCompletedBackup(t *testing.T, svc *Service, encrypted bool) *Job {
	t.Helper()
	dir := filepath.Join(svc.workRoot(), "bk_test_001")
	if err := os.MkdirAll(dir, 0o700); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(dir, "bk_test_001.dump")
	if err := os.WriteFile(path, []byte("fake backup artifact payload"), 0o600); err != nil {
		t.Fatal(err)
	}
	sum, size, err := backupruntime.SHA256File(path)
	if err != nil {
		t.Fatal(err)
	}
	now := time.Now().UTC()
	row := &Job{
		BackupID: "bk_test_001", BackupType: TypePostgresLogical, Environment: "test",
		Status: StatusCompleted, VerificationStatus: VerificationPending,
		StorageProvider: "local", Encrypted: encrypted, EncryptionKeyID: "app-master-key",
		Checksum: sum, ArtifactSize: size, CompletedAt: &now,
	}
	manifest := svc.buildManifest(row, "bk_test_001.dump", size, sum, "")
	raw, err := json.Marshal(manifest)
	if err != nil {
		t.Fatal(err)
	}
	row.ManifestJSON = datatypes.JSON(raw)
	if err := svc.DB.Create(row).Error; err != nil {
		t.Fatal(err)
	}
	artifact := &Artifact{BackupID: row.BackupID, Name: "bk_test_001.dump", Size: size, SHA256: sum, StorageProvider: "local", LocalPath: path}
	if err := svc.DB.Create(artifact).Error; err != nil {
		t.Fatal(err)
	}
	return row
}

func TestVerifyUnencryptedBackupSkipsEncryptionCheck(t *testing.T) {
	svc := newTestService(t)
	seedCompletedBackup(t, svc, false)
	v, err := svc.Verify(context.Background(), "bk_test_001")
	if err != nil {
		t.Fatal(err)
	}
	if v.Status != VerificationPassed {
		t.Fatalf("expected passed, got %s (%s)", v.Status, v.ErrorSummary)
	}
	if v.EncryptionPassed {
		t.Fatalf("encryption check should not report passed when encryption is disabled")
	}
	var details struct {
		Checks []Check `json:"checks"`
	}
	if err := json.Unmarshal(v.Details, &details); err != nil {
		t.Fatal(err)
	}
	byKey := map[string]Check{}
	for _, c := range details.Checks {
		byKey[c.Key] = c
	}
	if byKey["encryption"].Status != CheckSkipped {
		t.Fatalf("expected encryption check skipped, got %+v", byKey["encryption"])
	}
	for _, key := range []string{"checksum", "manifest", "pg_restore_list"} {
		if byKey[key].Status != CheckPassed {
			t.Fatalf("expected %s passed, got %+v", key, byKey[key])
		}
	}
}

func TestVerifyFailsOnChecksumMismatch(t *testing.T) {
	svc := newTestService(t)
	row := seedCompletedBackup(t, svc, false)
	var artifact Artifact
	if err := svc.DB.Where("backup_id = ?", row.BackupID).First(&artifact).Error; err != nil {
		t.Fatal(err)
	}
	artifact.SHA256 = "deadbeef"
	if err := svc.DB.Save(&artifact).Error; err != nil {
		t.Fatal(err)
	}
	v, err := svc.Verify(context.Background(), "bk_test_001")
	if err != nil {
		t.Fatal(err)
	}
	if v.Status != VerificationFailed {
		t.Fatalf("expected failed, got %s", v.Status)
	}
}

func TestDownloadRequiresVerifiedCompletedBackup(t *testing.T) {
	svc := newTestService(t)
	seedCompletedBackup(t, svc, false)
	if _, _, err := svc.Download(context.Background(), "bk_test_001"); err == nil {
		t.Fatalf("expected rejection before verification passed")
	}
	if _, err := svc.Verify(context.Background(), "bk_test_001"); err != nil {
		t.Fatal(err)
	}
	job, artifact, err := svc.Download(context.Background(), "bk_test_001")
	if err != nil {
		t.Fatal(err)
	}
	if job.BackupID != "bk_test_001" || artifact.LocalPath == "" {
		t.Fatalf("unexpected download result: %+v %+v", job, artifact)
	}
}

func TestDownloadUnknownBackupReturnsNotFound(t *testing.T) {
	svc := newTestService(t)
	if _, _, err := svc.Download(context.Background(), "bk_missing"); err == nil {
		t.Fatalf("expected error for unknown backup")
	}
}
