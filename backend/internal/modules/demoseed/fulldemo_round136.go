package demoseed

import (
	"fmt"
	"time"

	"gorm.io/gorm"

	"github.com/trademind-ai/trademind/backend/internal/modules/finance"
	"github.com/trademind-ai/trademind/backend/internal/modules/order"
	"github.com/trademind-ai/trademind/backend/internal/modules/product"
	"github.com/trademind-ai/trademind/backend/internal/modules/shop"
)

// seedRound136OperatorAutomation adds DEMO-AT-1005: the operator-scope twin
// of DEMO-AT-1004. It lives on the manual demo shop (the store granted to the
// demo operator/readonly accounts), is unpaid + review auto-passed and its
// item is matched to a local SKU with a primary source, so the operator
// account can batch-mark it paid and really trigger the「自动生成采购单」
// positive automation path without borrowing the admin account.
func (s *FullDemoSeeder) seedRound136OperatorAutomation(tx *gorm.DB, res *FullDemoResult, now time.Time,
	shops []shop.Shop, products []product.Product, skus []product.ProductSKU) error {
	if len(shops) < 2 || len(products) == 0 || len(skus) == 0 {
		return nil
	}
	count := func(table string, n int64) { res.Counts[table] += n }

	amount := 96.0
	created := now.Add(-20 * time.Minute)
	o := order.Order{
		TenantID: s.TenantID, Platform: shops[1].Platform, ShopID: &shops[1].ID,
		OrderNo: "DEMO-AT-1005", CustomerName: "DEMO-自动化买家", CustomerPhone: "13800000119",
		Status: order.StatusPending, ReviewStatus: order.ReviewStatusAutoPassed,
		PaymentStatus: order.PaymentUnpaid, FulfillmentStatus: order.FulfillmentUnfulfilled,
		Currency: "CNY", TotalAmount: amount, OrderedAt: &created,
	}
	if err := tx.Create(&o).Error; err != nil {
		return fmt.Errorf("demoseed: operator automation order: %w", err)
	}
	item := order.OrderItem{
		OrderID: o.ID, ProductID: &products[0].ID, ProductSKUID: &skus[0].ID,
		ProductTitle: products[0].Title, SKUCode: skus[0].SKUCode, SKUName: skus[0].SKUName,
		Quantity: 1, UnitPrice: amount, TotalPrice: amount,
	}
	if err := tx.Create(&item).Error; err != nil {
		return fmt.Errorf("demoseed: operator automation item: %w", err)
	}
	match := order.OrderItemSKUMatch{
		OrderID: o.ID, OrderItemID: item.ID, Platform: o.Platform,
		SKUCode:   item.SKUCode,
		ProductID: item.ProductID, ProductSKUID: item.ProductSKUID,
		MatchType: order.MatchTypeLocalSKUCode, MatchStatus: order.MatchStatusMatched, Confidence: 100,
		Reason: "DEMO- 种子数据本地 SKU 匹配",
	}
	if err := tx.Create(&match).Error; err != nil {
		return fmt.Errorf("demoseed: operator automation sku match: %w", err)
	}
	count("orders", 1)
	count("order_items", 1)
	count("order_item_sku_matches", 1)
	return nil
}

// round136FinanceOrderTarget is the minimum number of paid orders the seed
// guarantees for the reconciliation workbench, so the 20-per-page table has a
// second page and the summary counters aggregate across pages out of the box.
const round136FinanceOrderTarget = 28

// seedRound136FinanceVolume tops up paid DEMO-FIN- orders with reconciliation
// payment samples (settled / short / over / unpaid) until the tenant has at
// least round136FinanceOrderTarget paid orders. Rows alternate between the
// douyin and manual demo shops so scoped roles also see multiple rows. All
// rows carry the DEMO- prefix and are removed by Cleanup / checked by
// VerifyClean.
func (s *FullDemoSeeder) seedRound136FinanceVolume(tx *gorm.DB, res *FullDemoResult, now time.Time,
	shops []shop.Shop, skus []product.ProductSKU) error {
	if len(shops) < 2 || len(skus) == 0 {
		return nil
	}
	count := func(table string, n int64) { res.Counts[table] += n }

	var paid int64
	if err := tx.Model(&order.Order{}).
		Where("tenant_id = ? AND order_no LIKE ? AND payment_status = ?",
			s.TenantID, DemoPrefix+"%", order.PaymentPaid).
		Count(&paid).Error; err != nil {
		return fmt.Errorf("demoseed: count paid orders: %w", err)
	}
	missing := round136FinanceOrderTarget - int(paid)
	if missing <= 0 {
		return nil
	}

	// settlement plans cycle: settled → short → over → unpaid (no record).
	type finPlan struct {
		kind  string
		ratio float64 // payment amount as ratio of receivable
	}
	plans := []finPlan{
		{kind: "settled", ratio: 1.0},
		{kind: "settled", ratio: 1.0},
		{kind: "short", ratio: 0.65},
		{kind: "over", ratio: 1.15},
		{kind: "unpaid", ratio: 0},
		{kind: "settled", ratio: 1.0},
	}
	kindLabel := map[string]string{
		"settled": "已结清样本", "short": "少款样本", "over": "多款样本",
	}

	for i := 0; i < missing; i++ {
		plan := plans[i%len(plans)]
		sku := skus[i%len(skus)]
		unit := 39.0
		if sku.Price != nil && *sku.Price > 0 {
			unit = *sku.Price
		}
		qty := 1 + i%3
		total := round2(unit * float64(qty))
		sh := shops[i%2]
		orderedAt := now.Add(-time.Duration(6+i*3) * time.Hour)
		paidAt := orderedAt.Add(25 * time.Minute)
		o := order.Order{TenantID: s.TenantID, Platform: sh.Platform, ShopID: &sh.ID,
			OrderNo:      fmt.Sprintf("DEMO-FIN-%04d", 2101+i),
			CustomerName: fmt.Sprintf("DEMO-对账买家%d", i+1),
			Status:       order.StatusPaid, PaymentStatus: order.PaymentPaid,
			FulfillmentStatus: order.FulfillmentUnfulfilled,
			Currency:          "CNY", TotalAmount: total,
			OrderedAt: &orderedAt, PaidAt: &paidAt,
			Remark: "DEMO- 对账量样本（种子数据）"}
		if err := tx.Create(&o).Error; err != nil {
			return fmt.Errorf("demoseed: round136 order %s: %w", o.OrderNo, err)
		}
		count("orders", 1)

		item := order.OrderItem{OrderID: o.ID, ProductID: &sku.ProductID, ProductSKUID: &sku.ID,
			ProductTitle: "DEMO-对账演示商品", SKUName: sku.SKUName, SKUCode: sku.SKUCode,
			Quantity: qty, UnitPrice: unit, TotalPrice: total}
		if err := tx.Create(&item).Error; err != nil {
			return fmt.Errorf("demoseed: round136 order item: %w", err)
		}
		count("order_items", 1)

		if plan.kind == "unpaid" {
			continue
		}
		p := finance.PaymentRecord{TenantID: s.TenantID, OrderID: o.ID, ShopID: o.ShopID,
			Amount: round2(total * plan.ratio), Currency: o.Currency,
			ReceivedAt: paidAt.Add(2 * time.Hour), Channel: "平台结算", Source: finance.SourceManual,
			Remark: "DEMO- 演示回款（对账量样本·" + kindLabel[plan.kind] + "）"}
		if plan.kind == "settled" {
			p.FeeAmount = round2(total * 0.02)
		}
		if err := tx.Create(&p).Error; err != nil {
			return fmt.Errorf("demoseed: round136 payment: %w", err)
		}
		count("finance_payment_records", 1)
	}
	return nil
}
