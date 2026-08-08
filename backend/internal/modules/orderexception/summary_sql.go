package orderexception

import (
	"context"
	"fmt"
	"strings"

	"github.com/google/uuid"
	"github.com/trademind-ai/trademind/backend/internal/modules/inventory"
	"github.com/trademind-ai/trademind/backend/internal/modules/order"
	"github.com/trademind-ai/trademind/backend/internal/modules/ordersync"
)

// SummaryOpenExceptions computes the open-exception summary with SQL COUNT
// aggregation instead of materializing every exception row in memory. It
// mirrors ListOrderExceptions summary semantics exactly: per-collector
// platform/shop/order/tenant filters, the AllowedShopIDs store scope, and
// exclusion of rows carrying a handled/ignored workbench mark. Start/End,
// Severity and Keyword do not affect the summary (they never did on the
// in-memory path). Negative margin still goes through the bounded collector
// (cost estimation cannot be expressed in SQL), so both paths share it.
func (s *Service) SummaryOpenExceptions(ctx context.Context, req ListOrderExceptionsRequest) (ExceptionSummaryDTO, error) {
	sum := ExceptionSummaryDTO{}
	if s == nil || s.DB == nil {
		return sum, fmt.Errorf("orderexception: unavailable")
	}
	// An explicit empty store scope means no shop is visible: every
	// exception row is bound to (or missing) a shop, so all counts are 0.
	if req.AllowedShopIDs != nil && len(req.AllowedShopIDs) == 0 {
		return sum, nil
	}

	if req.ExceptionType == "" || req.ExceptionType == TypeSKUUnmatched {
		if n, err := s.countSKUUnmatched(ctx, req); err == nil {
			sum.SKUUnmatched = n
		}
	}
	if req.ExceptionType == "" || req.ExceptionType == TypeSKUAmbiguous {
		if n, err := s.countSKUAmbiguous(ctx, req); err == nil {
			sum.SKUAmbiguous = n
		}
	}
	if req.ExceptionType == "" || req.ExceptionType == TypeInsufficientStock || req.ExceptionType == TypeInventoryDeductFailed {
		if insufficient, deductFailed, err := s.countInventoryDeductFailures(ctx, req); err == nil {
			sum.InsufficientStock = insufficient
			sum.InventoryDeductFailed = deductFailed
		}
	}
	if req.ExceptionType == "" || req.ExceptionType == TypeInventoryRestoreFailed {
		if n, err := s.countInventoryRestoreFailures(ctx, req); err == nil {
			sum.InventoryRestoreFailed = n
		}
	}
	if req.ExceptionType == "" || req.ExceptionType == TypeInventorySyncFailed {
		if n, err := s.countInventorySyncFailed(ctx, req); err == nil {
			sum.InventorySyncFailed = n
		}
	}
	if req.ExceptionType == "" || req.ExceptionType == TypeOrderSyncPartialFailed {
		if n, err := s.countOrderSyncPartialFailed(ctx, req); err == nil {
			sum.OrderSyncPartial = n
		}
	}
	if req.ExceptionType == "" || req.ExceptionType == TypeProcurementBlocked {
		if n, err := s.countProcurementBlocked(ctx, req); err == nil {
			sum.ProcurementBlocked = n
		}
	}
	if req.ExceptionType == "" || req.ExceptionType == TypeNegativeMargin {
		if n, err := s.countNegativeMargin(ctx, req); err == nil {
			sum.NegativeMargin = n
		}
	}

	sum.TotalOpen = sum.SKUUnmatched + sum.SKUAmbiguous + sum.InsufficientStock +
		sum.InventoryDeductFailed + sum.InventoryRestoreFailed + sum.InventorySyncFailed +
		sum.OrderSyncPartial + sum.ProcurementBlocked + sum.NegativeMargin
	return sum, nil
}

// notMarked renders a NOT EXISTS guard excluding rows that carry a
// handled/ignored workbench mark. typeExpr / sourceTypeExpr / sourceIDExpr
// are SQL expressions evaluated per candidate row.
func notMarked(typeExpr, sourceTypeExpr, sourceIDExpr string) string {
	return ` AND NOT EXISTS (
    SELECT 1 FROM order_exception_marks em
    WHERE em.exception_type = ` + typeExpr + `
      AND em.source_type = ` + sourceTypeExpr + `
      AND em.source_id = ` + sourceIDExpr + `
      AND em.mark_type IN ('handled','ignored')
  )`
}

// shopScopeSQL appends the AllowedShopIDs store scope on shopCol: rows bound
// to a shop outside the scope (or with no shop binding) are excluded,
// matching ListOrderExceptionsRequest.shopAllowed.
func shopScopeSQL(q string, args []any, req ListOrderExceptionsRequest, shopCol string) (string, []any) {
	if req.AllowedShopIDs == nil {
		return q, args
	}
	q += ` AND ` + shopCol + ` IS NOT NULL AND ` + shopCol + ` <> '00000000-0000-0000-0000-000000000000' AND ` + shopCol + ` IN ?`
	args = append(args, req.AllowedShopIDs)
	return q, args
}

func (s *Service) countSKUUnmatched(ctx context.Context, req ListOrderExceptionsRequest) (int64, error) {
	orderItem := &order.OrderItem{}
	if !s.DB.Migrator().HasTable(orderItem) {
		return 0, nil
	}
	if !s.DB.Migrator().HasColumn(orderItem, "product_sku_id") || !s.DB.Migrator().HasColumn(orderItem, "external_sku_id") {
		return 0, nil
	}
	q := `
SELECT COUNT(*)
FROM order_items oi
JOIN orders o ON o.id = oi.order_id AND o.deleted_at IS NULL
LEFT JOIN order_item_sku_matches m ON m.order_item_id = oi.id
WHERE LOWER(TRIM(o.platform)) NOT IN ('', 'manual')
  AND (
    (oi.product_sku_id IS NULL OR oi.product_sku_id = '00000000-0000-0000-0000-000000000000')
    OR m.match_status IN ('unmatched','skipped')
  )
  AND (m.id IS NULL OR m.match_status <> 'ambiguous')
`
	args := []any{}
	q, args = appendOrderFilters(q, args, req, "o")
	q += notMarked(`'`+TypeSKUUnmatched+`'`,
		`CASE WHEN m.id IS NULL THEN '`+SourceOrderItem+`' ELSE '`+SourceOrderItemSKUMatch+`' END`,
		`(CASE WHEN m.id IS NULL THEN oi.id ELSE m.id END)::text`)
	q, args = shopScopeSQL(q, args, req, "o.shop_id")
	var n int64
	err := s.DB.WithContext(ctx).Raw(q, args...).Scan(&n).Error
	return n, err
}

func (s *Service) countSKUAmbiguous(ctx context.Context, req ListOrderExceptionsRequest) (int64, error) {
	if !s.DB.Migrator().HasTable(&order.OrderItemSKUMatch{}) {
		return 0, nil
	}
	q := `
SELECT COUNT(*)
FROM order_item_sku_matches m
JOIN orders o ON o.id = m.order_id AND o.deleted_at IS NULL
WHERE m.match_status = 'ambiguous'
`
	args := []any{}
	if req.TenantID != nil {
		q += ` AND o.tenant_id = ?`
		args = append(args, *req.TenantID)
	}
	if req.Platform != "" {
		q += ` AND LOWER(m.platform) = ?`
		args = append(args, strings.ToLower(strings.TrimSpace(req.Platform)))
	}
	if req.ShopID != "" {
		if sid, err := uuid.Parse(strings.TrimSpace(req.ShopID)); err == nil {
			q += ` AND o.shop_id = ?`
			args = append(args, sid)
		}
	}
	if req.OrderID != "" {
		if oid, err := uuid.Parse(strings.TrimSpace(req.OrderID)); err == nil {
			q += ` AND m.order_id = ?`
			args = append(args, oid)
		}
	}
	q += notMarked(`'`+TypeSKUAmbiguous+`'`, `'`+SourceOrderItemSKUMatch+`'`, `m.id::text`)
	q, args = shopScopeSQL(q, args, req, "o.shop_id")
	var n int64
	err := s.DB.WithContext(ctx).Raw(q, args...).Scan(&n).Error
	return n, err
}

// countInventoryDeductFailures splits failed deduct effects into
// insufficient_stock vs inventory_deduct_failed on the same error-message
// heuristic the collector uses.
func (s *Service) countInventoryDeductFailures(ctx context.Context, req ListOrderExceptionsRequest) (int64, int64, error) {
	if !s.DB.Migrator().HasTable(&inventory.OrderInventoryEffect{}) {
		return 0, 0, nil
	}
	const insufficientExpr = `LOWER(COALESCE(e.error_message, '')) LIKE '%insufficient%'`
	q := `
SELECT
  COALESCE(SUM(CASE WHEN ` + insufficientExpr + ` THEN 1 ELSE 0 END), 0) AS insufficient,
  COALESCE(SUM(CASE WHEN ` + insufficientExpr + ` THEN 0 ELSE 1 END), 0) AS deduct_failed
FROM order_inventory_effects e
JOIN orders o ON o.id = e.order_id AND o.deleted_at IS NULL
WHERE e.effect_type = ? AND e.status = ?
`
	args := []any{inventory.EffectTypeDeduct, inventory.InventoryEffectFailed}
	q, args = appendOrderFilters(q, args, req, "o")
	q += notMarked(`CASE WHEN `+insufficientExpr+` THEN '`+TypeInsufficientStock+`' ELSE '`+TypeInventoryDeductFailed+`' END`,
		`'`+SourceOrderInventoryEffect+`'`, `e.id::text`)
	q, args = shopScopeSQL(q, args, req, "o.shop_id")
	var row struct {
		Insufficient int64 `gorm:"column:insufficient"`
		DeductFailed int64 `gorm:"column:deduct_failed"`
	}
	err := s.DB.WithContext(ctx).Raw(q, args...).Scan(&row).Error
	return row.Insufficient, row.DeductFailed, err
}

func (s *Service) countInventoryRestoreFailures(ctx context.Context, req ListOrderExceptionsRequest) (int64, error) {
	if !s.DB.Migrator().HasTable(&inventory.OrderInventoryEffect{}) {
		return 0, nil
	}
	q := `
SELECT COUNT(*)
FROM order_inventory_effects e
JOIN orders o ON o.id = e.order_id AND o.deleted_at IS NULL
WHERE e.effect_type = ? AND e.status = ?
`
	args := []any{inventory.EffectTypeRestore, inventory.InventoryEffectFailed}
	q, args = appendOrderFilters(q, args, req, "o")
	q += notMarked(`'`+TypeInventoryRestoreFailed+`'`, `'`+SourceOrderInventoryEffect+`'`, `e.id::text`)
	q, args = shopScopeSQL(q, args, req, "o.shop_id")
	var n int64
	err := s.DB.WithContext(ctx).Raw(q, args...).Scan(&n).Error
	return n, err
}

func (s *Service) countInventorySyncFailed(ctx context.Context, req ListOrderExceptionsRequest) (int64, error) {
	dst := &inventory.InventorySyncTask{}
	if !s.DB.Migrator().HasTable(dst) {
		return 0, nil
	}
	q := `
SELECT COUNT(*)
FROM inventory_sync_tasks t
WHERE t.status = ?
`
	args := []any{inventory.StatusFailed}
	hasTaskSKU := s.DB.Migrator().HasColumn(dst, "product_sku_id")
	hasEffectSKU := s.DB.Migrator().HasTable(&inventory.OrderInventoryEffect{}) &&
		s.DB.Migrator().HasColumn(&inventory.OrderInventoryEffect{}, "product_sku_id")
	if hasTaskSKU && hasEffectSKU {
		q += `  AND t.product_sku_id IS NOT NULL
  AND EXISTS (
    SELECT 1 FROM order_inventory_effects oie
    WHERE oie.product_sku_id = t.product_sku_id
      AND oie.effect_type = ?
      AND oie.status = ?
  )
`
		args = append(args, inventory.EffectTypeDeduct, inventory.InventoryEffectSuccess)
	}
	if req.TenantID != nil {
		q += ` AND t.tenant_id = ?`
		args = append(args, *req.TenantID)
	}
	if req.Platform != "" {
		q += ` AND LOWER(t.platform) = ?`
		args = append(args, strings.ToLower(strings.TrimSpace(req.Platform)))
	}
	if req.ShopID != "" {
		if sid, err := uuid.Parse(strings.TrimSpace(req.ShopID)); err == nil {
			q += ` AND t.shop_id = ?`
			args = append(args, sid)
		}
	}
	q += notMarked(`'`+TypeInventorySyncFailed+`'`, `'`+SourceInventorySyncTask+`'`, `t.id::text`)
	q, args = shopScopeSQL(q, args, req, "t.shop_id")
	var n int64
	err := s.DB.WithContext(ctx).Raw(q, args...).Scan(&n).Error
	return n, err
}

func (s *Service) countOrderSyncPartialFailed(ctx context.Context, req ListOrderExceptionsRequest) (int64, error) {
	if !s.DB.Migrator().HasTable(&ordersync.OrderSyncTask{}) {
		return 0, nil
	}
	q := `
SELECT COUNT(*)
FROM order_sync_tasks t
WHERE t.status = ?
`
	args := []any{ordersync.StatusPartialSuccess}
	if req.TenantID != nil {
		q += ` AND t.tenant_id = ?`
		args = append(args, *req.TenantID)
	}
	if req.Platform != "" {
		q += ` AND LOWER(t.platform) = ?`
		args = append(args, strings.ToLower(strings.TrimSpace(req.Platform)))
	}
	if req.ShopID != "" {
		if sid, err := uuid.Parse(strings.TrimSpace(req.ShopID)); err == nil {
			q += ` AND t.shop_id = ?`
			args = append(args, sid)
		}
	}
	q += notMarked(`'`+TypeOrderSyncPartialFailed+`'`, `'`+SourceOrderSyncTask+`'`, `t.id::text`)
	q, args = shopScopeSQL(q, args, req, "t.shop_id")
	var n int64
	err := s.DB.WithContext(ctx).Raw(q, args...).Scan(&n).Error
	return n, err
}

func (s *Service) countProcurementBlocked(ctx context.Context, req ListOrderExceptionsRequest) (int64, error) {
	mig := s.DB.Migrator()
	if !mig.HasTable("orders") || !mig.HasTable("order_items") ||
		!mig.HasTable("product_skus") || !mig.HasTable("product_sources") ||
		!mig.HasTable("product_source_skus") {
		return 0, nil
	}
	hasPOI := mig.HasTable("purchase_order_items") && mig.HasTable("purchase_orders")

	q := `
SELECT COUNT(*)
FROM order_items oi
JOIN orders o ON o.id = oi.order_id AND o.deleted_at IS NULL
JOIN product_skus ps ON ps.id = oi.product_sku_id
LEFT JOIN product_sources src
  ON src.product_id = ps.product_id
 AND src.is_primary = TRUE
 AND src.status <> 'disabled'
 AND src.deleted_at IS NULL
LEFT JOIN product_source_skus map
  ON map.product_source_id = src.id
 AND map.local_sku_id = oi.product_sku_id
 AND map.deleted_at IS NULL
WHERE o.payment_status = ?
  AND o.status NOT IN (?, ?, ?)
  AND o.fulfillment_status = ?
  AND oi.product_sku_id IS NOT NULL
  AND oi.product_sku_id <> '00000000-0000-0000-0000-000000000000'
  AND (src.id IS NULL OR map.id IS NULL)
`
	args := []any{
		order.PaymentPaid,
		order.StatusCancelled, order.StatusRefunded, order.StatusClosed,
		order.FulfillmentUnfulfilled,
	}
	if hasPOI {
		q += `  AND NOT EXISTS (
    SELECT 1 FROM purchase_order_items poi
    JOIN purchase_orders po ON po.id = poi.purchase_order_id AND po.deleted_at IS NULL
    WHERE poi.sales_order_id = o.id
      AND poi.local_sku_id = oi.product_sku_id
      AND po.status NOT IN ('cancelled','failed','voided')
  )
`
	}
	q, args = appendOrderFilters(q, args, req, "o")
	q += notMarked(`'`+TypeProcurementBlocked+`'`, `'`+SourceOrderItem+`'`, `oi.id::text`)
	q, args = shopScopeSQL(q, args, req, "o.shop_id")
	var n int64
	err := s.DB.WithContext(ctx).Raw(q, args...).Scan(&n).Error
	return n, err
}

// countNegativeMargin reuses the bounded collector (cost estimation runs in
// Go) and drops marked rows, so both summary paths share one margin oracle.
func (s *Service) countNegativeMargin(ctx context.Context, req ListOrderExceptionsRequest) (int64, error) {
	xs, err := s.collectNegativeMargin(ctx, req)
	if err != nil || len(xs) == 0 {
		return 0, err
	}
	var markRows []OrderExceptionMark
	if err := s.DB.WithContext(ctx).
		Where("exception_type = ?", TypeNegativeMargin).
		Find(&markRows).Error; err != nil {
		return 0, err
	}
	marks := buildMarkIndex(markRows)
	var n int64
	for _, r := range xs {
		if !req.shopAllowed(r.shopID) {
			continue
		}
		mp := marks[markKey(r.exceptionType, r.sourceType, r.sourceID.String())]
		if mp.handled || mp.ignored {
			continue
		}
		n++
	}
	return n, nil
}

// appendOrderFilters appends the shared tenant/platform/shop/order filters on
// the joined orders table alias.
func appendOrderFilters(q string, args []any, req ListOrderExceptionsRequest, alias string) (string, []any) {
	if req.TenantID != nil {
		q += ` AND ` + alias + `.tenant_id = ?`
		args = append(args, *req.TenantID)
	}
	if req.Platform != "" {
		q += ` AND LOWER(` + alias + `.platform) = ?`
		args = append(args, strings.ToLower(strings.TrimSpace(req.Platform)))
	}
	if req.ShopID != "" {
		if sid, err := uuid.Parse(strings.TrimSpace(req.ShopID)); err == nil {
			q += ` AND ` + alias + `.shop_id = ?`
			args = append(args, sid)
		}
	}
	if req.OrderID != "" {
		if oid, err := uuid.Parse(strings.TrimSpace(req.OrderID)); err == nil {
			q += ` AND ` + alias + `.id = ?`
			args = append(args, oid)
		}
	}
	return q, args
}
