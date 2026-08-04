package settings

import (
	"context"
	"testing"

	"github.com/trademind-ai/trademind/backend/internal/providers/fxrate"
)

func TestEnsureReportCurrencyDefaultsIdempotent(t *testing.T) {
	svc := newSettingsTestSvc(t)
	ctx := context.Background()
	for i := 0; i < 2; i++ {
		if err := EnsureReportCurrencyDefaults(ctx, svc.DB); err != nil {
			t.Fatal(err)
		}
	}
	var n int64
	if err := svc.DB.Model(&Setting{}).
		Where("tenant_id = ? AND group_key = ?", 0, fxrate.SettingsGroup).Count(&n).Error; err != nil {
		t.Fatal(err)
	}
	if n != 3 {
		t.Fatalf("expected 3 default rows, got %d", n)
	}
	m, err := svc.PlainByGroup(ctx, 0, fxrate.SettingsGroup)
	if err != nil {
		t.Fatal(err)
	}
	dto := reportCurrencyDTOFromPlain(m)
	if dto.BaseCurrency != "CNY" || dto.Provider != fxrate.ProviderManual || len(dto.Rates) != 0 {
		t.Fatalf("unexpected defaults: %+v", dto)
	}
}

func TestReportCurrencyDTOFromPlain(t *testing.T) {
	dto := reportCurrencyDTOFromPlain(map[string]string{
		fxrate.KeyBaseCurrency: "usd",
		fxrate.KeyRates:        `{"CNY":"0.140000","EUR":"1.08","BAD":"x"}`,
	})
	if dto.BaseCurrency != "USD" {
		t.Fatalf("base: %s", dto.BaseCurrency)
	}
	if len(dto.Rates) != 2 || dto.Rates[0].Currency != "CNY" || dto.Rates[0].Rate != "0.14" || dto.Rates[1].Currency != "EUR" {
		t.Fatalf("rates: %+v", dto.Rates)
	}
	// invalid base falls back to default
	dto = reportCurrencyDTOFromPlain(map[string]string{fxrate.KeyBaseCurrency: "china-yuan"})
	if dto.BaseCurrency != fxrate.DefaultBaseCurrency {
		t.Fatalf("fallback base: %s", dto.BaseCurrency)
	}
}
