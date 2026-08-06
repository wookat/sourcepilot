package restore

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/glebarez/sqlite"
	"github.com/trademind-ai/trademind/backend/internal/config"
	"github.com/trademind-ai/trademind/backend/internal/modules/backup"
	"github.com/trademind-ai/trademind/backend/internal/pkg/backupruntime"
	"github.com/trademind-ai/trademind/backend/internal/providers/backupstore"
	"gorm.io/datatypes"
	"gorm.io/gorm"
)

// fakeObjectStore is an in-memory backupstore.Store for the restore-path
// object storage retrieval coverage (#290).
type fakeObjectStore struct {
	objects   map[string][]byte
	downloads int
}

func newFakeObjectStore() *fakeObjectStore {
	return &fakeObjectStore{objects: map[string][]byte{}}
}

func (f *fakeObjectStore) Upload(_ context.Context, key, localPath, _ string) error {
	src, err := os.Open(localPath)
	if err != nil {
		return err
	}
	defer func() { _ = src.Close() }()
	data, err := io.ReadAll(src)
	if err != nil {
		return err
	}
	f.objects[key] = data
	return nil
}

func (f *fakeObjectStore) Download(_ context.Context, key, localPath string) error {
	data, ok := f.objects[key]
	if !ok {
		return fmt.Errorf("object %s not found", key)
	}
	if err := os.MkdirAll(filepath.Dir(localPath), 0o700); err != nil {
		return err
	}
	f.downloads++
	return os.WriteFile(localPath, data, 0o600)
}

func (f *fakeObjectStore) List(_ context.Context, prefix string) ([]backupstore.Object, error) {
	out := make([]backupstore.Object, 0, len(f.objects))
	for k, v := range f.objects {
		if prefix == "" || len(k) >= len(prefix) && k[:len(prefix)] == prefix {
			out = append(out, backupstore.Object{Key: k, Size: int64(len(v))})
		}
	}
	return out, nil
}

func (f *fakeObjectStore) Delete(_ context.Context, key string) error {
	delete(f.objects, key)
	return nil
}

func (f *fakeObjectStore) Target() string { return "fake://bucket/prefix" }

// newObjectFetchFixture seeds a verified backup whose artifact exists only in
// the fake object store (local copy removed), mirroring a container rebuild.
func newObjectFetchFixture(t *testing.T) (*Service, *fakeObjectStore, *backup.Artifact) {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Skipf("sqlite unavailable: %v", err)
	}
	if err := db.AutoMigrate(&backup.Job{}, &backup.Artifact{}, &backup.Verification{}, &Job{}, &Validation{}); err != nil {
		t.Fatal(err)
	}
	workRoot := t.TempDir()
	dir := filepath.Join(workRoot, "bk_objfetch")
	if err := os.MkdirAll(dir, 0o700); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(dir, "bk_objfetch.dump")
	if err := os.WriteFile(path, []byte("fake backup artifact payload"), 0o600); err != nil {
		t.Fatal(err)
	}
	sum, size, err := backupruntime.SHA256File(path)
	if err != nil {
		t.Fatal(err)
	}
	now := time.Now().UTC()
	bk := &backup.Job{
		BackupID: "bk_objfetch", BackupType: backup.TypePostgresLogical, Environment: "test",
		Status: backup.StatusCompleted, VerificationStatus: backup.VerificationPassed,
		StorageProvider: "local", Checksum: sum, ArtifactSize: size, CompletedAt: &now,
		ManifestJSON: datatypes.JSON([]byte(`{}`)),
	}
	if err := db.Create(bk).Error; err != nil {
		t.Fatal(err)
	}
	objectKey := "backups/test/bk_objfetch.dump"
	artifact := &backup.Artifact{
		BackupID: "bk_objfetch", Name: "bk_objfetch.dump", Size: size, SHA256: sum,
		StorageProvider: "local", LocalPath: path, ObjectKey: objectKey,
	}
	if err := db.Create(artifact).Error; err != nil {
		t.Fatal(err)
	}
	store := newFakeObjectStore()
	if err := store.Upload(context.Background(), objectKey, path, "application/octet-stream"); err != nil {
		t.Fatal(err)
	}
	// Simulate the container rebuild: the local artifact is gone.
	if err := os.Remove(path); err != nil {
		t.Fatal(err)
	}
	cfg := &config.Config{AppEnv: config.EnvDevelopment}
	cfg.Backup.CommandTimeoutSeconds = 30
	cfg.PostgresBackup.PGRestorePath = "true" // stand-in binary that always succeeds
	backupSvc := &backup.Service{DB: db, Cfg: cfg, Store: store, WorkRoot: workRoot}
	svc := &Service{DB: db, Cfg: cfg, Backup: backupSvc}
	return svc, store, artifact
}

func TestRestoreFetchesMissingArtifactFromObjectStore(t *testing.T) {
	svc, store, artifact := newObjectFetchFixture(t)
	row := &Job{RestoreID: "rs_objfetch", BackupID: "bk_objfetch", TargetEnvironment: "isolated", Status: StatusCreated, SafetyGateStatus: "passed"}
	if err := svc.DB.Create(row).Error; err != nil {
		t.Fatal(err)
	}
	if err := svc.runPgRestore(context.Background(), row, "trademind_p6v_restore_objfetch"); err != nil {
		t.Fatalf("restore must fetch the artifact from object storage and continue: %v", err)
	}
	if store.downloads != 1 {
		t.Fatalf("expected exactly one object store download, got %d", store.downloads)
	}
	if _, err := os.Stat(artifact.LocalPath); err != nil {
		t.Fatal("artifact must be re-materialized locally for pg_restore")
	}
	var report map[string]any
	if err := json.Unmarshal(row.ReportJSON, &report); err != nil {
		t.Fatal(err)
	}
	if report["pgRestore"] != "passed" {
		t.Fatalf("expected pgRestore passed, got %v", report["pgRestore"])
	}
}

func TestRestoreVerifyFetchesMissingArtifactFromObjectStore(t *testing.T) {
	svc, store, _ := newObjectFetchFixture(t)
	row := &Job{RestoreID: "rs_objverify", BackupID: "bk_objfetch", TargetEnvironment: "isolated", Status: StatusCompleted, SafetyGateStatus: "passed"}
	if err := svc.DB.Create(row).Error; err != nil {
		t.Fatal(err)
	}
	v, err := svc.Verify(context.Background(), "rs_objverify")
	if err != nil {
		t.Fatal(err)
	}
	if v.Status != "passed" {
		t.Fatalf("expected passed, got %q (%s)", v.Status, v.ErrorSummary)
	}
	if store.downloads == 0 {
		t.Fatal("verify must retrieve the missing artifact from object storage")
	}
}

func TestRestoreFailsWhenObjectCopyMissing(t *testing.T) {
	svc, store, artifact := newObjectFetchFixture(t)
	if err := store.Delete(context.Background(), artifact.ObjectKey); err != nil {
		t.Fatal(err)
	}
	row := &Job{RestoreID: "rs_objgone", BackupID: "bk_objfetch", TargetEnvironment: "isolated", Status: StatusCreated, SafetyGateStatus: "passed"}
	if err := svc.DB.Create(row).Error; err != nil {
		t.Fatal(err)
	}
	err := svc.runPgRestore(context.Background(), row, "trademind_p6v_restore_objgone")
	if err == nil {
		t.Fatal("restore must fail when neither local nor object copy exists")
	}
}
