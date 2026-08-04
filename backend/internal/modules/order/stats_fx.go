package order

import (
	"context"
	"math/big"
	"sort"

	"github.com/trademind-ai/trademind/backend/internal/providers/fxrate"
)

// fxTable resolves the report base currency / manual rate table for one
// tenant. Never fails: without configuration it returns the default base
// currency with an empty table (all foreign currencies unconverted).
func (s *Service) fxTable(ctx context.Context, tenantID int64) *fxrate.Table {
	var reader fxrate.SettingsReader
	if s != nil && s.Settings != nil {
		reader = s.Settings
	}
	p := &fxrate.ManualProvider{Settings: reader}
	tab, err := p.Table(ctx, tenantID)
	if err != nil || tab == nil {
		return fxrate.NewTable(fxrate.DefaultBaseCurrency, nil)
	}
	return tab
}

// fxAccumulator sums converted paid amounts in base currency with exact
// decimal arithmetic and tracks currencies lacking a manual rate.
type fxAccumulator struct {
	table       *fxrate.Table
	sum         *big.Rat
	perCurrency map[string]*big.Rat
	unconverted map[string]bool
}

func newFxAccumulator(table *fxrate.Table) *fxAccumulator {
	return &fxAccumulator{
		table:       table,
		sum:         new(big.Rat),
		perCurrency: map[string]*big.Rat{},
		unconverted: map[string]bool{},
	}
}

// Add converts one paid amount; amounts without a rate are recorded as
// unconverted instead of being silently mis-counted.
func (a *fxAccumulator) Add(currency string, amount float64) {
	rate, ok := a.table.Rate(currency)
	if !ok {
		a.unconverted[currency] = true
		return
	}
	v := new(big.Rat).Mul(fxrate.AmountRat(amount), rate)
	a.sum.Add(a.sum, v)
	cur := a.perCurrency[currency]
	if cur == nil {
		cur = new(big.Rat)
		a.perCurrency[currency] = cur
	}
	cur.Add(cur, v)
}

func (a *fxAccumulator) Total() float64 { return fxrate.Round2(a.sum) }

func (a *fxAccumulator) BaseAmount(currency string) *float64 {
	r, ok := a.perCurrency[currency]
	if !ok {
		return nil
	}
	v := fxrate.Round2(r)
	return &v
}

func (a *fxAccumulator) Unconverted() []string {
	if len(a.unconverted) == 0 {
		return nil
	}
	out := make([]string, 0, len(a.unconverted))
	for c := range a.unconverted {
		out = append(out, c)
	}
	sort.Strings(out)
	return out
}
