package database

import (
	"gorm.io/gorm"
)

// migrateRound105AlertTenant backfills task_alerts.tenant_id from each
// alert's source task row (directly for tenant-columned task tables, via the
// owning product/batch/shop otherwise). Before round 105 the alert scan ran
// with a platform context and every alert row landed at tenant 0, so the
// round-104 read-side tenant filter hid business tenants' alerts from them.
// Alerts whose source row is gone (or has no tenant linkage) stay at tenant 0
// — the legacy platform bucket — and never gain visibility in non-zero
// tenants. Mirrors taskcenter.(*Service).resolveSourceTenant.
func migrateRound105AlertTenant(db *gorm.DB) error {
	if db == nil || !db.Migrator().HasTable("task_alerts") {
		return nil
	}
	direct := map[string]string{
		"collect":               "collect_tasks",
		"order_sync":            "order_sync_tasks",
		"customer_message_sync": "customer_message_sync_tasks",
		"product_publish":       "product_publish_tasks",
		"inventory_sync":        "inventory_sync_tasks",
	}
	for taskType, table := range direct {
		if !db.Migrator().HasTable(table) {
			continue
		}
		sql := `UPDATE task_alerts a SET tenant_id = t.tenant_id
			FROM ` + table + ` t
			WHERE a.task_type = '` + taskType + `'
			AND a.source_id = CAST(t.id AS TEXT)
			AND t.tenant_id <> 0 AND a.tenant_id = 0`
		if err := db.Exec(sql).Error; err != nil {
			// SQLite dev may not support UPDATE FROM; non-fatal —
			// unresolved rows stay in the tenant-0 bucket.
			warnMigrateSkipped("migrateRound105AlertTenant:"+taskType, err)
		}
	}
	via := []struct {
		taskType string
		sql      string
	}{
		{"image", `UPDATE task_alerts a SET tenant_id = p.tenant_id
			FROM image_tasks t JOIN products p ON p.id = t.product_id
			WHERE a.task_type = 'image' AND a.source_id = CAST(t.id AS TEXT)
			AND p.tenant_id <> 0 AND a.tenant_id = 0`},
		{"ai_text", `UPDATE task_alerts a SET tenant_id = b.tenant_id
			FROM ai_product_text_items i JOIN ai_product_text_batches b ON b.id = i.batch_id
			WHERE a.task_type = 'ai_text' AND a.source_id = CAST(i.id AS TEXT)
			AND b.tenant_id <> 0 AND a.tenant_id = 0`},
		{"ai_image", `UPDATE task_alerts a SET tenant_id = b.tenant_id
			FROM ai_product_image_items i JOIN ai_product_image_batches b ON b.id = i.batch_id
			WHERE a.task_type = 'ai_image' AND a.source_id = CAST(i.id AS TEXT)
			AND b.tenant_id <> 0 AND a.tenant_id = 0`},
		{"customer_failure", `UPDATE task_alerts a SET tenant_id = sh.tenant_id
			FROM customer_failure_events e JOIN shops sh ON sh.id = e.shop_id
			WHERE a.task_type = 'customer_failure' AND a.source_id = CAST(e.id AS TEXT)
			AND sh.tenant_id <> 0 AND a.tenant_id = 0`},
	}
	for _, v := range via {
		if err := db.Exec(v.sql).Error; err != nil {
			warnMigrateSkipped("migrateRound105AlertTenant:"+v.taskType, err)
		}
	}
	return nil
}
