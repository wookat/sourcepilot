package integration

import (
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/trademind-ai/trademind/backend/internal/database"
	"github.com/trademind-ai/trademind/backend/internal/testing/safeenv"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

// Regression (upgrade drill after R95): when legacy rows contain per-tenant
// duplicate order numbers, AutoMigrate must fail with an actionable preflight
// error (naming the duplicate tenant/order_no groups and the cleanup guide)
// instead of a bare unique-index SQLSTATE 23505, and must succeed after the
// duplicates are cleaned up.
func TestOrderNoTenantUniquePreflight(t *testing.T) {
	t.Setenv("APP_ENV", "test")
	cfg, ok, err := safeenv.TestDatabaseURLFromEnv()
	require.NoError(t, err)
	if !ok {
		t.Skip("TEST_DATABASE_URL is not set; skipping PostgreSQL integration migration test")
	}

	db, err := gorm.Open(postgres.Open(cfg.URL), &gorm.Config{})
	require.NoError(t, err)
	sqlDB, err := db.DB()
	require.NoError(t, err)
	t.Cleanup(func() { sqlDB.Close() })

	require.NoError(t, database.AutoMigrate(db))

	cleanup := func() {
		db.Exec("DELETE FROM orders WHERE order_no LIKE 'r95-preflight-%'")
	}
	cleanup()
	t.Cleanup(cleanup)

	// Simulate a drifted legacy schema: the per-tenant unique index is absent
	// and one tenant holds a duplicate order number.
	require.NoError(t, db.Exec("DROP INDEX IF EXISTS idx_orders_tenant_order_no").Error)
	insert := `INSERT INTO orders (id, tenant_id, platform, order_no, customer_name, status,
		payment_status, fulfillment_status, currency, total_amount, created_at, updated_at)
		VALUES (gen_random_uuid(), 7001, 'manual', 'r95-preflight-dup', ?, 'pending',
		'unpaid', 'unfulfilled', 'CNY', 1, now(), now())`
	require.NoError(t, db.Exec(insert, "客户甲").Error)
	require.NoError(t, db.Exec(insert, "客户乙").Error)

	err = database.AutoMigrate(db)
	require.Error(t, err)
	require.Contains(t, err.Error(), "round95 preflight")
	require.Contains(t, err.Error(), "r95-preflight-dup")
	require.Contains(t, err.Error(), "tenant_id=7001")
	require.Contains(t, err.Error(), "docs/upgrade-guide.md")

	// The failed run must not have created the index or touched the rows.
	require.False(t, db.Migrator().HasIndex("orders", "idx_orders_tenant_order_no"))
	var cnt int64
	require.NoError(t, db.Raw("SELECT COUNT(*) FROM orders WHERE order_no = 'r95-preflight-dup'").Scan(&cnt).Error)
	require.EqualValues(t, 2, cnt)

	// After cleanup (keep the oldest row per group) the migration succeeds.
	require.NoError(t, db.Exec(`WITH dup AS (
		SELECT id, ROW_NUMBER() OVER (PARTITION BY tenant_id, order_no ORDER BY created_at, id) rn
		FROM orders WHERE deleted_at IS NULL AND order_no = 'r95-preflight-dup')
		DELETE FROM orders WHERE id IN (SELECT id FROM dup WHERE rn > 1)`).Error)
	require.NoError(t, database.AutoMigrate(db))
	require.True(t, db.Migrator().HasIndex("orders", "idx_orders_tenant_order_no"))
}
