package demoseed

import (
	"fmt"
	"time"

	"github.com/trademind-ai/trademind/backend/internal/modules/order"
	"github.com/trademind-ai/trademind/backend/internal/modules/product"
	"github.com/trademind-ai/trademind/backend/internal/modules/shop"
	"gorm.io/gorm"
)

// round156ScreenFXOrders are today's multi-currency big-screen samples: the
// USD orders convert into salesBase via the seeded manual rate (USD 7.20),
// while the EUR order has no rate on purpose, so the screen's「未折算」
// explicit fallback (excluded from the base total) is demonstrable out of
// the box. Order numbers carry the DEMO- prefix for cleanup / verify.
var round156ScreenFXOrders = []struct {
	orderNo  string
	currency string
	amount   float64
}{
	{orderNo: "DEMO-FX-USD-0001", currency: "USD", amount: 199.99},
	{orderNo: "DEMO-FX-EUR-0001", currency: "EUR", amount: 88.00},
}

// seedRound156ScreenFXOrders creates paid orders stamped inside today's
// local day so the big-screen「今日」KPIs pick them up regardless of when the
// seed runs.
func (s *FullDemoSeeder) seedRound156ScreenFXOrders(tx *gorm.DB, res *FullDemoResult,
	shops []shop.Shop, products []product.Product, skus []product.ProductSKU) error {
	if len(shops) < 3 || len(skus) == 0 || len(products) == 0 {
		return fmt.Errorf("demoseed: round156 needs shops/products/skus")
	}
	count := func(table string, n int64) { res.Counts[table] += n }

	localNow := time.Now()
	todayStart := time.Date(localNow.Year(), localNow.Month(), localNow.Day(), 0, 0, 0, 0, localNow.Location())
	orderedAt := localNow.Add(-1 * time.Hour)
	if orderedAt.Before(todayStart) {
		orderedAt = todayStart.Add(5 * time.Minute)
	}

	for i, p := range round156ScreenFXOrders {
		at := orderedAt.Add(time.Duration(i) * time.Minute)
		paidAt := at.Add(10 * time.Minute)
		o := order.Order{TenantID: s.TenantID, Platform: shops[2].Platform, ShopID: &shops[2].ID,
			OrderNo:      p.orderNo,
			CustomerName: fmt.Sprintf("DEMO-大屏多币种买家%d", i+1), Status: order.StatusPaid,
			PaymentStatus: order.PaymentPaid, FulfillmentStatus: order.FulfillmentUnfulfilled,
			Currency: p.currency, TotalAmount: p.amount, OrderedAt: &at, PaidAt: &paidAt,
			Remark: "DEMO- 大屏汇率折算演示订单（今日多币种样本）"}
		if err := tx.Create(&o).Error; err != nil {
			return fmt.Errorf("demoseed: round156 fx order: %w", err)
		}
		count("orders", 1)
		sku := skus[i%len(skus)]
		item := order.OrderItem{OrderID: o.ID, ProductID: &sku.ProductID, ProductSKUID: &sku.ID,
			ProductTitle: products[0].Title, SKUName: sku.SKUName, SKUCode: sku.SKUCode,
			Quantity: 1, UnitPrice: p.amount, TotalPrice: p.amount}
		if err := tx.Create(&item).Error; err != nil {
			return fmt.Errorf("demoseed: round156 fx order item: %w", err)
		}
		count("order_items", 1)
	}
	return nil
}
