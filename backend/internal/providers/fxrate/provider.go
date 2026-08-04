// Package fxrate resolves report base-currency exchange rates. The only
// implementation today is the manual rate table stored in settings group
// report_currency; the Provider interface reserves the slot for future live
// rate API providers without touching report aggregation code.
package fxrate

import (
	"context"
	"encoding/json"
	"math/big"
	"regexp"
	"strconv"
	"strings"
)

// Settings group / item keys for the manual report currency configuration.
const (
	SettingsGroup   = "report_currency"
	KeyProvider     = "provider"
	KeyBaseCurrency = "base_currency"
	KeyRates        = "rates"

	// ProviderManual is the only implemented provider (manual rate table).
	ProviderManual = "manual"

	// DefaultBaseCurrency is used when the tenant has not configured one.
	DefaultBaseCurrency = "CNY"

	// MaxRates caps the manual rate table size.
	MaxRates = 50
)

var currencyCodeRe = regexp.MustCompile(`^[A-Z]{3}$`)

// ValidCurrencyCode reports whether s is a 3-letter uppercase currency code.
func ValidCurrencyCode(s string) bool { return currencyCodeRe.MatchString(s) }

// ParseRate parses a positive decimal rate string exactly (no float round-trip).
func ParseRate(s string) (*big.Rat, bool) {
	s = strings.TrimSpace(s)
	if s == "" {
		return nil, false
	}
	// Reject exponent / hex forms that big.Rat would accept.
	if strings.ContainsAny(s, "eEpPxX/+- ") {
		return nil, false
	}
	r, ok := new(big.Rat).SetString(s)
	if !ok || r.Sign() <= 0 {
		return nil, false
	}
	// Guard absurd values (fat-finger protection).
	if r.Cmp(big.NewRat(1_000_000, 1)) > 0 {
		return nil, false
	}
	return r, true
}

// Table is a resolved base currency plus manual rates (1 unit of currency =
// rate units of base currency). Amount arithmetic is exact decimal via
// big.Rat; results round half-up to 2 decimals only at output.
type Table struct {
	Base  string
	rates map[string]*big.Rat
}

// NewTable builds a table; rates keys are normalized to upper case.
func NewTable(base string, rates map[string]*big.Rat) *Table {
	b := strings.ToUpper(strings.TrimSpace(base))
	if !ValidCurrencyCode(b) {
		b = DefaultBaseCurrency
	}
	m := make(map[string]*big.Rat, len(rates))
	for k, v := range rates {
		k = strings.ToUpper(strings.TrimSpace(k))
		if ValidCurrencyCode(k) && v != nil && v.Sign() > 0 {
			m[k] = v
		}
	}
	return &Table{Base: b, rates: m}
}

// Rate returns the currency→base rate. The base currency itself is always 1.
func (t *Table) Rate(currency string) (*big.Rat, bool) {
	if t == nil {
		return nil, false
	}
	c := strings.ToUpper(strings.TrimSpace(currency))
	if c == t.Base {
		return big.NewRat(1, 1), true
	}
	r, ok := t.rates[c]
	return r, ok
}

// RateStrings returns the configured rates as decimal strings (for DTOs).
func (t *Table) RateStrings() map[string]string {
	if t == nil {
		return map[string]string{}
	}
	out := make(map[string]string, len(t.rates))
	for k, v := range t.rates {
		out[k] = TrimRate(v)
	}
	return out
}

// AmountRat converts a money amount (decimal(18,4) column) to an exact
// rational using its 4-decimal literal, avoiding binary float artifacts.
func AmountRat(v float64) *big.Rat {
	r, ok := new(big.Rat).SetString(strconv.FormatFloat(v, 'f', 4, 64))
	if !ok {
		return new(big.Rat)
	}
	return r
}

// Round2 rounds a rational amount half-up to 2 decimals and returns float64
// (safe: 2-decimal values in report ranges are exactly representable enough
// for display/JSON).
func Round2(r *big.Rat) float64 {
	if r == nil {
		return 0
	}
	f, _ := strconv.ParseFloat(r.FloatString(2), 64)
	return f
}

// TrimRate renders a rate with up to 6 decimals, trimming trailing zeros.
func TrimRate(r *big.Rat) string {
	s := r.FloatString(6)
	s = strings.TrimRight(s, "0")
	return strings.TrimRight(s, ".")
}

// Provider resolves the rate table for one tenant.
type Provider interface {
	Table(ctx context.Context, tenantID int64) (*Table, error)
}

// SettingsReader decouples fxrate from the settings module implementation.
type SettingsReader interface {
	PlainByGroup(ctx context.Context, tenantID int64, groupKey string) (map[string]string, error)
}

// ManualProvider reads the manual rate table from settings group
// report_currency. Configuration is strictly per tenant: one tenant's base
// currency / rates never leak into another tenant's reports.
type ManualProvider struct {
	Settings SettingsReader
}

// Table implements Provider. It never fails hard: on missing/invalid
// configuration it returns the default base currency with an empty table so
// reports degrade to "unconverted" hints instead of erroring.
func (p *ManualProvider) Table(ctx context.Context, tenantID int64) (*Table, error) {
	if p == nil || p.Settings == nil {
		return NewTable(DefaultBaseCurrency, nil), nil
	}
	m, err := p.Settings.PlainByGroup(ctx, tenantID, SettingsGroup)
	if err != nil || len(m) == 0 {
		return NewTable(DefaultBaseCurrency, nil), nil
	}
	base := strings.TrimSpace(m[KeyBaseCurrency])
	rates := ParseRatesJSON(m[KeyRates])
	if base == "" && len(rates) == 0 {
		return NewTable(DefaultBaseCurrency, nil), nil
	}
	return NewTable(base, rates), nil
}

// ParseRatesJSON parses the stored rates JSON object ({"USD":"7.10"});
// invalid entries are dropped.
func ParseRatesJSON(raw string) map[string]*big.Rat {
	out := map[string]*big.Rat{}
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return out
	}
	var m map[string]string
	if err := json.Unmarshal([]byte(raw), &m); err != nil {
		return out
	}
	for k, v := range m {
		code := strings.ToUpper(strings.TrimSpace(k))
		if !ValidCurrencyCode(code) {
			continue
		}
		if r, ok := ParseRate(v); ok {
			out[code] = r
		}
	}
	return out
}
