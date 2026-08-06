package restore

import (
	"context"
	"strings"
	"testing"

	"github.com/glebarez/sqlite"
	"github.com/trademind-ai/trademind/backend/internal/config"
	"github.com/trademind-ai/trademind/backend/internal/modules/backup"
	"gorm.io/gorm"
)

func newProductionGateService(t *testing.T, allowProduction bool) *Service {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Skipf("sqlite unavailable: %v", err)
	}
	if err := db.AutoMigrate(&backup.Job{}, &Job{}, &Validation{}); err != nil {
		t.Fatal(err)
	}
	cfg := &config.Config{AppEnv: config.EnvProduction}
	cfg.Backup.RestoreAllowProduction = allowProduction
	return &Service{DB: db, Cfg: cfg}
}

func drillRequest() CreateRequest {
	return CreateRequest{
		BackupID: "bk_gate", TargetEnvironment: "isolated",
		TargetDatabaseName: "trademind_p6v_restore_gate", TargetIsIsolated: true,
		OperatorReauthenticated: true, HighRiskConfirmed: true,
	}
}

func TestProductionRestoreForbiddenWithoutExplicitSwitch(t *testing.T) {
	svc := newProductionGateService(t, false)
	row, err := svc.Create(context.Background(), drillRequest(), nil)
	if err == nil || !strings.Contains(err.Error(), "RESTORE_APP_ENV_FORBIDDEN") {
		t.Fatalf("expected RESTORE_APP_ENV_FORBIDDEN, got %v", err)
	}
	if row == nil || row.Status != StatusRejected || row.SafetyGateStatus != "failed" {
		t.Fatalf("rejected restore must be persisted with failed gate: %+v", row)
	}
}

func TestProductionRestoreSwitchAllowsIsolatedDrill(t *testing.T) {
	svc := newProductionGateService(t, true)
	// With the switch on, the production gate no longer rejects; the next
	// gate (backup existence/verification) takes over, proving the gate
	// passed without weakening the remaining safeguards.
	_, err := svc.Create(context.Background(), drillRequest(), nil)
	if err == nil {
		t.Fatal("expected backup lookup failure, not success")
	}
	if strings.Contains(err.Error(), "RESTORE_APP_ENV_FORBIDDEN") {
		t.Fatalf("production switch must lift the app-env gate, got %v", err)
	}
}

func TestProductionRestoreStillForbidsProductionTarget(t *testing.T) {
	svc := newProductionGateService(t, true)
	req := drillRequest()
	req.TargetEnvironment = "production"
	_, err := svc.Create(context.Background(), req, nil)
	if err == nil || !strings.Contains(err.Error(), "RESTORE_TARGET_FORBIDDEN") {
		t.Fatalf("restore into production target must stay forbidden, got %v", err)
	}
}

func TestProductionRestoreStillRequiresConfirmations(t *testing.T) {
	svc := newProductionGateService(t, true)
	req := drillRequest()
	req.HighRiskConfirmed = false
	_, err := svc.Create(context.Background(), req, nil)
	if err == nil || !strings.Contains(err.Error(), "RESTORE_CONFIRMATION_REQUIRED") {
		t.Fatalf("confirmations must stay required, got %v", err)
	}
	req = drillRequest()
	req.TargetDatabaseName = "prod_main"
	_, err = svc.Create(context.Background(), req, nil)
	if err == nil || !strings.Contains(err.Error(), "RESTORE_TARGET_PREFIX_REJECTED") {
		t.Fatalf("isolated target prefix must stay required, got %v", err)
	}
}

func TestProductionRestoreVerifyGate(t *testing.T) {
	svc := newProductionGateService(t, false)
	row := &Job{RestoreID: "rs_prodgate", BackupID: "bk_gate", TargetEnvironment: "isolated", Status: StatusCompleted, SafetyGateStatus: "passed"}
	if err := svc.DB.Create(row).Error; err != nil {
		t.Fatal(err)
	}
	if _, err := svc.Verify(context.Background(), "rs_prodgate"); err == nil || !strings.Contains(err.Error(), "RESTORE_VERIFY_APP_ENV_FORBIDDEN") {
		t.Fatalf("verify must stay forbidden without the switch, got %v", err)
	}
}
