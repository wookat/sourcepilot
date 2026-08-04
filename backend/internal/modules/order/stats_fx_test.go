package order_test

import (
	"strings"
	"testing"
	"time"

	"github.com/trademind-ai/trademind/backend/internal/modules/order"
	"github.com/trademind-ai/trademind/backend/internal/modules/settings"
	"github.com/trademind-ai/trademind/backend/internal/providers/fxrate"
	"gorm.io/gorm"
)

func seedReportCurrency(t *testing.T, db *gorm.DB, tenantID int64, base, ratesJSON string) {
	t.Helper()
	if err := db.AutoMigrate(&settings.Setting{}); err != nil {
		t.Fatal(err)
	}
	rows := []settings.Setting{
		{TenantID: tenantID, GroupKey: fxrate.SettingsGroup, ItemKey: fxrate.KeyBaseCurrency, ItemValue: base, ValueType: "string"},
		{TenantID: tenantID, GroupKey: fxrate.SettingsGroup, ItemKey: fxrate.KeyRates, ItemValue: ratesJSON, ValueType: "string"},
	}
	for _, r := range rows {
		if err := db.Create(&r).Error; err != nil {
			t.Fatal(err)
		}
	}
}

func TestDailyStatsBaseCurrencyConversion(t *testing.T) {
	db := openImportTestDB(t)
	seedReportCurrency(t, db, 1, "CNY", `{"USD":"7.13"}`)
	svc := &order.Service{DB: db, Settings: &settings.Service{DB: db}}

	// paid today: 12.5 USD (rate 7.13 → 89.125 → 89.13), 30 CNY, 7.5 EUR (no rate)
	for _, o := range []struct {
		no, cur string
		amt     float64
	}{{"SO-FX-USD", "USD", 12.5}, {"SO-FX-CNY", "CNY", 30}, {"SO-FX-EUR", "EUR", 7.5}} {
		created := createFlowTestOrder(t, svc, o.no)
		if err := db.Model(&order.Order{}).Where("id = ?", created.ID).
			Updates(map[string]any{"payment_status": order.PaymentPaid, "total_amount": o.amt, "currency": o.cur}).Error; err != nil {
			t.Fatal(err)
		}
	}

	c := importTestCtx(1)
	res, err := svc.DailyStats(c, 7)
	if err != nil {
		t.Fatal(err)
	}
	if res.BaseCurrency != "CNY" {
		t.Fatalf("base currency: %s", res.BaseCurrency)
	}
	today := time.Now().Format("2006-01-02")
	var st *order.DailyStat
	for i := range res.Items {
		if res.Items[i].Date == today {
			st = &res.Items[i]
		}
	}
	if st == nil {
		t.Fatal("today bucket missing")
	}
	// 12.5*7.13 + 30 = 119.125 → 119.13 (EUR excluded, exact decimal rounding)
	if st.PaidAmountBase != 119.13 {
		t.Fatalf("paidAmountBase got %v want 119.13", st.PaidAmountBase)
	}
	if len(st.UnconvertedCurrencies) != 1 || st.UnconvertedCurrencies[0] != "EUR" {
		t.Fatalf("unconverted got %v want [EUR]", st.UnconvertedCurrencies)
	}
	for _, a := range st.PaidAmounts {
		switch a.Currency {
		case "USD":
			if a.BaseAmount == nil || *a.BaseAmount != 89.13 {
				t.Fatalf("USD baseAmount got %v", a.BaseAmount)
			}
		case "CNY":
			if a.BaseAmount == nil || *a.BaseAmount != 30 {
				t.Fatalf("CNY baseAmount got %v", a.BaseAmount)
			}
		case "EUR":
			if a.BaseAmount != nil {
				t.Fatalf("EUR must stay unconverted, got %v", *a.BaseAmount)
			}
		}
	}
}

func TestDailyStatsWithoutRatesDefaultsToCNYUnconverted(t *testing.T) {
	db := openImportTestDB(t)
	svc := &order.Service{DB: db} // no settings at all
	created := createFlowTestOrder(t, svc, "SO-FX-NOCFG")
	if err := db.Model(&order.Order{}).Where("id = ?", created.ID).
		Updates(map[string]any{"payment_status": order.PaymentPaid, "total_amount": 10, "currency": "USD"}).Error; err != nil {
		t.Fatal(err)
	}
	res, err := svc.DailyStats(importTestCtx(1), 7)
	if err != nil {
		t.Fatal(err)
	}
	if res.BaseCurrency != "CNY" {
		t.Fatalf("default base: %s", res.BaseCurrency)
	}
	today := time.Now().Format("2006-01-02")
	for _, it := range res.Items {
		if it.Date != today {
			continue
		}
		if it.PaidAmountBase != 0 || len(it.UnconvertedCurrencies) != 1 || it.UnconvertedCurrencies[0] != "USD" {
			t.Fatalf("expected USD unconverted with 0 base, got base=%v unconverted=%v", it.PaidAmountBase, it.UnconvertedCurrencies)
		}
	}
}

func TestSalesStatsBaseCurrencyConversion(t *testing.T) {
	db := openImportTestDB(t)
	seedReportCurrency(t, db, 1, "CNY", `{"USD":"7"}`)
	svc := &order.Service{DB: db, Settings: &settings.Service{DB: db}}
	created := createFlowTestOrder(t, svc, "SO-FX-SALES")
	if err := db.Model(&order.Order{}).Where("id = ?", created.ID).
		Updates(map[string]any{"payment_status": order.PaymentPaid, "total_amount": 10, "currency": "USD"}).Error; err != nil {
		t.Fatal(err)
	}
	res, err := svc.SalesStats(importTestCtx(1))
	if err != nil {
		t.Fatal(err)
	}
	if res.BaseCurrency != "CNY" {
		t.Fatalf("base currency: %s", res.BaseCurrency)
	}
	for _, w := range res.Windows {
		if w.Key != "today" {
			continue
		}
		if w.PaidAmountBase != 70 {
			t.Fatalf("today base got %v want 70", w.PaidAmountBase)
		}
		if len(w.PaidAmounts) != 1 || w.PaidAmounts[0].BaseAmount == nil || *w.PaidAmounts[0].BaseAmount != 70 {
			t.Fatalf("paidAmounts %+v", w.PaidAmounts)
		}
		if len(w.UnconvertedCurrencies) != 0 {
			t.Fatalf("unexpected unconverted %v", w.UnconvertedCurrencies)
		}
	}
}

func TestExportDailyStatsCSVConvertedColumns(t *testing.T) {
	db := openImportTestDB(t)
	seedReportCurrency(t, db, 1, "CNY", `{"USD":"7.13"}`)
	svc := &order.Service{DB: db, Settings: &settings.Service{DB: db}}
	for _, o := range []struct {
		no, cur string
		amt     float64
	}{{"SO-FXCSV-USD", "USD", 12.5}, {"SO-FXCSV-EUR", "EUR", 7.5}} {
		created := createFlowTestOrder(t, svc, o.no)
		if err := db.Model(&order.Order{}).Where("id = ?", created.ID).
			Updates(map[string]any{"payment_status": order.PaymentPaid, "total_amount": o.amt, "currency": o.cur}).Error; err != nil {
			t.Fatal(err)
		}
	}
	data, _, err := svc.ExportDailyStatsCSV(importTestCtx(1), 7)
	if err != nil {
		t.Fatal(err)
	}
	body := string(data)
	lines := strings.Split(strings.TrimSpace(body), "\n")
	header := lines[0]
	for _, want := range []string{"已付款销售额(USD)", "折算金额(USD→CNY)", "已付款销售额(EUR)", "折算金额(EUR→CNY)", "已付款销售额合计(CNY)", "未折算币种"} {
		if !strings.Contains(header, want) {
			t.Fatalf("header missing %q: %s", want, header)
		}
	}
	today := time.Now().Format("2006-01-02")
	var todayLine string
	for _, ln := range lines[1:] {
		if strings.HasPrefix(ln, today) {
			todayLine = ln
		}
	}
	if todayLine == "" {
		t.Fatal("today row missing")
	}
	// EUR converted column must be blank (unconverted), USD converted 89.13, total 89.13, hint EUR
	if !strings.Contains(todayLine, "89.13") || !strings.Contains(todayLine, ",,") || !strings.HasSuffix(todayLine, "EUR") {
		t.Fatalf("today row unexpected: %s", todayLine)
	}
}
