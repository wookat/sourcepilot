// Package logistics provides shipping-quote providers for the selection profit
// model. The mock implementation prices linearly by weight; real carrier APIs
// or imported rate tables plug in behind the same interface later.
package logistics

import (
	"context"
	"fmt"
	"strconv"
	"strings"
)

// QuoteRequest describes one shipment leg to price.
type QuoteRequest struct {
	// WeightKG is the chargeable weight in kilograms.
	WeightKG float64
	// TargetCountry is an ISO country/region code, e.g. "US".
	TargetCountry string
	// Currency is the currency the quote should be returned in.
	Currency string
}

// Quote is a priced shipment leg.
type Quote struct {
	Cost     float64 `json:"cost"`
	Currency string  `json:"currency"`
	Provider string  `json:"provider"`
}

// Provider prices a first-leg (head) shipment for the profit model.
type Provider interface {
	Name() string
	Quote(ctx context.Context, req QuoteRequest) (*Quote, error)
}

// LinearProvider is the mock implementation: cost = base + perKG × weight.
type LinearProvider struct {
	BaseFee  float64
	PerKGFee float64
	Currency string
}

// Name implements Provider.
func (p *LinearProvider) Name() string { return "linear_mock" }

// Quote implements Provider.
func (p *LinearProvider) Quote(_ context.Context, req QuoteRequest) (*Quote, error) {
	if p == nil {
		return nil, fmt.Errorf("logistics: provider nil")
	}
	w := req.WeightKG
	if w < 0 {
		return nil, fmt.Errorf("logistics: negative weight")
	}
	cur := strings.TrimSpace(req.Currency)
	if cur == "" {
		cur = p.Currency
	}
	if cur == "" {
		cur = "USD"
	}
	return &Quote{
		Cost:     p.BaseFee + p.PerKGFee*w,
		Currency: cur,
		Provider: p.Name(),
	}, nil
}

// LinearFromSettings builds a LinearProvider from a settings map with keys
// logistics_base_fee / logistics_fee_per_kg (falling back to defaults).
func LinearFromSettings(plain map[string]string) *LinearProvider {
	p := &LinearProvider{BaseFee: 2.0, PerKGFee: 8.0, Currency: "USD"}
	if v := strings.TrimSpace(plain["logistics_base_fee"]); v != "" {
		if f, err := strconv.ParseFloat(v, 64); err == nil && f >= 0 {
			p.BaseFee = f
		}
	}
	if v := strings.TrimSpace(plain["logistics_fee_per_kg"]); v != "" {
		if f, err := strconv.ParseFloat(v, 64); err == nil && f >= 0 {
			p.PerKGFee = f
		}
	}
	if v := strings.TrimSpace(plain["logistics_currency"]); v != "" {
		p.Currency = strings.ToUpper(v)
	}
	return p
}
