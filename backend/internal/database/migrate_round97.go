package database

import (
	"gorm.io/gorm"
)

// migrateRound97ReportCurrencyTenant copies the legacy tenant-0
// report_currency settings (base currency / manual rate table) to every
// existing tenant that has no report_currency configuration of its own.
// Before round 97 the settings page wrote the group at tenant 0 and reports
// fell back to it, so all tenants effectively shared one table; after the
// per-tenant isolation each tenant reads only its own rows, and this backfill
// keeps existing tenants' report conversion unchanged. Tenant 0 keeps its
// rows (it stays a valid tenant for legacy single-tenant / demo data). New
// tenants start unconfigured and use the default base currency (CNY, empty
// rate table).
func migrateRound97ReportCurrencyTenant(db *gorm.DB) error {
	if db == nil || !db.Migrator().HasTable("settings") || !db.Migrator().HasTable("tenants") {
		return nil
	}
	sql := `INSERT INTO settings (tenant_id, group_key, item_key, item_value, value_type, is_encrypted, remark, created_at, updated_at)
		SELECT t.id, s.group_key, s.item_key, s.item_value, s.value_type, s.is_encrypted, s.remark, NOW(), NOW()
		FROM tenants t
		JOIN settings s ON s.tenant_id = 0 AND s.group_key = 'report_currency'
		WHERE t.id <> 0 AND t.deleted_at IS NULL
		AND NOT EXISTS (
			SELECT 1 FROM settings s2
			WHERE s2.tenant_id = t.id AND s2.group_key = 'report_currency'
		)`
	if err := db.Exec(sql).Error; err != nil {
		// SQLite dev databases may lack NOW(); non-fatal — tenants without a
		// copied table fall back to the default base currency.
		warnMigrateSkipped("migrateRound97ReportCurrencyTenant", err)
	}
	return nil
}
