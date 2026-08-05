package integration

import (
	"context"
	"sync"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/trademind-ai/trademind/backend/internal/database"
	"github.com/trademind-ai/trademind/backend/internal/modules/inventory"
	"github.com/trademind-ai/trademind/backend/internal/modules/platformtenant"
	"github.com/trademind-ai/trademind/backend/internal/testing/safeenv"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

// Regression (R113): the default-warehouse backfill is application-level
// idempotent (count-then-create), so concurrent callers could create two
// default warehouses for one tenant. The partial unique index
// idx_warehouses_tenant_default must reject the second insert, and
// EnsureDefaultWarehouse must converge every concurrent caller onto the
// single surviving row. Pre-existing duplicates are demoted by the migration.
func TestRound113DefaultWarehouseUnique(t *testing.T) {
	t.Setenv("APP_ENV", "test")
	cfg, ok, err := safeenv.TestDatabaseURLFromEnv()
	require.NoError(t, err)
	if !ok {
		t.Skip("TEST_DATABASE_URL is not set; skipping PostgreSQL integration migration test")
	}

	db, err := gorm.Open(postgres.Open(cfg.URL), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	require.NoError(t, err)
	sqlDB, err := db.DB()
	require.NoError(t, err)
	sqlDB.SetMaxOpenConns(16)
	t.Cleanup(func() { sqlDB.Close() })

	require.NoError(t, database.AutoMigrate(db))

	cleanup := func() {
		db.Exec(`DELETE FROM warehouses WHERE tenant_id IN (SELECT id FROM tenants WHERE name LIKE 'r113-tenant%')`)
		db.Exec(`DELETE FROM tenants WHERE name LIKE 'r113-tenant%'`)
	}
	cleanup()
	t.Cleanup(cleanup)

	countDefaults := func(tenantID int64) int64 {
		var n int64
		require.NoError(t, db.Model(&inventory.Warehouse{}).
			Where("tenant_id = ? AND is_default = ?", tenantID, true).
			Count(&n).Error)
		return n
	}

	t.Run("concurrent EnsureDefaultWarehouse creates exactly one default", func(t *testing.T) {
		tn := &platformtenant.Tenant{Name: "r113-tenant-concurrent", Status: "active"}
		require.NoError(t, db.Create(tn).Error)

		svc := &inventory.Service{DB: db}
		const workers = 12
		ids := make([]string, workers)
		errs := make([]error, workers)
		start := make(chan struct{})
		var wg sync.WaitGroup
		for i := 0; i < workers; i++ {
			wg.Add(1)
			go func(i int) {
				defer wg.Done()
				<-start
				w, err := svc.EnsureDefaultWarehouse(context.Background(), tn.ID)
				errs[i] = err
				if err == nil {
					ids[i] = w.ID.String()
				}
			}(i)
		}
		close(start)
		wg.Wait()

		for i := 0; i < workers; i++ {
			require.NoError(t, errs[i], "worker %d", i)
			require.Equal(t, ids[0], ids[i], "all workers must converge on one default warehouse")
		}
		require.EqualValues(t, 1, countDefaults(tn.ID))
	})

	t.Run("direct duplicate insert is rejected by the partial unique index", func(t *testing.T) {
		tn := &platformtenant.Tenant{Name: "r113-tenant-index", Status: "active"}
		require.NoError(t, db.Create(tn).Error)

		svc := &inventory.Service{DB: db}
		_, err := svc.EnsureDefaultWarehouse(context.Background(), tn.ID)
		require.NoError(t, err)

		dup := inventory.Warehouse{
			TenantID:  tn.ID,
			Code:      "r113-dup",
			Name:      "重复默认仓",
			IsDefault: true,
			Enabled:   true,
		}
		require.Error(t, db.Create(&dup).Error, "second default warehouse for the tenant must violate idx_warehouses_tenant_default")
		require.EqualValues(t, 1, countDefaults(tn.ID))
	})

	t.Run("migration demotes pre-existing duplicates keeping the oldest", func(t *testing.T) {
		tn := &platformtenant.Tenant{Name: "r113-tenant-demote", Status: "active"}
		require.NoError(t, db.Create(tn).Error)

		require.NoError(t, db.Exec(`DROP INDEX IF EXISTS idx_warehouses_tenant_default`).Error)

		first := inventory.Warehouse{TenantID: tn.ID, Code: "default", Name: "默认仓", IsDefault: true, Enabled: true}
		require.NoError(t, db.Create(&first).Error)
		require.NoError(t, db.Exec(
			`INSERT INTO warehouses (id, tenant_id, code, name, is_default, enabled, priority, created_at, updated_at)
			 SELECT gen_random_uuid()::text, tenant_id, code, name, is_default, enabled, priority, created_at + interval '1 second', updated_at
			 FROM warehouses WHERE id = ?`, first.ID).Error, "seed duplicate default warehouse")

		require.EqualValues(t, 2, countDefaults(tn.ID))
		require.NoError(t, database.AutoMigrate(db))

		require.EqualValues(t, 1, countDefaults(tn.ID))
		var kept inventory.Warehouse
		require.NoError(t, db.Where("tenant_id = ? AND is_default = ?", tn.ID, true).First(&kept).Error)
		require.Equal(t, first.ID, kept.ID, "the oldest default warehouse must be kept")

		var demoted inventory.Warehouse
		require.NoError(t, db.Where("tenant_id = ? AND is_default = ? AND code LIKE ?", tn.ID, false, "default-dup-%").
			First(&demoted).Error)
		require.False(t, demoted.Enabled, "demoted duplicate must be disabled")
	})
}
