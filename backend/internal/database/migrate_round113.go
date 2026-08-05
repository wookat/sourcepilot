package database

import (
	"fmt"

	"gorm.io/gorm"
)

// migrateRound113DefaultWarehouseUnique enforces at the database level that
// each tenant has at most one default warehouse. The round112 backfill and
// EnsureDefaultWarehouse are application-level idempotent (count-then-create),
// so two concurrent requests for a tenant that has no default warehouse yet
// could both pass the existence check and create two defaults. A partial
// unique index makes the second insert fail, and EnsureDefaultWarehouse
// already re-reads on insert failure, so callers converge on a single row.
//
// Preflight: duplicates that already exist from the race window are resolved
// deterministically before the index is created — the oldest default per
// tenant is kept, the rest are demoted to disabled non-default warehouses
// (default-warehouse stock is derived, never persisted, so demoted rows hold
// no stock; their id stays valid for historical change-log references).
func migrateRound113DefaultWarehouseUnique(db *gorm.DB) error {
	if db == nil {
		return fmt.Errorf("migrate round113: db is nil")
	}
	if !db.Migrator().HasTable("warehouses") {
		return nil
	}
	if db.Migrator().HasIndex("warehouses", "idx_warehouses_tenant_default") {
		return nil
	}
	demote := `
UPDATE warehouses SET
  is_default = FALSE,
  enabled = FALSE,
  code = code || '-dup-' || SUBSTR(REPLACE(id, '-', ''), 1, 8)
WHERE is_default = TRUE AND deleted_at IS NULL
  AND EXISTS (
    SELECT 1 FROM warehouses k
    WHERE k.tenant_id = warehouses.tenant_id
      AND k.is_default = TRUE AND k.deleted_at IS NULL
      AND (k.created_at < warehouses.created_at
           OR (k.created_at = warehouses.created_at AND k.id < warehouses.id))
  )`
	if err := db.Exec(demote).Error; err != nil {
		return fmt.Errorf("round113 demote duplicate default warehouses: %w", err)
	}
	create := `CREATE UNIQUE INDEX IF NOT EXISTS idx_warehouses_tenant_default
		ON warehouses (tenant_id) WHERE is_default = TRUE AND deleted_at IS NULL`
	if err := db.Exec(create).Error; err != nil {
		return fmt.Errorf("round113 index idx_warehouses_tenant_default: %w", err)
	}
	return nil
}
