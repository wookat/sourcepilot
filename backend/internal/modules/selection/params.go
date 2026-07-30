package selection

import (
	"strconv"
	"strings"
)

// Config keys live in settings group "selection" (all optional; defaults keep
// the mock pipeline runnable out of the box). Platform commission can be
// overridden per target platform with commission_percent_<platform>.
const settingsGroup = "selection"

// EngineConfig is the resolved runtime configuration for one task run.
type EngineConfig struct {
	Profit              ProfitParams
	SourceMatchProvider string // mock | crawler | open1688
	MarketPriceProvider string // mock (manual rows always win)
	TargetCurrency      string
	MinMarginPercent    float64 // advisory threshold surfaced to the LLM/UI
}

func parseF(m map[string]string, key string, def float64) float64 {
	v := strings.TrimSpace(m[key])
	if v == "" {
		return def
	}
	f, err := strconv.ParseFloat(v, 64)
	if err != nil {
		return def
	}
	return f
}

// ConfigFromSettings builds EngineConfig from the settings.selection map,
// falling back to settings.pricing conventions where sensible.
func ConfigFromSettings(sel, pricing map[string]string, targetPlatform string) EngineConfig {
	if sel == nil {
		sel = map[string]string{}
	}
	if pricing == nil {
		pricing = map[string]string{}
	}
	rate := parseF(sel, "exchange_rate", 0)
	if rate <= 0 {
		rate = parseF(pricing, "exchangeRate", 0)
	}
	if rate <= 0 {
		rate = 7.2
	}
	commission := parseF(sel, "commission_percent", -1)
	if p := strings.TrimSpace(targetPlatform); p != "" {
		if v := parseF(sel, "commission_percent_"+strings.ToLower(p), -1); v >= 0 {
			commission = v
		}
	}
	if commission < 0 {
		commission = parseF(pricing, "platformCommissionPercent", 8)
	}

	cfg := EngineConfig{
		Profit: ProfitParams{
			ExchangeRate:      rate,
			CommissionPercent: commission,
			LogisticsBaseFee:  parseF(sel, "logistics_base_fee", 2),
			LogisticsPerKGFee: parseF(sel, "logistics_fee_per_kg", 8),
			LastMileFee:       parseF(sel, "last_mile_fee", 1.5),
			ReturnRatePercent: parseF(sel, "return_rate_percent", 3),
			FixedCostPerOrder: parseF(sel, "fixed_cost_per_order", 0),
		},
		SourceMatchProvider: strings.TrimSpace(strings.ToLower(sel["source_match_provider"])),
		MarketPriceProvider: strings.TrimSpace(strings.ToLower(sel["market_price_provider"])),
		TargetCurrency:      strings.ToUpper(strings.TrimSpace(sel["target_currency"])),
		MinMarginPercent:    parseF(sel, "min_margin_percent", 15),
	}
	if cfg.SourceMatchProvider == "" {
		cfg.SourceMatchProvider = "mock"
	}
	if cfg.MarketPriceProvider == "" {
		cfg.MarketPriceProvider = "mock"
	}
	if cfg.TargetCurrency == "" {
		cfg.TargetCurrency = "USD"
	}
	cfg.Profit.Normalize()
	return cfg
}

// ApplyOverrides overlays per-task JSON param overrides onto the config.
func (c *EngineConfig) ApplyOverrides(o *TaskParamOverrides) {
	if c == nil || o == nil {
		return
	}
	if o.ExchangeRate != nil && *o.ExchangeRate > 0 {
		c.Profit.ExchangeRate = *o.ExchangeRate
	}
	if o.CommissionPercent != nil && *o.CommissionPercent >= 0 {
		c.Profit.CommissionPercent = *o.CommissionPercent
	}
	if o.LogisticsBaseFee != nil && *o.LogisticsBaseFee >= 0 {
		c.Profit.LogisticsBaseFee = *o.LogisticsBaseFee
	}
	if o.LogisticsPerKGFee != nil && *o.LogisticsPerKGFee >= 0 {
		c.Profit.LogisticsPerKGFee = *o.LogisticsPerKGFee
	}
	if o.LastMileFee != nil && *o.LastMileFee >= 0 {
		c.Profit.LastMileFee = *o.LastMileFee
	}
	if o.ReturnRatePercent != nil && *o.ReturnRatePercent >= 0 {
		c.Profit.ReturnRatePercent = *o.ReturnRatePercent
	}
	if o.FixedCostPerOrder != nil && *o.FixedCostPerOrder >= 0 {
		c.Profit.FixedCostPerOrder = *o.FixedCostPerOrder
	}
	if v := strings.TrimSpace(strings.ToLower(o.SourceMatchProvider)); v != "" {
		c.SourceMatchProvider = v
	}
	if v := strings.ToUpper(strings.TrimSpace(o.TargetCurrency)); v != "" {
		c.TargetCurrency = v
	}
	if o.MinMarginPercent != nil && *o.MinMarginPercent >= 0 {
		c.MinMarginPercent = *o.MinMarginPercent
	}
	c.Profit.Normalize()
}
