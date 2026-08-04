package integration

import (
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/trademind-ai/trademind/backend/internal/database"
	"github.com/trademind-ai/trademind/backend/internal/modules/platformtenant"
	"github.com/trademind-ai/trademind/backend/internal/modules/settings"
	"github.com/trademind-ai/trademind/backend/internal/providers/fxrate"
	"github.com/trademind-ai/trademind/backend/internal/testing/safeenv"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

// Regression (R97): the legacy tenant-0 report_currency configuration is
// copied to existing tenants without their own configuration, tenants that
// already configured the group are left untouched, and the backfill is
// idempotent.
func TestRound97ReportCurrencyBackfill(t *testing.T) {
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
		db.Exec("DELETE FROM settings WHERE group_key = ?", fxrate.SettingsGroup)
		db.Exec("DELETE FROM tenants WHERE name IN (?, ?)", "r97-copy", "r97-own")
	}
	cleanup()
	t.Cleanup(cleanup)

	tCopy := &platformtenant.Tenant{Name: "r97-copy", Status: "active"}
	tOwn := &platformtenant.Tenant{Name: "r97-own", Status: "active"}
	require.NoError(t, db.Create(tCopy).Error)
	require.NoError(t, db.Create(tOwn).Error)

	seed := []settings.Setting{
		{TenantID: 0, GroupKey: fxrate.SettingsGroup, ItemKey: fxrate.KeyProvider, ItemValue: fxrate.ProviderManual, ValueType: "string"},
		{TenantID: 0, GroupKey: fxrate.SettingsGroup, ItemKey: fxrate.KeyBaseCurrency, ItemValue: "CNY", ValueType: "string"},
		{TenantID: 0, GroupKey: fxrate.SettingsGroup, ItemKey: fxrate.KeyRates, ItemValue: `{"USD":"7.20"}`, ValueType: "string"},
		{TenantID: tOwn.ID, GroupKey: fxrate.SettingsGroup, ItemKey: fxrate.KeyBaseCurrency, ItemValue: "USD", ValueType: "string"},
	}
	for i := range seed {
		require.NoError(t, db.Create(&seed[i]).Error)
	}

	// AutoMigrate runs the round97 backfill; run twice to prove idempotency.
	require.NoError(t, database.AutoMigrate(db))
	require.NoError(t, database.AutoMigrate(db))

	var copied []settings.Setting
	require.NoError(t, db.Where("tenant_id = ? AND group_key = ?", tCopy.ID, fxrate.SettingsGroup).
		Order("item_key").Find(&copied).Error)
	require.Len(t, copied, 3)
	byKey := map[string]string{}
	for _, r := range copied {
		byKey[r.ItemKey] = r.ItemValue
	}
	require.Equal(t, "CNY", byKey[fxrate.KeyBaseCurrency])
	require.Equal(t, `{"USD":"7.20"}`, byKey[fxrate.KeyRates])

	var ownRows []settings.Setting
	require.NoError(t, db.Where("tenant_id = ? AND group_key = ?", tOwn.ID, fxrate.SettingsGroup).
		Find(&ownRows).Error)
	require.Len(t, ownRows, 1)
	require.Equal(t, "USD", ownRows[0].ItemValue)

	var zeroRows int64
	require.NoError(t, db.Model(&settings.Setting{}).
		Where("tenant_id = 0 AND group_key = ?", fxrate.SettingsGroup).Count(&zeroRows).Error)
	require.EqualValues(t, 3, zeroRows)
}
