package order_test

import (
	"strings"
	"testing"
	"time"

	"github.com/trademind-ai/trademind/backend/internal/modules/order"
)

func TestExportDailyStatsCSV(t *testing.T) {
	db := openImportTestDB(t)
	svc := &order.Service{DB: db}
	now := time.Now()
	today := now.Format("2006-01-02")

	paid := createFlowTestOrder(t, svc, "SO-DSE-PAID")
	if err := db.Model(&order.Order{}).Where("id = ?", paid.ID).
		Updates(map[string]any{"payment_status": order.PaymentPaid, "total_amount": 12.5, "currency": "USD"}).Error; err != nil {
		t.Fatal(err)
	}
	shipped := createFlowTestOrder(t, svc, "SO-DSE-SHIPPED")
	if err := db.Model(&order.Order{}).Where("id = ?", shipped.ID).
		Updates(map[string]any{"payment_status": order.PaymentPaid, "total_amount": 7.5, "currency": "EUR",
			"status": order.StatusShipped}).Error; err != nil {
		t.Fatal(err)
	}
	createFlowTestOrder(t, svc, "SO-DSE-UNPAID")

	c := importTestCtx(1)
	data, name, err := svc.ExportDailyStatsCSV(c, 7)
	if err != nil {
		t.Fatal(err)
	}
	if name != "daily-report-7d.csv" {
		t.Fatalf("unexpected filename %q", name)
	}
	body := string(data)
	if !strings.HasPrefix(body, "\xEF\xBB\xBF") {
		t.Fatal("expected UTF-8 BOM")
	}
	lines := strings.Split(strings.TrimSpace(strings.TrimPrefix(body, "\xEF\xBB\xBF")), "\n")
	if len(lines) != 8 { // header + 7 days
		t.Fatalf("expected 8 lines, got %d", len(lines))
	}
	if lines[0] != "日期,订单数,已付款数,已发货数,已付款销售额(EUR),已付款销售额(USD)" {
		t.Fatalf("unexpected header %q", lines[0])
	}
	last := lines[len(lines)-1]
	if last != today+",3,2,1,7.50,12.50" {
		t.Fatalf("unexpected today row %q", last)
	}
	// empty days are zero-filled
	if lines[1] != strings.Split(lines[1], ",")[0]+",0,0,0,0.00,0.00" {
		t.Fatalf("unexpected empty row %q", lines[1])
	}
}

func TestExportDailyStatsCSVNoPaid(t *testing.T) {
	db := openImportTestDB(t)
	svc := &order.Service{DB: db}
	c := importTestCtx(1)
	data, _, err := svc.ExportDailyStatsCSV(c, 0)
	if err != nil {
		t.Fatal(err)
	}
	lines := strings.Split(strings.TrimSpace(strings.TrimPrefix(string(data), "\xEF\xBB\xBF")), "\n")
	if len(lines) != 31 {
		t.Fatalf("default expected 31 lines, got %d", len(lines))
	}
	if lines[0] != "日期,订单数,已付款数,已发货数" {
		t.Fatalf("unexpected header %q", lines[0])
	}
}
