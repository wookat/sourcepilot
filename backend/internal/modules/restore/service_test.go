package restore

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/glebarez/sqlite"
	"github.com/trademind-ai/trademind/backend/internal/config"
	"github.com/trademind-ai/trademind/backend/internal/modules/backup"
	"github.com/trademind-ai/trademind/backend/internal/pkg/backupruntime"
	"gorm.io/datatypes"
	"gorm.io/gorm"
)

func TestRestoreSafetyGateRejectsUnverifiedBackup(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Skipf("sqlite unavailable: %v", err)
	}
	if err := db.AutoMigrate(&backup.Job{}, &Job{}, &Validation{}); err != nil {
		t.Fatal(err)
	}
	if err := db.Create(&backup.Job{BackupID: "bk_test", BackupType: backup.TypePostgresLogical, Environment: "test", Status: backup.StatusCompleted, VerificationStatus: backup.VerificationPending, StorageProvider: "local", Checksum: "abc"}).Error; err != nil {
		t.Fatal(err)
	}
	svc := &Service{DB: db, Cfg: &config.Config{AppEnv: config.EnvDevelopment}}
	_, err = svc.Create(context.Background(), CreateRequest{
		BackupID: "bk_test", TargetEnvironment: "isolated", TargetDatabaseName: "restore_db",
		TargetIsIsolated: true, OperatorReauthenticated: true, HighRiskConfirmed: true,
	}, nil)
	if err == nil {
		t.Fatalf("expected unverified backup rejection")
	}
}

func newVerifyFixture(t *testing.T) (*Service, string) {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Skipf("sqlite unavailable: %v", err)
	}
	if err := db.AutoMigrate(&backup.Job{}, &backup.Artifact{}, &Job{}, &Validation{}); err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	path := filepath.Join(dir, "bk_verify.dump")
	if err := os.WriteFile(path, []byte("fake backup artifact payload"), 0o600); err != nil {
		t.Fatal(err)
	}
	sum, size, err := backupruntime.SHA256File(path)
	if err != nil {
		t.Fatal(err)
	}
	now := time.Now().UTC()
	if err := db.Create(&backup.Job{
		BackupID: "bk_verify", BackupType: backup.TypePostgresLogical, Environment: "test",
		Status: backup.StatusCompleted, VerificationStatus: backup.VerificationPassed,
		StorageProvider: "local", Checksum: sum, ArtifactSize: size, CompletedAt: &now,
		ManifestJSON: datatypes.JSON([]byte(`{}`)),
	}).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Create(&backup.Artifact{
		BackupID: "bk_verify", Name: "bk_verify.dump", Size: size, SHA256: sum,
		StorageProvider: "local", LocalPath: path,
	}).Error; err != nil {
		t.Fatal(err)
	}
	cfg := &config.Config{AppEnv: config.EnvDevelopment}
	cfg.Backup.CommandTimeoutSeconds = 30
	cfg.PostgresBackup.PGRestorePath = "true" // stand-in binary that always succeeds
	backupSvc := &backup.Service{DB: db, Cfg: cfg}
	svc := &Service{DB: db, Cfg: cfg, Backup: backupSvc}
	row := &Job{RestoreID: "rs_verify", BackupID: "bk_verify", TargetEnvironment: "isolated", Status: StatusCompleted, SafetyGateStatus: "passed"}
	if err := db.Create(row).Error; err != nil {
		t.Fatal(err)
	}
	return svc, row.RestoreID
}

func TestRestoreVerifyRunsRealChecksAndMarksOthersNotImplemented(t *testing.T) {
	svc, restoreID := newVerifyFixture(t)
	v, err := svc.Verify(context.Background(), restoreID)
	if err != nil {
		t.Fatal(err)
	}
	if v.Status != "passed" {
		t.Fatalf("expected passed, got %s (%s)", v.Status, v.ErrorSummary)
	}
	if v.MigrationVersionChecked || v.TenantIsolationChecked || v.RBACChecked || v.AuditChainChecked || v.ObjectInventoryChecked || v.SecretCiphertextChecked {
		t.Fatalf("unimplemented checks must not report as checked: %+v", v)
	}
	var details struct {
		Checks []backup.Check `json:"checks"`
	}
	if err := json.Unmarshal(v.Details, &details); err != nil {
		t.Fatal(err)
	}
	byKey := map[string]backup.Check{}
	for _, c := range details.Checks {
		byKey[c.Key] = c
	}
	if byKey["backup_file_integrity"].Status != backup.CheckPassed {
		t.Fatalf("expected backup_file_integrity passed, got %+v", byKey["backup_file_integrity"])
	}
	if byKey["pg_restore_list"].Status != backup.CheckPassed {
		t.Fatalf("expected pg_restore_list passed, got %+v", byKey["pg_restore_list"])
	}
	for _, key := range []string{"migration_version", "tenant_isolation", "rbac", "audit_chain", "object_inventory", "secret_ciphertext"} {
		if byKey[key].Status != backup.CheckNotImplemented {
			t.Fatalf("expected %s not_implemented, got %+v", key, byKey[key])
		}
	}
}

func TestRestoreVerifyFailsOnTamperedArtifact(t *testing.T) {
	svc, restoreID := newVerifyFixture(t)
	var artifact backup.Artifact
	if err := svc.DB.Where("backup_id = ?", "bk_verify").First(&artifact).Error; err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(artifact.LocalPath, []byte("tampered payload content!"), 0o600); err != nil {
		t.Fatal(err)
	}
	v, err := svc.Verify(context.Background(), restoreID)
	if err != nil {
		t.Fatal(err)
	}
	if v.Status != "failed" {
		t.Fatalf("expected failed on tampered artifact, got %s", v.Status)
	}
}

func TestRestoreVerifyForbiddenInProduction(t *testing.T) {
	svc, restoreID := newVerifyFixture(t)
	svc.Cfg.AppEnv = config.EnvProduction
	if _, err := svc.Verify(context.Background(), restoreID); err == nil {
		t.Fatalf("expected production restriction error")
	}
}
