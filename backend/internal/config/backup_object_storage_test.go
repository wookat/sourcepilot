package config

import (
	"strings"
	"testing"
)

func baseBackupCfg() BackupConfig {
	return BackupConfig{
		Mode: "local", StorageProvider: "local",
		RetentionDaily: 7, CommandTimeoutSeconds: 900, UploadMaxAttempts: 3,
	}
}

func TestValidateBackupObjectStorage(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name    string
		mutate  func(*BackupConfig)
		wantErr bool
	}{
		{"unset is valid degraded path", func(b *BackupConfig) {}, false},
		{"complete config valid", func(b *BackupConfig) {
			b.S3Endpoint = "http://minio:9000"
			b.StorageBucket = "trademind-backups"
			b.S3AccessKeyID = "ak"
			b.S3SecretAccessKey = "sk"
		}, false},
		{"missing secret key", func(b *BackupConfig) {
			b.StorageBucket = "trademind-backups"
			b.S3AccessKeyID = "ak"
		}, true},
		{"missing access key", func(b *BackupConfig) {
			b.StorageBucket = "trademind-backups"
			b.S3SecretAccessKey = "sk"
		}, true},
		{"endpoint without credentials", func(b *BackupConfig) {
			b.S3Endpoint = "http://minio:9000"
		}, true},
		{"credentials without bucket", func(b *BackupConfig) {
			b.S3AccessKeyID = "ak"
			b.S3SecretAccessKey = "sk"
		}, true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			b := baseBackupCfg()
			tc.mutate(&b)
			err := validateBackupObjectStorage(b)
			if tc.wantErr && err == nil {
				t.Fatal("expected validation error")
			}
			if !tc.wantErr && err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
		})
	}
}

func TestBackupS3EnvParsing(t *testing.T) {
	t.Setenv("BACKUP_S3_ENDPOINT", "http://127.0.0.1:9000")
	t.Setenv("BACKUP_S3_ACCESS_KEY_ID", "ak")
	t.Setenv("BACKUP_S3_SECRET_ACCESS_KEY", "sk")
	t.Setenv("BACKUP_UPLOAD_MAX_ATTEMPTS", "5")
	t.Setenv("BACKUP_OBJECT_RETENTION_COUNT", "9")

	b := loadBackupConfig(EnvDevelopment)
	if b.S3Endpoint != "http://127.0.0.1:9000" || b.S3AccessKeyID != "ak" || b.S3SecretAccessKey != "sk" {
		t.Fatal("s3 settings not parsed")
	}
	if !b.S3UsePathStyle {
		t.Fatal("path style should default to true when a custom endpoint is set")
	}
	if b.S3Region != "us-east-1" {
		t.Fatalf("unexpected default region %q", b.S3Region)
	}
	if b.UploadMaxAttempts != 5 || b.ObjectRetentionCount != 9 {
		t.Fatalf("unexpected attempts/retention: %d/%d", b.UploadMaxAttempts, b.ObjectRetentionCount)
	}
}

func TestBackupUploadGuardValues(t *testing.T) {
	t.Parallel()
	c := &Config{AppEnv: EnvDevelopment}
	c.Backup = baseBackupCfg()
	c.Backup.UploadMaxAttempts = -1
	if err := c.validateP6ProductionGuards(); err == nil || !strings.Contains(err.Error(), "BACKUP_UPLOAD_MAX_ATTEMPTS") {
		t.Fatalf("expected attempts validation error, got %v", err)
	}
	c.Backup = baseBackupCfg()
	c.Backup.ObjectRetentionCount = -2
	if err := c.validateP6ProductionGuards(); err == nil || !strings.Contains(err.Error(), "BACKUP_OBJECT_RETENTION_COUNT") {
		t.Fatalf("expected retention validation error, got %v", err)
	}
}
