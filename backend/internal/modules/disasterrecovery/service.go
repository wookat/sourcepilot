package disasterrecovery

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/trademind-ai/trademind/backend/internal/config"
	"github.com/trademind-ai/trademind/backend/internal/modules/backup"
	"github.com/trademind-ai/trademind/backend/internal/pkg/backupruntime"
	"gorm.io/datatypes"
	"gorm.io/gorm"
)

type Service struct {
	DB     *gorm.DB
	Cfg    *config.Config
	Backup *backup.Service
}

func (s *Service) Status(ctx context.Context) (map[string]any, error) {
	var last Drill
	status := "deferred"
	if err := s.DB.WithContext(ctx).Order("created_at DESC").First(&last).Error; err == nil {
		status = last.Status
	}
	return map[string]any{
		"status":                           status,
		"rpoTarget":                        "draft",
		"rtoTarget":                        "draft",
		"realProductionDRVerification":     "deferred",
		"realProductionBackupVerification": "deferred",
		"realPITRDrill":                    "deferred",
		"lastDrill":                        last,
	}, nil
}

// CreateDrill runs a DR drill limited to local/development environments. The
// backup file integrity and pg_restore --list structure checks are real; the
// remaining drill items are reported as not implemented instead of fake passes.
func (s *Service) CreateDrill(ctx context.Context, req DrillRequest, actor *uuid.UUID) (*Drill, error) {
	if !req.ConfirmedIsolated {
		return nil, fmt.Errorf("DR drill must be confirmed isolated")
	}
	if s.Cfg != nil && config.IsProduction(s.Cfg.AppEnv) {
		return nil, fmt.Errorf("DR_DRILL_APP_ENV_FORBIDDEN: DR drills are limited to local/development environments")
	}
	backupID := strings.TrimSpace(req.BackupID)
	if backupID == "" {
		return nil, fmt.Errorf("DR_DRILL_BACKUP_REQUIRED: drill requires a verified backupId")
	}
	if s.Backup == nil {
		return nil, fmt.Errorf("DR drill unavailable: backup service missing")
	}
	typ := strings.TrimSpace(req.DrillType)
	if typ == "" {
		typ = "isolated_restore"
	}
	started := time.Now().UTC()
	status := "passed"
	summary := ""
	checks := make([]backup.Check, 0, 5)
	if err := s.Backup.ArtifactIntegrityCheck(ctx, backupID); err != nil {
		status = "failed"
		summary = backupruntime.RedactCommandOutput(err.Error())
		checks = append(checks, backup.Check{Key: "backup_file_integrity", Status: backup.CheckFailed, Message: summary})
		checks = append(checks, backup.Check{Key: "pg_restore_list", Status: backup.CheckSkipped, Message: "integrity check failed"})
	} else {
		checks = append(checks, backup.Check{Key: "backup_file_integrity", Status: backup.CheckPassed})
		if err := s.Backup.ArtifactStructureCheck(ctx, backupID); err != nil {
			status = "failed"
			summary = backupruntime.RedactCommandOutput(err.Error())
			checks = append(checks, backup.Check{Key: "pg_restore_list", Status: backup.CheckFailed, Message: summary})
		} else {
			checks = append(checks, backup.Check{Key: "pg_restore_list", Status: backup.CheckPassed})
		}
	}
	for _, key := range []string{"rpo_measurement", "rto_measurement", "application_failover"} {
		checks = append(checks, backup.Check{Key: key, Status: backup.CheckNotImplemented})
	}
	now := time.Now().UTC()
	row := &Drill{
		DrillID:          "dr_" + strings.ReplaceAll(uuid.NewString(), "-", ""),
		Environment:      s.Cfg.AppEnv,
		DrillType:        typ,
		Status:           status,
		BackupID:         backupID,
		RestoreID:        strings.TrimSpace(req.RestoreID),
		ReleaseID:        strings.TrimSpace(req.ReleaseID),
		RPOSecondsTarget: 3600,
		RTOSecondsTarget: 7200,
		StartedAt:        started,
		CompletedAt:      &now,
		ErrorSummary:     summary,
		CreatedBy:        actor,
	}
	if raw, err := json.Marshal(map[string]any{"checks": checks}); err == nil {
		row.ReportJSON = datatypes.JSON(raw)
	}
	return row, s.DB.WithContext(ctx).Create(row).Error
}
