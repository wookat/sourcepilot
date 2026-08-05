package database

import (
	"fmt"

	"gorm.io/gorm"
)

// migrateRound122PerfIndexes adds composite indexes for the round122
// performance audit. The single-column tenant indexes force a sort (or a
// full scan under LIMIT/OFFSET) on the hottest list/report scans once the
// tables reach 万级 rows:
//
//   - orders (tenant_id, payment_status, created_at): profit / finance
//     reconciliation / finance report range scans over paid orders;
//   - order_automation_logs (tenant_id, created_at DESC, id DESC): the
//     execution-log list is always paginated in that order;
//   - inventory_change_logs (tenant_id, created_at DESC): the global
//     inventory ledger feed is always paginated by created_at.
//
// Safe to re-run (IF NOT EXISTS). Rollback: DROP INDEX IF EXISTS <name>.
func migrateRound122PerfIndexes(db *gorm.DB) error {
	if db == nil {
		return fmt.Errorf("migrate round122: db is nil")
	}
	if db.Dialector.Name() != "postgres" {
		return nil
	}
	stmts := []string{
		`CREATE INDEX IF NOT EXISTS idx_orders_tenant_pay_created
		 ON orders (tenant_id, payment_status, created_at DESC)
		 WHERE deleted_at IS NULL`,
		`CREATE INDEX IF NOT EXISTS idx_order_automation_logs_tenant_created
		 ON order_automation_logs (tenant_id, created_at DESC, id DESC)`,
		`CREATE INDEX IF NOT EXISTS idx_inventory_change_logs_tenant_created
		 ON inventory_change_logs (tenant_id, created_at DESC)`,
	}
	for _, sql := range stmts {
		if err := db.Exec(sql).Error; err != nil {
			return fmt.Errorf("round122 index: %w", err)
		}
	}
	return nil
}
