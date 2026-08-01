package procurement

import (
	"context"
	"testing"

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
