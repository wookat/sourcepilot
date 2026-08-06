package backup

import (
	"context"
	"fmt"
	"path"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/trademind-ai/trademind/backend/internal/config"
	"github.com/trademind-ai/trademind/backend/internal/pkg/backupruntime"
	"github.com/trademind-ai/trademind/backend/internal/providers/backupstore"
)

// uploadArtifact pushes a completed backup artifact to the configured object
// storage. Upload failures never fail the backup job itself: the job stays
// completed with uploadStatus=failed so operators can retry.
func (s *Service) uploadArtifact(ctx context.Context, row *Job, artifact *Artifact) {
	now := time.Now().UTC()
	// Upload state must be persisted even when the request context is already
	// cancelled (e.g. the HTTP request timed out while the upload hung).
	saveCtx := context.WithoutCancel(ctx)
	if s.Store == nil {
		row.UploadStatus = UploadSkipped
		_ = s.DB.WithContext(saveCtx).Save(row).Error
		return
	}
	row.UploadTarget = s.Store.Target()
	key := s.objectKey(artifact.Name)
	var lastErr error
	for attempt := 1; attempt <= s.uploadMaxAttempts(); attempt++ {
		row.UploadAttempts = attempt
		lastErr = s.Store.Upload(ctx, key, artifact.LocalPath, "application/octet-stream")
		if lastErr == nil {
			break
		}
		if ctx.Err() != nil {
			break
		}
		time.Sleep(time.Duration(attempt) * time.Second)
	}
	if lastErr != nil {
		row.UploadStatus = UploadFailed
		row.UploadError = s.redactUploadError(lastErr.Error())
		_ = s.DB.WithContext(saveCtx).Save(row).Error
		return
	}
	row.UploadStatus = UploadUploaded
	row.UploadError = ""
	row.UploadedAt = &now
	artifact.ObjectKey = key
	_ = s.DB.WithContext(saveCtx).Save(artifact).Error
	_ = s.DB.WithContext(saveCtx).Save(row).Error
	if err := s.pruneObjectStore(ctx); err != nil {
		// Retention pruning is best-effort; record but do not fail the upload.
		row.UploadError = "retention prune: " + s.redactUploadError(err.Error())
		_ = s.DB.WithContext(saveCtx).Save(row).Error
	}
}

// redactUploadError removes S3 credential literals from an error message
// before it is persisted or surfaced, then applies the generic redaction
// rules. S3-side errors may echo credentials (e.g. AWSAccessKeyId in XML
// bodies), which the generic rules alone do not cover.
func (s *Service) redactUploadError(msg string) string {
	if s.Cfg != nil {
		for _, secret := range []string{s.Cfg.Backup.S3SecretAccessKey, s.Cfg.Backup.S3AccessKeyID} {
			if secret != "" {
				msg = strings.ReplaceAll(msg, secret, "[redacted]")
			}
		}
	}
	return backupruntime.RedactCommandOutput(msg)
}

// RetryUpload re-uploads the newest artifact of a completed backup.
func (s *Service) RetryUpload(ctx context.Context, backupID string) (*Job, error) {
	if s.Store == nil {
		return nil, fmt.Errorf("BACKUP_UPLOAD_NOT_CONFIGURED: backup object storage is not configured")
	}
	row, artifact, err := s.jobArtifact(ctx, backupID)
	if err != nil {
		return nil, err
	}
	if row.Status != StatusCompleted {
		return nil, fmt.Errorf("only completed backups can be uploaded")
	}
	if err := backupruntime.VerifySHA256File(artifact.LocalPath, artifact.SHA256, 1); err != nil {
		return nil, fmt.Errorf("BACKUP_UPLOAD_ARTIFACT_INVALID: %w", err)
	}
	s.uploadArtifact(ctx, row, artifact)
	if row.UploadStatus != UploadUploaded {
		return row, fmt.Errorf("backup upload failed: %s", row.UploadError)
	}
	return row, nil
}

// pruneObjectStore keeps the newest ObjectRetentionCount objects under the
// backup prefix. Objects belonging to backups under retention hold are kept.
// ObjectRetentionCount=0 disables pruning.
func (s *Service) pruneObjectStore(ctx context.Context) error {
	keep := s.Cfg.Backup.ObjectRetentionCount
	if s.Store == nil || keep <= 0 {
		return nil
	}
	prefix := s.objectPrefix()
	if prefix == "" {
		return fmt.Errorf("retention prune skipped: BACKUP_STORAGE_PREFIX resolves to empty, refusing to enumerate the whole bucket")
	}
	objects, err := s.Store.List(ctx, prefix)
	if err != nil {
		return err
	}
	if len(objects) <= keep {
		return nil
	}
	heldIDs, err := s.heldBackupIDs(ctx)
	if err != nil {
		return err
	}
	sort.Slice(objects, func(i, j int) bool { return objects[i].LastModified.After(objects[j].LastModified) })
	var errs []string
	for _, obj := range objects[keep:] {
		if !isBackupArtifactKey(obj.Key) {
			continue
		}
		if keyMatchesAnyBackup(obj.Key, heldIDs) {
			continue
		}
		if err := s.Store.Delete(ctx, obj.Key); err != nil {
			errs = append(errs, err.Error())
		}
	}
	if len(errs) > 0 {
		return fmt.Errorf("delete pruned objects: %s", strings.Join(errs, "; "))
	}
	return nil
}

// fetchFromObjectStore restores a missing local artifact from object storage
// and verifies its checksum, enabling download/restore after container churn.
func (s *Service) fetchFromObjectStore(ctx context.Context, artifact *Artifact) error {
	if s.Store == nil {
		return fmt.Errorf("backup object storage is not configured")
	}
	if strings.TrimSpace(artifact.ObjectKey) == "" {
		return fmt.Errorf("backup artifact has no object storage copy")
	}
	if err := s.ensureUnderWorkRoot(artifact.LocalPath); err != nil {
		return err
	}
	if err := s.Store.Download(ctx, artifact.ObjectKey, artifact.LocalPath); err != nil {
		return err
	}
	return backupruntime.VerifySHA256File(artifact.LocalPath, artifact.SHA256, 1)
}

func (s *Service) heldBackupIDs(ctx context.Context) ([]string, error) {
	var ids []string
	if err := s.DB.WithContext(ctx).Model(&RetentionHold{}).Distinct("backup_id").Pluck("backup_id", &ids).Error; err != nil {
		return nil, err
	}
	return ids, nil
}

// isBackupArtifactKey reports whether an object key looks like a backup
// artifact produced by this service (bk_*.dump / bk_*.dump.enc). Retention
// pruning must never delete unrelated objects sharing the prefix.
func isBackupArtifactKey(key string) bool {
	base := path.Base(strings.ReplaceAll(key, "\\", "/"))
	if !strings.HasPrefix(base, "bk_") {
		return false
	}
	return strings.HasSuffix(base, ".dump") || strings.HasSuffix(base, ".dump.enc")
}

// ensureUnderWorkRoot rejects artifact local paths outside the backup work
// directory so object-store retrieval can never write elsewhere on disk.
func (s *Service) ensureUnderWorkRoot(localPath string) error {
	root, err := filepath.Abs(s.workRoot())
	if err != nil {
		return err
	}
	abs, err := filepath.Abs(localPath)
	if err != nil {
		return err
	}
	rel, err := filepath.Rel(root, abs)
	if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return fmt.Errorf("backup artifact local path escapes the backup work directory")
	}
	return nil
}

func keyMatchesAnyBackup(key string, backupIDs []string) bool {
	for _, id := range backupIDs {
		if id != "" && strings.Contains(key, id) {
			return true
		}
	}
	return false
}

func (s *Service) objectPrefix() string {
	return strings.Trim(s.Cfg.Backup.StoragePrefix, "/")
}

func (s *Service) objectKey(name string) string {
	prefix := s.objectPrefix()
	if prefix == "" {
		return name
	}
	return prefix + "/" + name
}

func (s *Service) uploadMaxAttempts() int {
	if s.Cfg.Backup.UploadMaxAttempts > 0 {
		return s.Cfg.Backup.UploadMaxAttempts
	}
	return 3
}

// NewStore builds the backup object store from config. It returns (nil, nil)
// when uploads are not configured (local-only degraded path).
func NewStore(cfg *config.Config) (backupstore.Store, error) {
	if cfg == nil {
		return nil, nil
	}
	return backupstore.New(backupstore.Config{
		Provider:        cfg.Backup.StorageProvider,
		Endpoint:        cfg.Backup.S3Endpoint,
		Region:          cfg.Backup.S3Region,
		Bucket:          cfg.Backup.StorageBucket,
		Prefix:          cfg.Backup.StoragePrefix,
		AccessKeyID:     cfg.Backup.S3AccessKeyID,
		SecretAccessKey: cfg.Backup.S3SecretAccessKey,
		UsePathStyle:    cfg.Backup.S3UsePathStyle,
	})
}
