package selection

import (
	"fmt"
	"math"
)

// ProfitParams are the configurable levers of the profit model. All money
// fields except purchase cost are in the target (sell) currency.
// 口径与 modules/pricing 保持一致: landed cost 折算到目标币种后再扣佣金。
type ProfitParams struct {
	// ExchangeRate is how many CNY one target-currency unit costs (e.g. 7.2 CNY/USD).
	ExchangeRate float64 `json:"exchangeRate"`
	// CommissionPercent is the platform commission on sell price (0–95).
	CommissionPercent float64 `json:"commissionPercent"`
	// LogisticsBaseFee is the fixed head-leg shipping fee per order.
	LogisticsBaseFee float64 `json:"logisticsBaseFee"`
	// LogisticsPerKGFee is the head-leg shipping fee per kg.
	LogisticsPerKGFee float64 `json:"logisticsPerKgFee"`
	// LastMileFee is the尾程 delivery fee per order.
	LastMileFee float64 `json:"lastMileFee"`
	// ReturnRatePercent models expected return loss as a % of sell price (0–100).
	ReturnRatePercent float64 `json:"returnRatePercent"`
	// FixedCostPerOrder is any other per-order fixed cost (packaging, overhead).
	FixedCostPerOrder float64 `json:"fixedCostPerOrder"`
}

// ProfitInput is one candidate's economics.
type ProfitInput struct {
	// PurchaseCostCNY is the 1688 purchase price in CNY.
	PurchaseCostCNY float64
	// SellPrice is the (expected) overseas sell price in target currency.
	SellPrice float64
	// WeightKG is chargeable weight; 0 means base fee only.
	WeightKG float64
	Params   ProfitParams
}

// ProfitResult is the computed breakdown, all in target currency.
type ProfitResult struct {
	PurchaseCost      float64 `json:"purchaseCost"`  // converted to target currency
	ShippingCost      float64 `json:"shippingCost"`  // head-leg
	LandedCost        float64 `json:"landedCost"`    // purchase + head-leg
	CommissionFee     float64 `json:"commissionFee"` // sell × commission%
	LastMileFee       float64 `json:"lastMileFee"`
	ReturnLoss        float64 `json:"returnLoss"` // sell × returnRate%
	FixedCost         float64 `json:"fixedCost"`
	EstProfit         float64 `json:"estProfit"`
	EstMarginPercent  float64 `json:"estMarginPercent"`
	ExchangeRate      float64 `json:"exchangeRate"`
	CommissionPercent float64 `json:"commissionPercent"`
}

func round2(v float64) float64 { return math.Round(v*100) / 100 }

// Normalize clamps params into valid ranges.
func (p *ProfitParams) Normalize() {
	if p == nil {
		return
	}
	if p.CommissionPercent < 0 {
		p.CommissionPercent = 0
	}
	if p.CommissionPercent > 95 {
		p.CommissionPercent = 95
	}
	if p.ReturnRatePercent < 0 {
		p.ReturnRatePercent = 0
	}
	if p.ReturnRatePercent > 100 {
		p.ReturnRatePercent = 100
	}
	if p.LogisticsBaseFee < 0 {
		p.LogisticsBaseFee = 0
	}
	if p.LogisticsPerKGFee < 0 {
		p.LogisticsPerKGFee = 0
	}
	if p.LastMileFee < 0 {
		p.LastMileFee = 0
	}
	if p.FixedCostPerOrder < 0 {
		p.FixedCostPerOrder = 0
	}
}

// ComputeProfit evaluates the profit model:
//
//	purchase = purchaseCostCNY / exchangeRate
//	shipping = base + perKG × weight
//	landed   = purchase + shipping
//	profit   = sell − landed − sell×commission% − lastMile − sell×returnRate% − fixed
//	margin   = profit / sell × 100
func ComputeProfit(in ProfitInput) (*ProfitResult, error) {
	p := in.Params
	p.Normalize()
	if p.ExchangeRate <= 0 {
		return nil, fmt.Errorf("selection profit: exchange rate must be > 0")
	}
	if in.PurchaseCostCNY < 0 {
		return nil, fmt.Errorf("selection profit: negative purchase cost")
	}
	if in.SellPrice < 0 {
		return nil, fmt.Errorf("selection profit: negative sell price")
	}
	weight := in.WeightKG
	if weight < 0 {
		weight = 0
	}

	purchase := in.PurchaseCostCNY / p.ExchangeRate
	shipping := p.LogisticsBaseFee + p.LogisticsPerKGFee*weight
	landed := purchase + shipping
	commission := in.SellPrice * p.CommissionPercent / 100
	returnLoss := in.SellPrice * p.ReturnRatePercent / 100
	profit := in.SellPrice - landed - commission - p.LastMileFee - returnLoss - p.FixedCostPerOrder
	margin := 0.0
	if in.SellPrice > 0 {
		margin = profit / in.SellPrice * 100
	}

	return &ProfitResult{
		PurchaseCost:      round2(purchase),
		ShippingCost:      round2(shipping),
		LandedCost:        round2(landed),
		CommissionFee:     round2(commission),
		LastMileFee:       round2(p.LastMileFee),
		ReturnLoss:        round2(returnLoss),
		FixedCost:         round2(p.FixedCostPerOrder),
		EstProfit:         round2(profit),
		EstMarginPercent:  round2(margin),
		ExchangeRate:      p.ExchangeRate,
		CommissionPercent: p.CommissionPercent,
	}, nil
}

// SuggestPrice returns the minimum sell price that achieves targetMarginPercent:
//
//	sell × (1 − commission% − returnRate% − margin%) = landed + lastMile + fixed
func SuggestPrice(purchaseCostCNY, weightKG, targetMarginPercent float64, p ProfitParams) (float64, error) {
	p.Normalize()
	if p.ExchangeRate <= 0 {
		return 0, fmt.Errorf("selection profit: exchange rate must be > 0")
	}
	if targetMarginPercent < 0 {
		targetMarginPercent = 0
	}
	denom := 1 - p.CommissionPercent/100 - p.ReturnRatePercent/100 - targetMarginPercent/100
	if denom <= 0 {
		return 0, fmt.Errorf("selection profit: commission+return+margin >= 100%%")
	}
	if weightKG < 0 {
		weightKG = 0
	}
	landed := purchaseCostCNY/p.ExchangeRate + p.LogisticsBaseFee + p.LogisticsPerKGFee*weightKG
	price := (landed + p.LastMileFee + p.FixedCostPerOrder) / denom
	return round2(price), nil
}
