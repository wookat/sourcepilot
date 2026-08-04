package fxrate

import (
	"context"
	"math/big"
	"testing"
)

func TestParseRate(t *testing.T) {
	cases := []struct {
		in string
		ok bool
	}{
		{"7.10", true}, {"0.052", true}, {"1", true},
		{"", false}, {"0", false}, {"-3", false}, {"1e3", false},
		{"abc", false}, {"1/3", false}, {"2000000", false},
	}
	for _, c := range cases {
		if _, ok := ParseRate(c.in); ok != c.ok {
			t.Fatalf("ParseRate(%q) ok=%v want %v", c.in, ok, c.ok)
		}
	}
}

func TestTableRateAndConvertExactDecimal(t *testing.T) {
	usd, _ := ParseRate("7.13")
	tab := NewTable("cny", map[string]*big.Rat{"usd": usd})
	if tab.Base != "CNY" {
		t.Fatalf("base normalize: %s", tab.Base)
	}
	if r, ok := tab.Rate("CNY"); !ok || r.Cmp(big.NewRat(1, 1)) != 0 {
		t.Fatal("base currency rate must be 1")
	}
	r, ok := tab.Rate("usd")
	if !ok {
		t.Fatal("usd rate missing")
	}
	// 12.5 USD * 7.13 = 89.125 → round half-up 89.13 (exact decimal path).
	got := Round2(new(big.Rat).Mul(AmountRat(12.5), r))
	if got != 89.13 {
		t.Fatalf("convert got %v want 89.13", got)
	}
	if _, ok := tab.Rate("EUR"); ok {
		t.Fatal("EUR should be unconverted")
	}
}

func TestParseRatesJSONDropsInvalid(t *testing.T) {
	m := ParseRatesJSON(`{"USD":"7.10","eur":"7.8","BAD":"x","TOOLONG":"1","JPY":"-1"}`)
	if len(m) != 2 || m["USD"] == nil || m["EUR"] == nil {
		t.Fatalf("unexpected parse result: %v", m)
	}
	if len(ParseRatesJSON("not json")) != 0 {
		t.Fatal("invalid json must yield empty map")
	}
}

type fakeSettings struct{ byTenant map[int64]map[string]string }

func (f *fakeSettings) PlainByGroup(_ context.Context, tenantID int64, _ string) (map[string]string, error) {
	return f.byTenant[tenantID], nil
}

func TestManualProviderTenantIsolation(t *testing.T) {
	p := &ManualProvider{Settings: &fakeSettings{byTenant: map[int64]map[string]string{
		0: {KeyBaseCurrency: "CNY", KeyRates: `{"USD":"7.10"}`},
		2: {KeyBaseCurrency: "USD", KeyRates: `{"CNY":"0.14"}`},
	}}}
	// Tenant without configuration gets the default base and empty table —
	// never another tenant's (or tenant 0's) rates.
	tab, err := p.Table(context.Background(), 5)
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := tab.Rate("USD"); ok || tab.Base != DefaultBaseCurrency {
		t.Fatalf("unconfigured tenant must not inherit other tenants' rates: %+v", tab)
	}
	// Each configured tenant reads its own table.
	tab, err = p.Table(context.Background(), 0)
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := tab.Rate("USD"); !ok || tab.Base != "CNY" {
		t.Fatalf("tenant 0 own table: %+v", tab)
	}
	tab, err = p.Table(context.Background(), 2)
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := tab.Rate("CNY"); !ok || tab.Base != "USD" {
		t.Fatalf("tenant 2 own table: %+v", tab)
	}

	empty := &ManualProvider{}
	tab, err = empty.Table(context.Background(), 1)
	if err != nil || tab.Base != DefaultBaseCurrency {
		t.Fatalf("nil settings must degrade to default base: %v %v", tab, err)
	}
}
