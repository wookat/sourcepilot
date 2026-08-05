package demoseed

import (
	"fmt"
	"time"

	"gorm.io/gorm"

	"github.com/trademind-ai/trademind/backend/internal/modules/finance"
	"github.com/trademind-ai/trademind/backend/internal/modules/order"
	"github.com/trademind-ai/trademind/backend/internal/modules/procurement"
	"github.com/trademind-ai/trademind/backend/internal/modules/product"
	"github.com/trademind-ai/trademind/backend/internal/modules/shop"
	"github.com/trademind-ai/trademind/backend/internal/modules/sourcing"
)

// seedRound121Finance seeds the manual bookkeeping / reconciliation loop
// (Round 121): payment records with settled / short / over / unpaid samples,
// order-level and shop-monthly expenses, and a purchase order carrying an
// actual paid price so the actual-vs-estimated profit view is demoable out of
// the box.
func (s *FullDemoSeeder) seedRound121Finance(tx *gorm.DB, res *FullDemoResult, now time.Time,
	shops []shop.Shop, supplier *sourcing.Supplier, sku product.ProductSKU) error {
	count := func(table string, n int64) { res.Counts[table] += n }

	var paidOrders []order.Order
	if err := tx.Where("tenant_id = ? AND order_no LIKE ? AND payment_status = ?",
		s.TenantID, DemoPrefix+"%", order.PaymentPaid).
		Order("order_no ASC").Limit(3).Find(&paidOrders).Error; err != nil {
		return fmt.Errorf("demoseed: finance orders: %w", err)
	}
	if len(paidOrders) == 0 {
		return nil
	}

	day := func(hoursAgo int) time.Time { return now.Add(-time.Duration(hoursAgo) * time.Hour) }

	// Order 1: fully settled payment + platform commission / promotion expenses.
	o1 := paidOrders[0]
	p1 := finance.PaymentRecord{TenantID: s.TenantID, OrderID: o1.ID, ShopID: o1.ShopID,
		Amount: o1.TotalAmount, Currency: o1.Currency, FeeAmount: round2(o1.TotalAmount * 0.02),
		ReceivedAt: day(20), Channel: "平台结算", Source: finance.SourceManual,
		Remark: "DEMO- 演示回款（已结清样本）"}
	if err := tx.Create(&p1).Error; err != nil {
		return fmt.Errorf("demoseed: payment settled: %w", err)
	}
	count("finance_payment_records", 1)

	expenses := []finance.OrderExpense{
		{TenantID: s.TenantID, OrderID: o1.ID, ShopID: o1.ShopID, TypeCode: "platform_commission",
			Amount: round2(o1.TotalAmount * 0.05), Currency: o1.Currency,
			Remark: "DEMO- 演示费用（平台佣金）"},
		{TenantID: s.TenantID, OrderID: o1.ID, ShopID: o1.ShopID, TypeCode: "promotion",
			Amount: 6.60, Currency: o1.Currency, Remark: "DEMO- 演示费用（推广费）"},
	}
	for i := range expenses {
		incurred := day(20)
		expenses[i].IncurredAt = &incurred
		if err := tx.Create(&expenses[i]).Error; err != nil {
			return fmt.Errorf("demoseed: order expense: %w", err)
		}
	}
	count("finance_order_expenses", int64(len(expenses)))

	// Order 2: short payment (少款) sample.
	if len(paidOrders) > 1 {
		o2 := paidOrders[1]
		p2 := finance.PaymentRecord{TenantID: s.TenantID, OrderID: o2.ID, ShopID: o2.ShopID,
			Amount: round2(o2.TotalAmount * 0.6), Currency: o2.Currency,
			ReceivedAt: day(16), Channel: "平台结算", Source: finance.SourceManual,
			Remark: "DEMO- 演示回款（少款样本）"}
		if err := tx.Create(&p2).Error; err != nil {
			return fmt.Errorf("demoseed: payment short: %w", err)
		}
		count("finance_payment_records", 1)
	}

	// Order 3: over payment (多款) sample, two records to demo accumulation.
	if len(paidOrders) > 2 {
		o3 := paidOrders[2]
		for i, amt := range []float64{round2(o3.TotalAmount * 0.7), round2(o3.TotalAmount*0.4) + 5} {
			p := finance.PaymentRecord{TenantID: s.TenantID, OrderID: o3.ID, ShopID: o3.ShopID,
				Amount: amt, Currency: o3.Currency, ReceivedAt: day(12 - i),
				Channel: "银行转账", Source: finance.SourceManual,
				Remark: "DEMO- 演示回款（多款样本）"}
			if err := tx.Create(&p).Error; err != nil {
				return fmt.Errorf("demoseed: payment over: %w", err)
			}
			count("finance_payment_records", 1)
		}
	}

	// Shop-monthly expenses for the reconciliation report.
	month := now.Format("2006-01")
	shopExpenses := []finance.ShopMonthlyExpense{
		{TenantID: s.TenantID, ShopID: shops[0].ID, Month: month, TypeCode: "promotion",
			Amount: 88.00, Currency: "CNY", Remark: "DEMO- 店铺月度推广费（种子数据）"},
		{TenantID: s.TenantID, ShopID: shops[0].ID, Month: month, TypeCode: "other",
			Amount: 30.00, Currency: "CNY", Remark: "DEMO- 店铺月度杂费（种子数据）"},
	}
	for i := range shopExpenses {
		if err := tx.Create(&shopExpenses[i]).Error; err != nil {
			return fmt.Errorf("demoseed: shop expense: %w", err)
		}
	}
	count("finance_shop_monthly_expenses", int64(len(shopExpenses)))

	// Purchase order with actual paid price bound to the settled order so the
	// actual-vs-estimated profit comparison has data.
	expected := 12.50
	actual := 13.20
	po := procurement.PurchaseOrder{TenantID: s.TenantID, SupplierID: supplier.ID, SupplierName: supplier.Name,
		SourcePlatform: "1688", Status: procurement.StatusDelivered,
		TotalAmount: round2(actual * 2), Currency: "CNY", PayStatus: procurement.PayStatusPaid,
		IdempotencyKey: "DEMO-R121-FINANCE-PO"}
	paidAt := day(18)
	po.PaidAt = &paidAt
	if err := tx.Create(&po).Error; err != nil {
		return fmt.Errorf("demoseed: finance po: %w", err)
	}
	count("purchase_orders", 1)
	item := procurement.PurchaseOrderItem{TenantID: s.TenantID, PurchaseOrderID: po.ID,
		LocalSKUID: sku.ID, SourceSKUID: sku.ID, SKUName: sku.SKUName,
		SalesOrderID: &o1.ID, ProductTitle: "DEMO-采购商品（实付价样本）",
		Quantity: 2, ExpectedPrice: &expected, ActualPrice: &actual}
	if err := tx.Create(&item).Error; err != nil {
		return fmt.Errorf("demoseed: finance po item: %w", err)
	}
	count("purchase_order_items", 1)

	return nil
}

func round2(v float64) float64 {
	return float64(int64(v*100+0.5)) / 100
}
