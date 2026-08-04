package taskcenter

import (
	"context"
)

// resolveSourceTenant derives the owning tenant of one failing task row from
// its source table (directly for tenant-columned tables, via the owning
// product/batch/shop otherwise). Returns 0 (platform bucket) when the source
// row is gone or carries no tenant linkage.
func (s *Service) resolveSourceTenant(ctx context.Context, taskType, sourceID string) int64 {
	if s == nil || s.DB == nil || sourceID == "" {
		return 0
	}
	var sql string
	switch taskType {
	case TaskTypeCollect:
		sql = `SELECT tenant_id FROM collect_tasks WHERE CAST(id AS TEXT) = ?`
	case TaskTypeOrderSync:
		sql = `SELECT tenant_id FROM order_sync_tasks WHERE CAST(id AS TEXT) = ?`
	case TaskTypeCustomerMessageSync:
		sql = `SELECT tenant_id FROM customer_message_sync_tasks WHERE CAST(id AS TEXT) = ?`
	case TaskTypeProductPublish:
		sql = `SELECT tenant_id FROM product_publish_tasks WHERE CAST(id AS TEXT) = ?`
	case TaskTypeInventorySync:
		sql = `SELECT tenant_id FROM inventory_sync_tasks WHERE CAST(id AS TEXT) = ?`
	case TaskTypeImage:
		sql = `SELECT COALESCE(p.tenant_id, 0) FROM image_tasks t
			LEFT JOIN products p ON p.id = t.product_id
			WHERE CAST(t.id AS TEXT) = ?`
	case TaskTypeAIText:
		sql = `SELECT b.tenant_id FROM ai_product_text_items i
			JOIN ai_product_text_batches b ON b.id = i.batch_id
			WHERE CAST(i.id AS TEXT) = ?`
	case TaskTypeAIImage:
		sql = `SELECT b.tenant_id FROM ai_product_image_items i
			JOIN ai_product_image_batches b ON b.id = i.batch_id
			WHERE CAST(i.id AS TEXT) = ?`
	case TaskTypeCustomerFailure:
		sql = `SELECT sh.tenant_id FROM customer_failure_events e
			JOIN shops sh ON sh.id = e.shop_id
			WHERE CAST(e.id AS TEXT) = ?`
	default:
		return 0
	}
	var tid int64
	if err := s.DB.WithContext(ctx).Raw(sql, sourceID).Scan(&tid).Error; err != nil {
		return 0
	}
	if tid < 0 {
		return 0
	}
	return tid
}
