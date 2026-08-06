package config

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"math/big"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestValidateBackupSchedule(t *testing.T) {
	t.Parallel()
	b := baseBackupCfg()
	b.Schedule = "not a cron at all here"
	if err := validateBackupSchedule(b); err != nil {
		t.Fatalf("schedule is metadata-only when scheduler disabled: %v", err)
	}
	b.ScheduleEnabled = true
	b.Enabled = true
	if err := validateBackupSchedule(b); err == nil {
		t.Fatal("invalid schedule must fail when scheduler enabled")
	}
	b.Schedule = "0 3 * * *"
	if err := validateBackupSchedule(b); err != nil {
		t.Fatalf("valid cron rejected: %v", err)
	}
	b.Schedule = "@every 6h"
	if err := validateBackupSchedule(b); err != nil {
		t.Fatalf("valid interval rejected: %v", err)
	}
	b.Enabled = false
	if err := validateBackupSchedule(b); err == nil {
		t.Fatal("scheduler requires BACKUP_ENABLED=true")
	}
	b.Enabled = true
	b.Mode = "disabled"
	if err := validateBackupSchedule(b); err == nil {
		t.Fatal("scheduler requires a non-disabled backup mode")
	}
}

// selfSignedCAPEM generates a throwaway self-signed CA certificate for
// pool-append checks (test-only, never trusted anywhere).
func selfSignedCAPEM(t *testing.T) []byte {
	t.Helper()
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	tmpl := &x509.Certificate{
		SerialNumber:          big.NewInt(1),
		Subject:               pkix.Name{CommonName: "trademind-test-ca"},
		NotBefore:             time.Now().Add(-time.Hour),
		NotAfter:              time.Now().Add(time.Hour),
		IsCA:                  true,
		BasicConstraintsValid: true,
	}
	der, err := x509.CreateCertificate(rand.Reader, tmpl, tmpl, &key.PublicKey, key)
	if err != nil {
		t.Fatal(err)
	}
	return pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der})
}

func TestValidateBackupS3CABundle(t *testing.T) {
	t.Parallel()
	if err := validateBackupS3CABundle(""); err != nil {
		t.Fatalf("empty CA bundle must be valid: %v", err)
	}
	if err := validateBackupS3CABundle(filepath.Join(t.TempDir(), "missing.pem")); err == nil {
		t.Fatal("missing CA bundle file must fail startup")
	}
	junk := filepath.Join(t.TempDir(), "junk.pem")
	if err := os.WriteFile(junk, []byte("not a certificate"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := validateBackupS3CABundle(junk); err == nil {
		t.Fatal("non-PEM CA bundle must fail startup")
	}
	valid := filepath.Join(t.TempDir(), "ca.pem")
	if err := os.WriteFile(valid, selfSignedCAPEM(t), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := validateBackupS3CABundle(valid); err != nil {
		t.Fatalf("valid PEM CA bundle rejected: %v", err)
	}
}

func TestBackupScheduleAndCAEnvParsing(t *testing.T) {
	t.Setenv("BACKUP_SCHEDULE_ENABLED", "true")
	t.Setenv("BACKUP_RESTORE_ALLOW_PRODUCTION", "true")
	t.Setenv("BACKUP_S3_CA_BUNDLE", "/etc/trademind/backup-s3-ca.pem")
	b := loadBackupConfig(EnvDevelopment)
	if !b.ScheduleEnabled || !b.RestoreAllowProduction || b.S3CABundle != "/etc/trademind/backup-s3-ca.pem" {
		t.Fatalf("env parsing mismatch: %+v", b)
	}
}
