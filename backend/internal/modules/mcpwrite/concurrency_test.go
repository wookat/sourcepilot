package mcpwrite_test

import (
	"context"
	"errors"
	"fmt"
	"math"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/trademind-ai/trademind/backend/internal/modules/mcpaudit"
	"github.com/trademind-ai/trademind/backend/internal/modules/mcpwrite"
	"github.com/trademind-ai/trademind/backend/internal/modules/settings"
	"github.com/trademind-ai/trademind/backend/internal/testing/postgrestest"
	"github.com/trademind-ai/trademind/backend/internal/testing/safeenv"
	"gorm.io/gorm"
)

// R182: concurrent executes must not slip past the tenant write quotas. The
// quota reads happen inside the execute transaction (READ COMMITTED on
// PostgreSQL), so without per-tenant serialization two parallel executes can
// both pass the count / amount check and land one write over the ceiling.
// The tenant-scoped advisory lock (plus the in-process per-tenant mutex)
// closes that window.

func openRaceTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	if _, ok, _ := safeenv.TestDatabaseURLFromEnv(); !ok {
		t.Skip("TEST_DATABASE_URL is not set; skipping PostgreSQL quota race regression")
	}
	h := postgrestest.Require(t)
	if err := h.DB.AutoMigrate(&mcpaudit.ToolCallLog{}, &mcpwrite.Confirmation{}, &settings.Setting{}); err != nil {
		t.Fatal(err)
	}
	return h.DB
}

func newRaceService(t *testing.T, db *gorm.DB, tenantID int64) *mcpwrite.Service {
	t.Helper()
	if err := db.Create(&settings.Setting{
		TenantID:  tenantID,
		GroupKey:  mcpwrite.SettingsGroupMCP,
		ItemKey:   mcpwrite.SettingsKeyWriteEnable,
		ItemValue: "true",
	}).Error; err != nil {
		t.Fatal(err)
	}
	return &mcpwrite.Service{
		DB:     db,
		Gate:   &mcpwrite.Gate{EnvEnabled: true, Settings: &settings.Service{DB: db}},
		Audits: &mcpaudit.Service{DB: db},
	}
}

func raceCaller(tenantID int64) mcpwrite.Caller {
	return mcpwrite.Caller{
		TenantID:    tenantID,
		TokenID:     uuid.New(),
		TokenName:   "race",
		TokenMasked: "tmk_****race",
	}
}

// seedExecuteRows backfills n successful execute audit rows so the tenant
// daily count quota is nearly exhausted before the race starts.
func seedExecuteRows(t *testing.T, svc *mcpwrite.Service, tenantID int64, n int) {
	t.Helper()
	for i := 0; i < n; i++ {
		if err := svc.Audits.Write(context.Background(), mcpaudit.WriteOpts{
			TenantID: tenantID,
			TokenID:  uuid.New(),
			Tool:     "orders_add_tag",
			Status:   mcpaudit.StatusSuccess,
			Mode:     mcpaudit.ModeExecute,
		}); err != nil {
			t.Fatal(err)
		}
	}
}

// raceRequest builds one write request whose execute mutation only holds
// the transaction open briefly (so concurrent executes overlap after their
// quota reads); idx keeps the params (and so the confirmation binding)
// distinct per call.
func raceRequest(caller mcpwrite.Caller, idx int, amount float64, execute func(ctx context.Context, tx *gorm.DB) (any, string, error)) mcpwrite.Request {
	if execute == nil {
		execute = func(ctx context.Context, tx *gorm.DB) (any, string, error) {
			time.Sleep(150 * time.Millisecond)
			return nil, "ok", nil
		}
	}
	return mcpwrite.Request{
		Caller:          caller,
		Tool:            "race_probe",
		ParamsCanonical: fmt.Sprintf("idx=%d", idx),
		ParamsSummary:   fmt.Sprintf("idx=%d", idx),
		Amount:          amount,
		DryRun: func(ctx context.Context, db *gorm.DB) (any, string, error) {
			return nil, "preview", nil
		},
		Execute: execute,
	}
}

// runRace confirms every request via dry_run first, then fires all executes
// at once and reports how many succeeded / were quota-rejected.
func runRace(t *testing.T, svc *mcpwrite.Service, reqs []mcpwrite.Request, wantErr error) (succeeded, rejected int) {
	t.Helper()
	confirmations := make([]string, len(reqs))
	for i, req := range reqs {
		req.Mode = mcpwrite.ModeDryRun
		res, err := svc.Run(context.Background(), req)
		if err != nil {
			t.Fatalf("dry_run %d: %v", i, err)
		}
		confirmations[i] = res.ConfirmationToken
	}
	var wg sync.WaitGroup
	start := make(chan struct{})
	errCh := make(chan error, len(reqs))
	for i, req := range reqs {
		wg.Add(1)
		go func(req mcpwrite.Request, conf string) {
			defer wg.Done()
			<-start
			req.Mode = mcpwrite.ModeExecute
			req.ConfirmationToken = conf
			_, err := svc.Run(context.Background(), req)
			errCh <- err
		}(req, confirmations[i])
	}
	close(start)
	wg.Wait()
	close(errCh)
	for err := range errCh {
		switch {
		case err == nil:
			succeeded++
		case errors.Is(err, wantErr):
			rejected++
		default:
			t.Fatalf("unexpected execute error: %v", err)
		}
	}
	return succeeded, rejected
}

// Two executes race for the last slot of the tenant daily count quota:
// exactly one may land, and the audit trail must never exceed the ceiling.
func TestExecuteCountQuotaRacePostgres(t *testing.T) {
	db := openRaceTestDB(t)
	svc := newRaceService(t, db, 1)
	caller := raceCaller(1)
	seedExecuteRows(t, svc, 1, mcpwrite.PerTenantDailyLimit-1)

	const racers = 8
	reqs := make([]mcpwrite.Request, 0, racers)
	for i := 0; i < racers; i++ {
		reqs = append(reqs, raceRequest(caller, i, 0, nil))
	}
	succeeded, rejected := runRace(t, svc, reqs, mcpwrite.ErrQuotaTenant)
	if succeeded != 1 || rejected != racers-1 {
		t.Fatalf("succeeded=%d rejected=%d, want exactly 1/%d", succeeded, rejected, racers-1)
	}
	n, err := svc.Audits.CountExecutesByTenant(db, 1, time.Now().UTC().Add(-24*time.Hour))
	if err != nil {
		t.Fatal(err)
	}
	if n > mcpwrite.PerTenantDailyLimit {
		t.Fatalf("executed rows = %d, tenant daily ceiling %d exceeded under concurrency", n, mcpwrite.PerTenantDailyLimit)
	}
}

// Two tenants execute concurrently: tenant 1 is at its last quota slot and
// must stay hard-capped, while tenant 2's executes all pass — the per-tenant
// locks must not leak across tenants or share quota.
func TestExecuteTenantIsolationRacePostgres(t *testing.T) {
	db := openRaceTestDB(t)
	svc := newRaceService(t, db, 1)
	if err := db.Create(&settings.Setting{
		TenantID:  2,
		GroupKey:  mcpwrite.SettingsGroupMCP,
		ItemKey:   mcpwrite.SettingsKeyWriteEnable,
		ItemValue: "true",
	}).Error; err != nil {
		t.Fatal(err)
	}
	seedExecuteRows(t, svc, 1, mcpwrite.PerTenantDailyLimit-1)

	const racers = 4
	reqs := make([]mcpwrite.Request, 0, 2*racers)
	for i := 0; i < racers; i++ {
		reqs = append(reqs, raceRequest(raceCaller(1), i, 0, nil))
		reqs = append(reqs, raceRequest(raceCaller(2), i, 0, nil))
	}
	succeeded, rejected := runRace(t, svc, reqs, mcpwrite.ErrQuotaTenant)
	if succeeded != 1+racers || rejected != racers-1 {
		t.Fatalf("succeeded=%d rejected=%d, want %d/%d (tenant 1 capped at 1, tenant 2 all pass)",
			succeeded, rejected, 1+racers, racers-1)
	}
	n1, err := svc.Audits.CountExecutesByTenant(db, 1, time.Now().UTC().Add(-24*time.Hour))
	if err != nil {
		t.Fatal(err)
	}
	if n1 > mcpwrite.PerTenantDailyLimit {
		t.Fatalf("tenant 1 executed rows = %d, ceiling %d exceeded", n1, mcpwrite.PerTenantDailyLimit)
	}
	n2, err := svc.Audits.CountExecutesByTenant(db, 2, time.Now().UTC().Add(-24*time.Hour))
	if err != nil {
		t.Fatal(err)
	}
	if n2 != racers {
		t.Fatalf("tenant 2 executed rows = %d, want %d (must not share tenant 1's quota or lock)", n2, racers)
	}
}

// Two amount-bearing executes race for a daily amount ceiling re-checked
// inside the execute transaction (the procurement_mark_paid pattern): only
// one may land, and the summed amounts must stay within the ceiling.
func TestExecuteAmountCeilingRacePostgres(t *testing.T) {
	db := openRaceTestDB(t)
	svc := newRaceService(t, db, 1)
	caller := raceCaller(1)

	const amount, ceiling = 88.0, 150.0
	overDaily := errors.New("amount ceiling exceeded")
	execute := func(ctx context.Context, tx *gorm.DB) (any, string, error) {
		used, err := svc.Audits.SumExecuteAmountByTenantTool(tx, 1, "race_probe", time.Now().UTC().Add(-24*time.Hour))
		if err != nil {
			return nil, "", err
		}
		if used+amount > ceiling {
			return nil, "", overDaily
		}
		time.Sleep(150 * time.Millisecond)
		return nil, "ok", nil
	}
	const racers = 8
	reqs := make([]mcpwrite.Request, 0, racers)
	for i := 0; i < racers; i++ {
		reqs = append(reqs, raceRequest(caller, i, amount, execute))
	}
	succeeded, rejected := runRace(t, svc, reqs, overDaily)
	if succeeded != 1 || rejected != racers-1 {
		t.Fatalf("succeeded=%d rejected=%d, want exactly 1/%d", succeeded, rejected, racers-1)
	}
	total, err := svc.Audits.SumExecuteAmountByTenantTool(db, 1, "race_probe", time.Now().UTC().Add(-24*time.Hour))
	if err != nil {
		t.Fatal(err)
	}
	if math.Round(total*100) > ceiling*100 {
		t.Fatalf("executed amount = %.2f, daily ceiling %.2f exceeded under concurrency", total, ceiling)
	}
}
