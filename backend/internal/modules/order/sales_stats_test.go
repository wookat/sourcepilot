package order_test

import (
	"testing"

	"github.com/trademind-ai/trademind/backend/internal/modules/order"
)

func TestSalesStats(t *testing.T) {
	db := openImportTestDB(t)
	svc := &order.Service{DB: db}
	paid := createFlowTestOrder(t, svc, "SO-ST-PAID")
	if err := db.Model(&order.Order{}).Where("id = ?", paid.ID).
		Updates(map[string]any{"payment_status": order.PaymentPaid, "total_amount": 12.5, "currency": "USD"}).Error; err != nil {
		t.Fatal(err)
	}
	shipped := createFlowTestOrder(t, svc, "SO-ST-SHIPPED")
	if err := db.Model(&order.Order{}).Where("id = ?", shipped.ID).
		Updates(map[string]any{"payment_status": order.PaymentPaid, "status": order.StatusShipped, "total_amount": 7.5, "currency": "USD"}).Error; err != nil {
		t.Fatal(err)
	}
	createFlowTestOrder(t, svc, "SO-ST-UNPAID")

	c := importTestCtx(1)
	res, err := svc.SalesStats(c)
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Windows) != 3 {
		t.Fatalf("expected 3 windows, got %d", len(res.Windows))
	}
	for _, w := range res.Windows {
		if w.OrderCount != 3 || w.PaidCount != 2 || w.ShippedCount != 1 {
			t.Fatalf("window %s expected 3/2/1, got %d/%d/%d", w.Key, w.OrderCount, w.PaidCount, w.ShippedCount)
		}
		if len(w.PaidAmounts) != 1 || w.PaidAmounts[0].Currency != "USD" || w.PaidAmounts[0].Amount != 20 || w.PaidAmounts[0].Orders != 2 {
			t.Fatalf("window %s unexpected paid amounts: %+v", w.Key, w.PaidAmounts)
		}
	}
}
