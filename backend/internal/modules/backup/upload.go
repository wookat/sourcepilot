package backup

import (
	"context"
	"fmt"
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
	if s.Store == nil {
		row.UploadStatus = UploadSkipped
		_ = s.DB.WithContext(ctx).Save(row).Error
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
		row.UploadError = backupruntime.RedactCommandOutput(lastErr.Error())
		_ = s.DB.WithContext(ctx).Save(row).Error
		return
	}
	row.UploadStatus = UploadUploaded
	row.UploadError = ""
	row.UploadedAt = &now
	artifact.ObjectKey = key
	_ = s.DB.WithContext(ctx).Save(artifact).Error
	_ = s.DB.WithContext(ctx).Save(row).Error
	if err := s.pruneObjectStore(ctx); err != nil {
		// Retention pruning is best-effort; record but do not fail the upload.
		row.UploadError = "retention prune: " + backupruntime.RedactCommandOutput(err.Error())
		_ = s.DB.WithContext(ctx).Save(row).Error
	}
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
	objects, err := s.Store.List(ctx, s.objectPrefix())
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
