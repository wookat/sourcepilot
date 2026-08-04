package procurement

import (
	"context"
	"math"
	"reflect"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/trademind-ai/trademind/backend/internal/modules/order"
	"github.com/trademind-ai/trademind/backend/internal/modules/sourcing"
)

type mapSettings map[string]string

func (m mapSettings) PlainByGroup(_ context.Context, _ int64, _ string) (map[string]string, error) {
	return m, nil
}

func TestEstimateOrderCostWithExchangeRate(t *testing.T) {
	f := setupFixture(t)
	f.svc.Settings = mapSettings{"exchangeRate": "0.14"}
	if err := f.svc.DB.Model(&order.Order{}).Where("id = ?", f.orderID).
		Update("total_amount", 30.0).Error; err != nil {
		t.Fatal(err)
	}

	out, err := f.svc.EstimateOrderCost(context.Background(), f.orderID)
	if err != nil {
		t.Fatalf("estimate: %v", err)
	}
	if out.MissingLines != 0 || len(out.Lines) != 1 {
		t.Fatalf("unexpected lines %+v", out)
	}
	// 3 × 9.9 = 29.7 CNY
	if out.EstimatedCostCNY != 29.7 {
		t.Fatalf("expected cost 29.7 CNY, got %v", out.EstimatedCostCNY)
	}
	if out.ExchangeRate == nil || *out.ExchangeRate != 0.14 {
		t.Fatalf("expected exchange rate 0.14, got %+v", out.ExchangeRate)
	}
	// 29.7 × 0.14 = 4.16 (rounded)
	if out.EstimatedCost == nil || *out.EstimatedCost != 4.16 {
		t.Fatalf("expected converted cost 4.16, got %+v", out.EstimatedCost)
	}
	if out.GrossProfit == nil || *out.GrossProfit != 25.84 {
		t.Fatalf("expected gross profit 25.84, got %+v", out.GrossProfit)
	}
	if out.MarginPercent == nil || *out.MarginPercent != 86.13 {
		t.Fatalf("expected margin 86.13, got %+v", out.MarginPercent)
	}
	if out.Lines[0].SupplierName != "supplier-a" {
		t.Fatalf("expected supplier name, got %+v", out.Lines[0])
	}
}

type tenantSettings map[int64]map[string]string

func (m tenantSettings) PlainByGroup(_ context.Context, tenantID int64, _ string) (map[string]string, error) {
	return m[tenantID], nil
}

func TestEstimateOrderCostFallbackTenantZeroSettings(t *testing.T) {
	f := setupFixture(t)
	// pricing page saves settings under tenant 0; order belongs to another tenant
	f.svc.Settings = tenantSettings{0: {"default_exchange_rate": "0.14"}}
	if err := f.svc.DB.Model(&order.Order{}).Where("id = ?", f.orderID).
		Update("tenant_id", 1).Error; err != nil {
		t.Fatal(err)
	}

	out, err := f.svc.EstimateOrderCost(context.Background(), f.orderID)
	if err != nil {
		t.Fatalf("estimate: %v", err)
	}
	if out.ExchangeRate == nil || *out.ExchangeRate != 0.14 {
		t.Fatalf("expected tenant-0 fallback rate 0.14, got %+v", out.ExchangeRate)
	}
}

func TestEstimateOrderCostFallbackDefaultExchangeRateKey(t *testing.T) {
	f := setupFixture(t)
	// pricing settings page writes default_exchange_rate, not exchangeRate
	f.svc.Settings = mapSettings{"default_exchange_rate": "0.14"}

	out, err := f.svc.EstimateOrderCost(context.Background(), f.orderID)
	if err != nil {
		t.Fatalf("estimate: %v", err)
	}
	if out.ExchangeRate == nil || *out.ExchangeRate != 0.14 {
		t.Fatalf("expected fallback rate 0.14, got %+v", out.ExchangeRate)
	}
	if out.EstimatedCost == nil || *out.EstimatedCost != 4.16 {
		t.Fatalf("expected converted cost 4.16, got %+v", out.EstimatedCost)
	}
}

type groupSettings map[string]map[string]string

func (m groupSettings) PlainByGroup(_ context.Context, _ int64, groupKey string) (map[string]string, error) {
	return m[groupKey], nil
}

func TestEstimateOrderCostPrefersReportCurrencyTable(t *testing.T) {
	f := setupFixture(t)
	// Report table USD→CNY=7.15 must win over legacy pricing rate 0.5 so the
	// gross margin estimate shares the sales report conversion rates.
	f.svc.Settings = groupSettings{
		"report_currency": {"base_currency": "CNY", "rates": `{"USD":"7.15"}`},
		"pricing":         {"exchangeRate": "0.5"},
	}
	if err := f.svc.DB.Model(&order.Order{}).Where("id = ?", f.orderID).
		Update("total_amount", 30.0).Error; err != nil {
		t.Fatal(err)
	}

	out, err := f.svc.EstimateOrderCost(context.Background(), f.orderID)
	if err != nil {
		t.Fatalf("estimate: %v", err)
	}
	if out.ExchangeRate == nil || math.Abs(*out.ExchangeRate-1/7.15) > 1e-12 {
		t.Fatalf("expected report-table rate 1/7.15, got %+v", out.ExchangeRate)
	}
	// 29.7 CNY / 7.15 = 4.1538… → 4.15 USD
	if out.EstimatedCost == nil || *out.EstimatedCost != 4.15 {
		t.Fatalf("expected converted cost 4.15, got %+v", out.EstimatedCost)
	}
	if out.GrossProfit == nil || *out.GrossProfit != 25.85 {
		t.Fatalf("expected gross profit 25.85, got %+v", out.GrossProfit)
	}
}

func TestEstimateOrderCostReportTableWithoutCurrencyFallsBack(t *testing.T) {
	f := setupFixture(t)
	// Table has no USD rate → legacy pricing exchangeRate still applies.
	f.svc.Settings = groupSettings{
		"report_currency": {"base_currency": "CNY", "rates": `{"EUR":"7.8"}`},
		"pricing":         {"exchangeRate": "0.14"},
	}

	out, err := f.svc.EstimateOrderCost(context.Background(), f.orderID)
	if err != nil {
		t.Fatalf("estimate: %v", err)
	}
	if out.ExchangeRate == nil || *out.ExchangeRate != 0.14 {
		t.Fatalf("expected fallback rate 0.14, got %+v", out.ExchangeRate)
	}
}

func TestEstimateOrderCostMissingRateAndPrice(t *testing.T) {
	f := setupFixture(t)
	// no Settings configured and currency is USD → no conversion/profit
	if err := f.svc.DB.Model(&sourcing.ProductSourceSKU{}).Where("1 = 1").
		Update("current_price", nil).Error; err != nil {
		t.Fatal(err)
	}

	out, err := f.svc.EstimateOrderCost(context.Background(), f.orderID)
	if err != nil {
		t.Fatalf("estimate: %v", err)
	}
	if out.ExchangeRate != nil || out.EstimatedCost != nil || out.GrossProfit != nil {
		t.Fatalf("expected no conversion without rate, got %+v", out)
	}
	if out.MissingLines != 1 || out.Lines[0].IssueCode != "price.missing" {
		t.Fatalf("expected price.missing line, got %+v", out.Lines)
	}
}

func TestEstimateOrderCostBatch(t *testing.T) {
	f := setupFixture(t)
	f.svc.Settings = mapSettings{"exchangeRate": "0.14"}
	if err := f.svc.DB.Model(&order.Order{}).Where("id = ?", f.orderID).
		Update("total_amount", 30.0).Error; err != nil {
		t.Fatal(err)
	}

	missing := uuid.New()
	out, err := f.svc.EstimateOrderCostBatch(context.Background(),
		[]uuid.UUID{f.orderID, f.orderID, missing})
	if err != nil {
		t.Fatalf("batch estimate: %v", err)
	}
	if len(out) != 1 {
		t.Fatalf("expected 1 summary (dedup + missing skipped), got %d", len(out))
	}
	sum, ok := out[f.orderID.String()]
	if !ok {
		t.Fatalf("missing summary for order, got %+v", out)
	}
	if sum.EstimatedCostCNY != 29.7 || sum.GrossProfit == nil || *sum.GrossProfit != 25.84 {
		t.Fatalf("unexpected summary %+v", sum)
	}
	if sum.MissingLines != 0 {
		t.Fatalf("expected no missing lines, got %+v", sum)
	}
}

func TestEstimateOrderCostCNYOrderDefaultsRateOne(t *testing.T) {
	f := setupFixture(t)
	if err := f.svc.DB.Model(&order.Order{}).Where("id = ?", f.orderID).
		Updates(map[string]any{"currency": "CNY", "total_amount": 50.0}).Error; err != nil {
		t.Fatal(err)
	}

	out, err := f.svc.EstimateOrderCost(context.Background(), f.orderID)
	if err != nil {
		t.Fatalf("estimate: %v", err)
	}
	if out.ExchangeRate == nil || *out.ExchangeRate != 1 {
		t.Fatalf("expected rate 1 for CNY order, got %+v", out.ExchangeRate)
	}
	if out.GrossProfit == nil || *out.GrossProfit != 20.3 {
		t.Fatalf("expected gross profit 20.3, got %+v", out.GrossProfit)
	}
}

// TestEstimateOrderCostBatchMatchesSingle asserts the batched estimator
// returns exactly what per-order EstimateOrderCost returns across pricing
// paths: current price, price-history fallback, missing mapping/source/SKU.
func TestEstimateOrderCostBatchMatchesSingle(t *testing.T) {
	f := setupFixture(t)
	f.svc.Settings = mapSettings{"exchangeRate": "0.14"}
	db := f.svc.DB

	// Order 2: mapping without current price, latest history price wins.
	sup := sourcing.Supplier{Platform: "1688", Name: "supplier-b", Status: "active"}
	if err := db.Create(&sup).Error; err != nil {
		t.Fatal(err)
	}
	productB := uuid.New()
	skuB := uuid.New()
	srcB := sourcing.ProductSource{
		ProductID: productB, SupplierID: sup.ID, IsPrimary: true, Priority: 10,
		Status: sourcing.SourceStatusActive, SourceOfferID: "222",
	}
	if err := db.Create(&srcB).Error; err != nil {
		t.Fatal(err)
	}
	mapB := sourcing.ProductSourceSKU{
		ProductSourceID: srcB.ID, LocalSKUID: skuB, ExternalSKUID: "ext-2",
		Currency: "CNY", Status: "active",
	}
	if err := db.Create(&mapB).Error; err != nil {
		t.Fatal(err)
	}
	old := time.Now().Add(-48 * time.Hour)
	recent := time.Now().Add(-1 * time.Hour)
	for _, h := range []sourcing.SourcePriceHistory{
		{SourceSKUID: mapB.ID, Price: 5.5, CapturedAt: old, CaptureSource: "manual"},
		{SourceSKUID: mapB.ID, Price: 7.7, CapturedAt: recent, CaptureSource: "manual"},
	} {
		if err := db.Create(&h).Error; err != nil {
			t.Fatal(err)
		}
	}
	o2 := order.Order{Platform: "tiktok", OrderNo: "SO-2", Status: "paid", Currency: "USD", TotalAmount: 40}
	if err := db.Create(&o2).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Create(&order.OrderItem{
		OrderID: o2.ID, ProductID: &productB, ProductSKUID: &skuB,
		SKUName: "blue / M", Quantity: 2,
	}).Error; err != nil {
		t.Fatal(err)
	}

	// Order 3: one unmatched line + one line whose product has no source.
	productC := uuid.New()
	skuC := uuid.New()
	o3 := order.Order{Platform: "shopee", OrderNo: "SO-3", Status: "paid", Currency: "CNY", TotalAmount: 15}
	if err := db.Create(&o3).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Create(&order.OrderItem{OrderID: o3.ID, SKUName: "unmatched", Quantity: 1}).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Create(&order.OrderItem{
		OrderID: o3.ID, ProductID: &productC, ProductSKUID: &skuC,
		SKUName: "no source", Quantity: 4,
	}).Error; err != nil {
		t.Fatal(err)
	}

	ids := []uuid.UUID{f.orderID, o2.ID, o3.ID}
	batch, err := f.svc.EstimateOrderCostBatch(context.Background(), ids)
	if err != nil {
		t.Fatalf("batch estimate: %v", err)
	}
	if len(batch) != 3 {
		t.Fatalf("expected 3 summaries, got %d", len(batch))
	}
	for _, id := range ids {
		single, err := f.svc.EstimateOrderCost(context.Background(), id)
		if err != nil {
			t.Fatalf("single estimate %s: %v", id, err)
		}
		want := CostEstimateSummary{
			OrderID:          single.OrderID,
			Currency:         single.Currency,
			TotalAmount:      single.TotalAmount,
			EstimatedCostCNY: single.EstimatedCostCNY,
			ExchangeRate:     single.ExchangeRate,
			EstimatedCost:    single.EstimatedCost,
			GrossProfit:      single.GrossProfit,
			MarginPercent:    single.MarginPercent,
			MissingLines:     single.MissingLines,
		}
		got, ok := batch[id.String()]
		if !ok {
			t.Fatalf("batch missing order %s", id)
		}
		if !reflect.DeepEqual(derefSummary(got), derefSummary(want)) {
			t.Fatalf("batch/single mismatch for %s:\n got %+v\nwant %+v", id, derefSummary(got), derefSummary(want))
		}
	}
	// Sanity: order 2 priced from the most recent history capture (7.7 × 2).
	if batch[o2.ID.String()].EstimatedCostCNY != 15.4 {
		t.Fatalf("expected history price 15.4, got %v", batch[o2.ID.String()].EstimatedCostCNY)
	}
	if batch[o3.ID.String()].MissingLines != 2 {
		t.Fatalf("expected 2 missing lines, got %+v", batch[o3.ID.String()])
	}
}

// derefSummary flattens pointer fields for value comparison.
func derefSummary(s CostEstimateSummary) map[string]any {
	deref := func(p *float64) any {
		if p == nil {
			return nil
		}
		return *p
	}
	return map[string]any{
		"orderId": s.OrderID, "currency": s.Currency, "totalAmount": s.TotalAmount,
		"estimatedCostCny": s.EstimatedCostCNY, "exchangeRate": deref(s.ExchangeRate),
		"estimatedCost": deref(s.EstimatedCost), "grossProfit": deref(s.GrossProfit),
		"marginPercent": deref(s.MarginPercent), "missingLines": s.MissingLines,
	}
}
