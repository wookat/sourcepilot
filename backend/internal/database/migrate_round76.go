package database

import (
	"gorm.io/gorm"
)

// migrateRound76PublishTaskTenant backfills product_publish_tasks.tenant_id
// from the owning product (tasks were historically persisted with tenant_id 0
// while ListTasks is tenant-scoped, so batch sub-tasks stayed invisible in
// non-zero tenants). Rows whose product tenant is 0 or whose product row is
// gone stay at tenant 0, the legacy single-tenant bucket.
func migrateRound76PublishTaskTenant(db *gorm.DB) error {
	if db == nil || !db.Migrator().HasTable("product_publish_tasks") {
		return nil
	}
	backfill := `UPDATE product_publish_tasks t SET tenant_id = p.tenant_id
		FROM products p
		WHERE t.product_id = p.id AND p.tenant_id <> 0
		AND (t.tenant_id IS NULL OR t.tenant_id = 0)`
	if err := db.Exec(backfill).Error; err != nil {
		// SQLite dev may not support UPDATE FROM; non-fatal, new tasks
		// carry the tenant at creation time.
		_ = err
	}
	return nil
}
