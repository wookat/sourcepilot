package database

import (
	"fmt"

	"gorm.io/gorm"
)

// migrateRound72AIBatchTenant backfills ai_operation_batches.tenant_id from
// each batch creator's admin user (the column itself is added by AutoMigrate
// on aioperationbatch.AIOperationBatch). Batches whose tenant cannot be
// derived (created_by NULL or creator row gone) stay at tenant 0 — the legacy
// single-tenant bucket, matching the round71 creator-derivation semantics —
// and never gain visibility in non-zero tenants. Adds the tenant list index.
func migrateRound72AIBatchTenant(db *gorm.DB) error {
	if db == nil {
		return fmt.Errorf("migrate round72: db is nil")
	}
	backfill := `UPDATE ai_operation_batches b SET tenant_id = u.tenant_id
		FROM admin_users u
		WHERE b.created_by = u.id AND u.tenant_id <> 0
		AND (b.tenant_id IS NULL OR b.tenant_id = 0)`
	if err := db.Exec(backfill).Error; err != nil {
		// SQLite dev may not support UPDATE FROM; skip non-fatal
		// (ensureBatchVisible falls back to creator derivation for
		// tenant-0 rows with a creator).
		warnMigrateSkipped("migrateRound72AIBatchTenant", err)
	}
	if !db.Migrator().HasTable("ai_operation_batches") {
		return nil
	}
	idx := `CREATE INDEX IF NOT EXISTS idx_ai_op_batches_tenant_created ON ai_operation_batches (tenant_id, created_at)`
	if err := db.Exec(idx).Error; err != nil {
		return fmt.Errorf("round72 index idx_ai_op_batches_tenant_created: %w", err)
	}
	return nil
}
