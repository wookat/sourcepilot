// Package fx provides exchange-rate providers for the selection profit model.
// Business code must depend on Provider only; concrete sources (settings-fixed
// rate today, openexchangerates etc. later) plug in behind it.
package fx

import (
	"context"
	"fmt"
	"strconv"
	"strings"
)

// Provider resolves how many units of base currency one quote-currency unit costs.
// Example: Rate(ctx, "CNY", "USD") == 7.20 means 1 USD = 7.20 CNY.
type Provider interface {
	Name() string
	Rate(ctx context.Context, base, quote string) (float64, error)
}

// FixedProvider serves rates from a static table merged with settings overrides.
// It is the mock/default implementation until a real rate API is wired in.
type FixedProvider struct {
	// Rates maps "BASE/QUOTE" (upper case) to rate. Overrides defaults.
	Rates map[string]float64
}

var defaultCNYRates = map[string]float64{
	"CNY/USD": 7.20,
	"CNY/EUR": 7.85,
	"CNY/GBP": 9.10,
	"CNY/JPY": 0.0475,
	"CNY/SGD": 5.35,
	"CNY/MYR": 1.53,
	"CNY/THB": 0.205,
	"CNY/VND": 0.00028,
	"CNY/IDR": 0.00044,
	"CNY/PHP": 0.124,
	"CNY/CNY": 1,
}

// Name implements Provider.
func (p *FixedProvider) Name() string { return "fixed" }

// Rate implements Provider.
func (p *FixedProvider) Rate(_ context.Context, base, quote string) (float64, error) {
	base = strings.ToUpper(strings.TrimSpace(base))
	quote = strings.ToUpper(strings.TrimSpace(quote))
	if base == "" || quote == "" {
		return 0, fmt.Errorf("fx: base/quote required")
	}
	if base == quote {
		return 1, nil
	}
	key := base + "/" + quote
	if p != nil && p.Rates != nil {
		if r, ok := p.Rates[key]; ok && r > 0 {
			return r, nil
		}
	}
	if r, ok := defaultCNYRates[key]; ok && r > 0 {
		return r, nil
	}
	// Try inverse.
	inv := quote + "/" + base
	if p != nil && p.Rates != nil {
		if r, ok := p.Rates[inv]; ok && r > 0 {
			return 1 / r, nil
		}
	}
	if r, ok := defaultCNYRates[inv]; ok && r > 0 {
		return 1 / r, nil
	}
	return 0, fmt.Errorf("fx: no rate for %s", key)
}

// RatesFromSettings parses "fx_rate_usd=7.2"-style keys from a settings map
// into a Rates table for FixedProvider (base is always CNY).
func RatesFromSettings(plain map[string]string) map[string]float64 {
	out := map[string]float64{}
	for k, v := range plain {
		if !strings.HasPrefix(k, "fx_rate_") {
			continue
		}
		cur := strings.ToUpper(strings.TrimPrefix(k, "fx_rate_"))
		if cur == "" {
			continue
		}
		if f, err := strconv.ParseFloat(strings.TrimSpace(v), 64); err == nil && f > 0 {
			out["CNY/"+cur] = f
		}
	}
	return out
}
