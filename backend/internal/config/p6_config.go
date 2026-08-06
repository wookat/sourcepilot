package config

import (
	"crypto/x509"
	"fmt"
	"net"
	"net/url"
	"os"
	"strings"

	"github.com/trademind-ai/trademind/backend/internal/pkg/backupsched"
)

// BackupConfig holds P6 backup, verification and retention settings.
type BackupConfig struct {
	Enabled               bool
	Mode                  string
	Schedule              string
	ScheduleEnabled       bool
	StorageProvider       string
	StorageBucket         string
	StoragePrefix         string
	EncryptionEnabled     bool
	EncryptionKeyID       string
	RetentionDaily        int
	RetentionWeekly       int
	RetentionMonthly      int
	MaxAgeHours           int
	CommandTimeoutSeconds int
	VerifyEnabled         bool
	RestoreDrillEnabled   bool
	RestoreDrillSchedule  string
	// RestoreAllowProduction gates restore drill endpoints when AppEnv is
	// production. Off by default: restores stay forbidden in production
	// unless explicitly opted in.
	RestoreAllowProduction bool

	// S3-compatible object storage upload for backup artifacts (R138).
	S3Endpoint        string
	S3Region          string
	S3AccessKeyID     string
	S3SecretAccessKey string
	S3UsePathStyle    bool
	// S3CABundle is a PEM file path with extra CA certificates trusted for
	// the backup S3 endpoint (self-signed MinIO drills). Empty uses the
	// system trust store only.
	S3CABundle           string
	UploadMaxAttempts    int
	ObjectRetentionCount int
}

// PostgresBackupConfig holds PostgreSQL backup/PITR command settings.
type PostgresBackupConfig struct {
	Format            string
	PGDumpPath        string
	PGRestorePath     string
	PSQLPath          string
	WALArchiveEnabled bool
	WALArchivePath    string
	PITREnabled       bool
}

// ReleaseConfig holds P6 controlled deployment settings.
type ReleaseConfig struct {
	Enabled              bool
	Root                 string
	ArtifactDir          string
	CurrentLink          string
	PreviousLink         string
	KeepCount            int
	HealthTimeoutSeconds int
	RollbackOnFailure    bool
	RequirePreBackup     bool
	Strategy             string
	TrafficSwitchMode    string
}

func loadBackupConfig(appEnv string) BackupConfig {
	return BackupConfig{
		Enabled:                envBool(os.Getenv("BACKUP_ENABLED"), false),
		Mode:                   strings.ToLower(strings.TrimSpace(firstNonEmpty(os.Getenv("BACKUP_MODE"), "disabled"))),
		Schedule:               strings.TrimSpace(firstNonEmpty(os.Getenv("BACKUP_SCHEDULE"), "0 3 * * *")),
		ScheduleEnabled:        envBool(os.Getenv("BACKUP_SCHEDULE_ENABLED"), false),
		StorageProvider:        strings.ToLower(strings.TrimSpace(firstNonEmpty(os.Getenv("BACKUP_STORAGE_PROVIDER"), "local"))),
		StorageBucket:          strings.TrimSpace(os.Getenv("BACKUP_STORAGE_BUCKET")),
		StoragePrefix:          strings.Trim(strings.TrimSpace(firstNonEmpty(os.Getenv("BACKUP_STORAGE_PREFIX"), "backups/"+NormalizeEnv(appEnv))), "/"),
		EncryptionEnabled:      envBool(os.Getenv("BACKUP_ENCRYPTION_ENABLED"), IsStagingOrProduction(appEnv)),
		EncryptionKeyID:        strings.TrimSpace(firstNonEmpty(os.Getenv("BACKUP_ENCRYPTION_KEY_ID"), "app-master-key")),
		RetentionDaily:         atoiOrDefault(os.Getenv("BACKUP_RETENTION_DAILY"), 7),
		RetentionWeekly:        atoiOrDefault(os.Getenv("BACKUP_RETENTION_WEEKLY"), 4),
		RetentionMonthly:       atoiOrDefault(os.Getenv("BACKUP_RETENTION_MONTHLY"), 6),
		MaxAgeHours:            atoiOrDefault(os.Getenv("BACKUP_MAX_AGE_HOURS"), 30),
		CommandTimeoutSeconds:  atoiOrDefault(os.Getenv("BACKUP_COMMAND_TIMEOUT_SECONDS"), 900),
		VerifyEnabled:          envBool(os.Getenv("BACKUP_VERIFY_ENABLED"), true),
		RestoreDrillEnabled:    envBool(os.Getenv("BACKUP_RESTORE_DRILL_ENABLED"), false),
		RestoreDrillSchedule:   strings.TrimSpace(firstNonEmpty(os.Getenv("BACKUP_RESTORE_DRILL_SCHEDULE"), "0 4 * * 0")),
		RestoreAllowProduction: envBool(os.Getenv("BACKUP_RESTORE_ALLOW_PRODUCTION"), false),
		S3Endpoint:             strings.TrimSpace(os.Getenv("BACKUP_S3_ENDPOINT")),
		S3Region:               strings.TrimSpace(firstNonEmpty(os.Getenv("BACKUP_S3_REGION"), "us-east-1")),
		S3AccessKeyID:          strings.TrimSpace(os.Getenv("BACKUP_S3_ACCESS_KEY_ID")),
		S3SecretAccessKey:      strings.TrimSpace(os.Getenv("BACKUP_S3_SECRET_ACCESS_KEY")),
		S3UsePathStyle:         envBool(os.Getenv("BACKUP_S3_USE_PATH_STYLE"), strings.TrimSpace(os.Getenv("BACKUP_S3_ENDPOINT")) != ""),
		S3CABundle:             strings.TrimSpace(os.Getenv("BACKUP_S3_CA_BUNDLE")),
		UploadMaxAttempts:      atoiOrDefault(os.Getenv("BACKUP_UPLOAD_MAX_ATTEMPTS"), 3),
		ObjectRetentionCount:   atoiOrDefault(os.Getenv("BACKUP_OBJECT_RETENTION_COUNT"), 14),
	}
}

func loadPostgresBackupConfig() PostgresBackupConfig {
	return PostgresBackupConfig{
		Format:            strings.ToLower(strings.TrimSpace(firstNonEmpty(os.Getenv("POSTGRES_BACKUP_FORMAT"), "custom"))),
		PGDumpPath:        strings.TrimSpace(firstNonEmpty(os.Getenv("POSTGRES_PG_DUMP_PATH"), "pg_dump")),
		PGRestorePath:     strings.TrimSpace(firstNonEmpty(os.Getenv("POSTGRES_PG_RESTORE_PATH"), "pg_restore")),
		PSQLPath:          strings.TrimSpace(firstNonEmpty(os.Getenv("POSTGRES_PSQL_PATH"), "psql")),
		WALArchiveEnabled: envBool(os.Getenv("POSTGRES_WAL_ARCHIVE_ENABLED"), false),
		WALArchivePath:    strings.TrimSpace(os.Getenv("POSTGRES_WAL_ARCHIVE_PATH")),
		PITREnabled:       envBool(os.Getenv("POSTGRES_PITR_ENABLED"), false),
	}
}

func loadReleaseConfig(appEnv string) ReleaseConfig {
	root := strings.TrimSpace(firstNonEmpty(os.Getenv("RELEASE_ROOT"), "/opt/trademind/releases"))
	return ReleaseConfig{
		Enabled:              envBool(os.Getenv("RELEASE_ENABLED"), false),
		Root:                 root,
		ArtifactDir:          strings.TrimSpace(firstNonEmpty(os.Getenv("RELEASE_ARTIFACT_DIR"), "artifacts/releases")),
		CurrentLink:          strings.TrimSpace(firstNonEmpty(os.Getenv("RELEASE_CURRENT_LINK"), root+"/current")),
		PreviousLink:         strings.TrimSpace(firstNonEmpty(os.Getenv("RELEASE_PREVIOUS_LINK"), root+"/previous")),
		KeepCount:            atoiOrDefault(os.Getenv("RELEASE_KEEP_COUNT"), 5),
		HealthTimeoutSeconds: atoiOrDefault(os.Getenv("RELEASE_HEALTH_TIMEOUT_SECONDS"), 120),
		RollbackOnFailure:    envBool(os.Getenv("RELEASE_ROLLBACK_ON_FAILURE"), true),
		RequirePreBackup:     envBool(os.Getenv("RELEASE_REQUIRE_PRE_BACKUP"), IsStagingOrProduction(appEnv)),
		Strategy:             strings.ToLower(strings.TrimSpace(firstNonEmpty(os.Getenv("RELEASE_STRATEGY"), "blue_green"))),
		TrafficSwitchMode:    strings.ToLower(strings.TrimSpace(firstNonEmpty(os.Getenv("RELEASE_TRAFFIC_SWITCH_MODE"), "manual"))),
	}
}

func (c *Config) validateP6ProductionGuards() error {
	if c == nil {
		return nil
	}
	if strings.TrimSpace(c.Backup.Mode) == "" {
		c.Backup.Mode = "disabled"
	}
	if strings.TrimSpace(c.Backup.StorageProvider) == "" {
		c.Backup.StorageProvider = "local"
	}
	if strings.TrimSpace(c.Release.Strategy) == "" {
		c.Release.Strategy = "blue_green"
	}
	if c.Backup.CommandTimeoutSeconds == 0 {
		c.Backup.CommandTimeoutSeconds = 900
	}
	if c.Backup.UploadMaxAttempts == 0 {
		c.Backup.UploadMaxAttempts = 3
	}
	if !validBackupMode(c.Backup.Mode) {
		return fmt.Errorf("%s: BACKUP_MODE must be disabled, local, object_storage, or hybrid", ErrCodeConfigInvalid)
	}
	if !validReleaseStrategy(c.Release.Strategy) {
		return fmt.Errorf("%s: RELEASE_STRATEGY must be in_place, rolling, or blue_green", ErrCodeConfigInvalid)
	}
	if c.Backup.CommandTimeoutSeconds <= 0 {
		return fmt.Errorf("%s: BACKUP_COMMAND_TIMEOUT_SECONDS must be positive", ErrCodeConfigInvalid)
	}
	if c.Backup.RetentionDaily < 0 || c.Backup.RetentionWeekly < 0 || c.Backup.RetentionMonthly < 0 {
		return fmt.Errorf("%s: backup retention counts cannot be negative", ErrCodeConfigInvalid)
	}
	if c.Backup.Enabled && c.Backup.RetentionDaily == 0 && c.Backup.RetentionWeekly == 0 && c.Backup.RetentionMonthly == 0 {
		return fmt.Errorf("%s: backup retention cannot be unlimited or empty", ErrCodeConfigInvalid)
	}
	if c.Backup.UploadMaxAttempts <= 0 {
		return fmt.Errorf("%s: BACKUP_UPLOAD_MAX_ATTEMPTS must be positive", ErrCodeConfigInvalid)
	}
	if c.Backup.ObjectRetentionCount < 0 {
		return fmt.Errorf("%s: BACKUP_OBJECT_RETENTION_COUNT cannot be negative", ErrCodeConfigInvalid)
	}
	if err := validateBackupObjectStorage(c.Backup); err != nil {
		return err
	}
	if err := validateBackupSchedule(c.Backup); err != nil {
		return err
	}
	if err := validateBackupS3CABundle(c.Backup.S3CABundle); err != nil {
		return err
	}
	if err := validateBackupS3Endpoint(c.Backup.S3Endpoint, c.AppEnv); err != nil {
		return err
	}
	if IsProduction(c.AppEnv) {
		if !c.Backup.Enabled {
			return fmt.Errorf("%s: BACKUP_ENABLED=true is required in production", ErrCodeConfigRequired)
		}
		if c.Backup.Mode == "disabled" || c.Backup.Mode == "local" {
			return fmt.Errorf("%s: production backups require object_storage or hybrid mode", ErrCodeConfigInvalid)
		}
		if !c.Backup.EncryptionEnabled {
			return fmt.Errorf("%s: BACKUP_ENCRYPTION_ENABLED=true is required in production", ErrCodeConfigRequired)
		}
		if c.Release.Enabled && !c.Release.RequirePreBackup {
			return fmt.Errorf("%s: RELEASE_REQUIRE_PRE_BACKUP=true is required for production release", ErrCodeConfigRequired)
		}
	}
	return nil
}

// validateBackupSchedule rejects an unparseable BACKUP_SCHEDULE when the
// built-in scheduler is enabled. Without the scheduler the expression stays
// informational metadata (host crontab path) and is not validated.
func validateBackupSchedule(b BackupConfig) error {
	if !b.ScheduleEnabled {
		return nil
	}
	if !b.Enabled || b.Mode == "disabled" {
		return fmt.Errorf("%s: BACKUP_SCHEDULE_ENABLED=true requires BACKUP_ENABLED=true and BACKUP_MODE other than disabled", ErrCodeConfigInvalid)
	}
	if _, err := backupsched.Parse(b.Schedule); err != nil {
		return fmt.Errorf("%s: BACKUP_SCHEDULE invalid: %v", ErrCodeConfigInvalid, err)
	}
	return nil
}

// validateBackupS3CABundle fails startup fast when a configured CA bundle
// path is missing or contains no usable certificate, instead of silently
// degrading uploads to the local-only path.
func validateBackupS3CABundle(path string) error {
	if path == "" {
		return nil
	}
	pem, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("%s: BACKUP_S3_CA_BUNDLE unreadable: %v", ErrCodeConfigInvalid, err)
	}
	pool := x509.NewCertPool()
	if !pool.AppendCertsFromPEM(pem) {
		return fmt.Errorf("%s: BACKUP_S3_CA_BUNDLE contains no valid PEM certificate", ErrCodeConfigInvalid)
	}
	return nil
}

// validateBackupObjectStorage rejects half-configured S3 upload settings.
// Fully unset settings are valid: uploads degrade to the local-only path.
func validateBackupObjectStorage(b BackupConfig) error {
	hasCreds := b.S3AccessKeyID != "" || b.S3SecretAccessKey != ""
	if !hasCreds && b.S3Endpoint == "" {
		return nil
	}
	if b.S3AccessKeyID == "" || b.S3SecretAccessKey == "" {
		return fmt.Errorf("%s: BACKUP_S3_ACCESS_KEY_ID and BACKUP_S3_SECRET_ACCESS_KEY must both be set for backup object storage upload", ErrCodeConfigInvalid)
	}
	if strings.TrimSpace(b.StorageBucket) == "" {
		return fmt.Errorf("%s: BACKUP_STORAGE_BUCKET is required when backup object storage credentials are configured", ErrCodeConfigInvalid)
	}
	return nil
}

// validateBackupS3Endpoint rejects malformed endpoints everywhere and, in
// production, plaintext HTTP plus loopback/link-local (metadata) targets.
func validateBackupS3Endpoint(endpoint, appEnv string) error {
	if endpoint == "" {
		return nil
	}
	u, err := url.Parse(endpoint)
	if err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Hostname() == "" {
		return fmt.Errorf("%s: BACKUP_S3_ENDPOINT must be a valid http(s) URL", ErrCodeConfigInvalid)
	}
	if !IsProduction(appEnv) {
		return nil
	}
	if u.Scheme != "https" {
		return fmt.Errorf("%s: BACKUP_S3_ENDPOINT must use https in production", ErrCodeConfigInvalid)
	}
	host := u.Hostname()
	if strings.EqualFold(host, "localhost") {
		return fmt.Errorf("%s: BACKUP_S3_ENDPOINT cannot target localhost in production", ErrCodeConfigInvalid)
	}
	if ip := net.ParseIP(host); ip != nil && (ip.IsLoopback() || ip.IsLinkLocalUnicast() || ip.IsLinkLocalMulticast() || ip.IsUnspecified()) {
		return fmt.Errorf("%s: BACKUP_S3_ENDPOINT cannot target loopback or link-local addresses in production", ErrCodeConfigInvalid)
	}
	return nil
}

func validBackupMode(v string) bool {
	switch strings.ToLower(strings.TrimSpace(v)) {
	case "disabled", "local", "object_storage", "hybrid":
		return true
	default:
		return false
	}
}

func validReleaseStrategy(v string) bool {
	switch strings.ToLower(strings.TrimSpace(v)) {
	case "in_place", "rolling", "blue_green":
		return true
	default:
		return false
	}
}
