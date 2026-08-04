package database

import (
	"fmt"

	"gorm.io/gorm"
)

// migrateRound81PublishBatchTenant backfills product_publish_batches.tenant_id
// from each batch creator's admin user (the column itself is added by
// AutoMigrate on productpublish.ProductPublishBatch), mirroring the round72
// ai_operation_batches treatment. Batches whose tenant cannot be derived
// (created_by NULL or creator row gone) stay at tenant 0 — the legacy
// single-tenant bucket — and never gain visibility in non-zero tenants.
// Adds the tenant list index.
func migrateRound81PublishBatchTenant(db *gorm.DB) error {
	if db == nil {
		return fmt.Errorf("migrate round81: db is nil")
	}
	if !db.Migrator().HasTable("product_publish_batches") {
		return nil
	}
	backfill := `UPDATE product_publish_batches b SET tenant_id = u.tenant_id
		FROM admin_users u
		WHERE b.created_by = u.id AND u.tenant_id <> 0
		AND (b.tenant_id IS NULL OR b.tenant_id = 0)`
	if err := db.Exec(backfill).Error; err != nil {
		// SQLite dev may not support UPDATE FROM; skip non-fatal
		// (ensureBatchVisible falls back to creator derivation for
		// tenant-0 rows with a creator).
		warnMigrateSkipped("migrateRound81PublishBatchTenant", err)
	}
	idx := `CREATE INDEX IF NOT EXISTS idx_publish_batches_tenant_created ON product_publish_batches (tenant_id, created_at)`
	if err := db.Exec(idx).Error; err != nil {
		return fmt.Errorf("round81 index idx_publish_batches_tenant_created: %w", err)
	}
	return nil
}
