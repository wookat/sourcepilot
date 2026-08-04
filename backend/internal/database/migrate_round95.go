package database

import (
	"fmt"

	"gorm.io/gorm"
)

// orderNoDuplicate is one (tenant_id, order_no) group that would break the
// per-tenant unique index.
type orderNoDuplicate struct {
	TenantID int64
	OrderNo  string
	Count    int64
}

// preflightOrderNoTenantUnique fails fast with an actionable message when
// existing rows would break the idx_orders_tenant_order_no unique index.
// Without this check the index is created inside GORM AutoMigrate (from the
// model tag) and the upgrade aborts with a bare `could not create unique
// index ... SQLSTATE 23505` that names neither the offending rows nor the
// cleanup procedure.
func preflightOrderNoTenantUnique(db *gorm.DB) error {
	if db == nil || !db.Migrator().HasTable("orders") {
		return nil
	}
	if db.Migrator().HasIndex("orders", "idx_orders_tenant_order_no") {
		return nil
	}
	var dups []orderNoDuplicate
	err := db.Raw(`SELECT tenant_id, order_no, COUNT(*) AS count
		FROM orders WHERE deleted_at IS NULL
		GROUP BY tenant_id, order_no HAVING COUNT(*) > 1
		ORDER BY tenant_id, order_no LIMIT 20`).Scan(&dups).Error
	if err != nil {
		// Pre-check is best-effort; unqueryable legacy schemas fall through to
		// the index creation itself.
		return nil
	}
	if len(dups) == 0 {
		return nil
	}
	list := ""
	for _, d := range dups {
		list += fmt.Sprintf(" (tenant_id=%d order_no=%q x%d)", d.TenantID, d.OrderNo, d.Count)
	}
	return fmt.Errorf(
		"round95 preflight: orders 表存在同租户重复订单号，无法创建唯一索引 idx_orders_tenant_order_no。"+
			"请先按 docs/upgrade-guide.md 清理重复订单号后重启（升级不会修改任何数据）。重复项（最多列 20 组）:%s", list)
}

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
