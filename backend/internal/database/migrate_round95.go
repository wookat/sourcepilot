package database

import (
	"fmt"

	"gorm.io/gorm"
)

// migrateRound95OrderNoTenantUnique replaces the global unique index on
// orders.order_no with a per-tenant one. A globally unique order number let a
// tenant probe (and squat) another tenant's order numbers: creating or
// importing an order whose number already exists elsewhere failed with a
// duplicate-key error, which is an existence oracle plus a cross-tenant
// denial-of-service on migration imports. Uniqueness is a per-tenant business
// rule, so the index must carry tenant_id.
func migrateRound95OrderNoTenantUnique(db *gorm.DB) error {
	if db == nil {
		return fmt.Errorf("migrate round95: db is nil")
	}
	if !db.Migrator().HasTable("orders") {
		return nil
	}
	create := `CREATE UNIQUE INDEX IF NOT EXISTS idx_orders_tenant_order_no
		ON orders (tenant_id, order_no)`
	if err := db.Exec(create).Error; err != nil {
		return fmt.Errorf("round95 index idx_orders_tenant_order_no: %w", err)
	}
	if err := db.Exec(`DROP INDEX IF EXISTS idx_orders_order_no`).Error; err != nil {
		return fmt.Errorf("round95 drop idx_orders_order_no: %w", err)
	}
	return nil
}
