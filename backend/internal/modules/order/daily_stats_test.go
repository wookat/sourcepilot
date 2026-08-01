package order_test

import (
	"testing"
	"time"

	"github.com/trademind-ai/trademind/backend/internal/modules/order"
)

func TestDailyStats(t *testing.T) {
	db := openImportTestDB(t)
	svc := &order.Service{DB: db}
	now := time.Now()
	today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())

	paid := createFlowTestOrder(t, svc, "SO-DS-PAID")
	if err := db.Model(&order.Order{}).Where("id = ?", paid.ID).
		Updates(map[string]any{"payment_status": order.PaymentPaid, "total_amount": 12.5, "currency": "USD"}).Error; err != nil {
		t.Fatal(err)
	}
	paidEUR := createFlowTestOrder(t, svc, "SO-DS-PAID-EUR")
	if err := db.Model(&order.Order{}).Where("id = ?", paidEUR.ID).
		Updates(map[string]any{"payment_status": order.PaymentPaid, "total_amount": 7.5, "currency": "EUR"}).Error; err != nil {
		t.Fatal(err)
	}
	createFlowTestOrder(t, svc, "SO-DS-UNPAID")

	// yesterday: one paid order
	yesterdayPaid := createFlowTestOrder(t, svc, "SO-DS-YESTERDAY")
	if err := db.Model(&order.Order{}).Where("id = ?", yesterdayPaid.ID).
		Updates(map[string]any{"payment_status": order.PaymentPaid, "total_amount": 3, "currency": "USD",
			"created_at": today.AddDate(0, 0, -1).Add(6 * time.Hour)}).Error; err != nil {
		t.Fatal(err)
	}
	// outside window: must not be counted
	old := createFlowTestOrder(t, svc, "SO-DS-OLD")
	if err := db.Model(&order.Order{}).Where("id = ?", old.ID).
		Updates(map[string]any{"created_at": today.AddDate(0, 0, -40)}).Error; err != nil {
		t.Fatal(err)
	}
	// soft-deleted: must not be counted
	deleted := createFlowTestOrder(t, svc, "SO-DS-DELETED")
	if err := db.Delete(&order.Order{}, "id = ?", deleted.ID).Error; err != nil {
		t.Fatal(err)
	}
	// other tenant: must not be counted
	otherTenant := createFlowTestOrder(t, svc, "SO-DS-T2")
	if err := db.Model(&order.Order{}).Where("id = ?", otherTenant.ID).
		Updates(map[string]any{"tenant_id": 2}).Error; err != nil {
		t.Fatal(err)
	}

	c := importTestCtx(1)
	res, err := svc.DailyStats(c, 30)
	if err != nil {
		t.Fatal(err)
	}
	if res.Days != 30 || len(res.Items) != 30 {
		t.Fatalf("expected 30 items, got days=%d items=%d", res.Days, len(res.Items))
	}
	last := res.Items[len(res.Items)-1]
	if last.Date != today.Format("2006-01-02") {
		t.Fatalf("expected last item today, got %s", last.Date)
	}
	if last.OrderCount != 3 || last.PaidCount != 2 {
		t.Fatalf("today expected 3/2, got %d/%d", last.OrderCount, last.PaidCount)
	}
	if len(last.PaidAmounts) != 2 ||
		last.PaidAmounts[0].Currency != "EUR" || last.PaidAmounts[0].Amount != 7.5 || last.PaidAmounts[0].Orders != 1 ||
		last.PaidAmounts[1].Currency != "USD" || last.PaidAmounts[1].Amount != 12.5 || last.PaidAmounts[1].Orders != 1 {
		t.Fatalf("today unexpected paid amounts: %+v", last.PaidAmounts)
	}
	yesterday := res.Items[len(res.Items)-2]
	if yesterday.OrderCount != 1 || yesterday.PaidCount != 1 {
		t.Fatalf("yesterday expected 1/1, got %d/%d", yesterday.OrderCount, yesterday.PaidCount)
	}
	if len(yesterday.PaidAmounts) != 1 || yesterday.PaidAmounts[0].Currency != "USD" || yesterday.PaidAmounts[0].Amount != 3 {
		t.Fatalf("yesterday unexpected paid amounts: %+v", yesterday.PaidAmounts)
	}
	var totalOrders int64
	for _, it := range res.Items {
		totalOrders += it.OrderCount
	}
	if totalOrders != 4 {
		t.Fatalf("expected 4 in-window orders, got %d", totalOrders)
	}
	// empty days stay zero-filled
	if res.Items[0].OrderCount != 0 || len(res.Items[0].PaidAmounts) != 0 {
		t.Fatalf("expected empty first day, got %+v", res.Items[0])
	}
}

func TestDailyStatsClampsDays(t *testing.T) {
	db := openImportTestDB(t)
	svc := &order.Service{DB: db}
	c := importTestCtx(1)
	res, err := svc.DailyStats(c, 0)
	if err != nil {
		t.Fatal(err)
	}
	if res.Days != 30 || len(res.Items) != 30 {
		t.Fatalf("default expected 30 days, got %d/%d", res.Days, len(res.Items))
	}
	res, err = svc.DailyStats(c, 400)
	if err != nil {
		t.Fatal(err)
	}
	if res.Days != 90 || len(res.Items) != 90 {
		t.Fatalf("clamp expected 90 days, got %d/%d", res.Days, len(res.Items))
	}
}
