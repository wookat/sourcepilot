package selection

import (
	"math"
	"testing"
)

func almostEq(a, b float64) bool { return math.Abs(a-b) < 0.005 }

func baseParams() ProfitParams {
	return ProfitParams{
		ExchangeRate:      7.2,
		CommissionPercent: 8,
		LogisticsBaseFee:  2,
		LogisticsPerKGFee: 8,
		LastMileFee:       1.5,
		ReturnRatePercent: 3,
		FixedCostPerOrder: 0.5,
	}
}

func TestComputeProfitHappyPath(t *testing.T) {
	res, err := ComputeProfit(ProfitInput{
		PurchaseCostCNY: 36, // 36/7.2 = 5.00 USD
		SellPrice:       19.99,
		WeightKG:        0.5, // shipping = 2 + 8*0.5 = 6.00
		Params:          baseParams(),
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !almostEq(res.PurchaseCost, 5.00) {
		t.Errorf("purchase = %v, want 5.00", res.PurchaseCost)
	}
	if !almostEq(res.ShippingCost, 6.00) {
		t.Errorf("shipping = %v, want 6.00", res.ShippingCost)
	}
	if !almostEq(res.LandedCost, 11.00) {
		t.Errorf("landed = %v, want 11.00", res.LandedCost)
	}
	// commission = 19.99*0.08 = 1.5992 → 1.60; returnLoss = 19.99*0.03 = 0.5997 → 0.60
	if !almostEq(res.CommissionFee, 1.60) {
		t.Errorf("commission = %v, want 1.60", res.CommissionFee)
	}
	if !almostEq(res.ReturnLoss, 0.60) {
		t.Errorf("returnLoss = %v, want 0.60", res.ReturnLoss)
	}
	// profit = 19.99 − 11.00 − 1.5992 − 1.5 − 0.5997 − 0.5 = 4.7911 → 4.79
	if !almostEq(res.EstProfit, 4.79) {
		t.Errorf("profit = %v, want 4.79", res.EstProfit)
	}
	// margin = 4.7911/19.99*100 = 23.9675 → 23.97
	if !almostEq(res.EstMarginPercent, 23.97) {
		t.Errorf("margin = %v, want 23.97", res.EstMarginPercent)
	}
}

func TestComputeProfitZeroWeightAndFees(t *testing.T) {
	res, err := ComputeProfit(ProfitInput{
		PurchaseCostCNY: 14.4,
		SellPrice:       10,
		Params:          ProfitParams{ExchangeRate: 7.2},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !almostEq(res.LandedCost, 2.00) {
		t.Errorf("landed = %v, want 2.00", res.LandedCost)
	}
	if !almostEq(res.EstProfit, 8.00) {
		t.Errorf("profit = %v, want 8.00", res.EstProfit)
	}
	if !almostEq(res.EstMarginPercent, 80.00) {
		t.Errorf("margin = %v, want 80.00", res.EstMarginPercent)
	}
}

func TestComputeProfitLossIsNegative(t *testing.T) {
	res, err := ComputeProfit(ProfitInput{
		PurchaseCostCNY: 144, // 20 USD purchase
		SellPrice:       9.99,
		Params:          baseParams(),
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res.EstProfit >= 0 {
		t.Errorf("profit = %v, want negative", res.EstProfit)
	}
	if res.EstMarginPercent >= 0 {
		t.Errorf("margin = %v, want negative", res.EstMarginPercent)
	}
}

func TestComputeProfitInvalidInputs(t *testing.T) {
	if _, err := ComputeProfit(ProfitInput{PurchaseCostCNY: 10, SellPrice: 10, Params: ProfitParams{ExchangeRate: 0}}); err == nil {
		t.Error("want error for zero exchange rate")
	}
	if _, err := ComputeProfit(ProfitInput{PurchaseCostCNY: -1, SellPrice: 10, Params: ProfitParams{ExchangeRate: 7}}); err == nil {
		t.Error("want error for negative purchase cost")
	}
	if _, err := ComputeProfit(ProfitInput{PurchaseCostCNY: 1, SellPrice: -1, Params: ProfitParams{ExchangeRate: 7}}); err == nil {
		t.Error("want error for negative sell price")
	}
}

func TestComputeProfitZeroSellPriceMarginZero(t *testing.T) {
	res, err := ComputeProfit(ProfitInput{PurchaseCostCNY: 7.2, SellPrice: 0, Params: ProfitParams{ExchangeRate: 7.2}})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res.EstMarginPercent != 0 {
		t.Errorf("margin = %v, want 0 when sell price is 0", res.EstMarginPercent)
	}
	if !almostEq(res.EstProfit, -1.00) {
		t.Errorf("profit = %v, want -1.00", res.EstProfit)
	}
}

func TestParamsNormalizeClamps(t *testing.T) {
	p := ProfitParams{
		ExchangeRate:      7.2,
		CommissionPercent: 120,
		ReturnRatePercent: -5,
		LogisticsBaseFee:  -1,
		LogisticsPerKGFee: -1,
		LastMileFee:       -1,
		FixedCostPerOrder: -1,
	}
	p.Normalize()
	if p.CommissionPercent != 95 {
		t.Errorf("commission = %v, want clamped to 95", p.CommissionPercent)
	}
	if p.ReturnRatePercent != 0 || p.LogisticsBaseFee != 0 || p.LogisticsPerKGFee != 0 || p.LastMileFee != 0 || p.FixedCostPerOrder != 0 {
		t.Errorf("negative fees not clamped: %+v", p)
	}
}

func TestComputeProfitNegativeWeightTreatedAsZero(t *testing.T) {
	p := baseParams()
	res, err := ComputeProfit(ProfitInput{PurchaseCostCNY: 7.2, SellPrice: 10, WeightKG: -3, Params: p})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !almostEq(res.ShippingCost, p.LogisticsBaseFee) {
		t.Errorf("shipping = %v, want base fee only %v", res.ShippingCost, p.LogisticsBaseFee)
	}
}

func TestSuggestPriceAchievesTargetMargin(t *testing.T) {
	p := baseParams()
	price, err := SuggestPrice(36, 0.5, 20, p)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	res, err := ComputeProfit(ProfitInput{PurchaseCostCNY: 36, SellPrice: price, WeightKG: 0.5, Params: p})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// Rounding may move margin slightly; allow 0.1pp tolerance.
	if math.Abs(res.EstMarginPercent-20) > 0.1 {
		t.Errorf("margin at suggested price = %v, want ≈20", res.EstMarginPercent)
	}
}

func TestSuggestPriceImpossibleTarget(t *testing.T) {
	p := ProfitParams{ExchangeRate: 7.2, CommissionPercent: 60, ReturnRatePercent: 30}
	if _, err := SuggestPrice(10, 0, 20, p); err == nil {
		t.Error("want error when commission+return+margin >= 100%")
	}
}

func TestSuggestPriceZeroRateError(t *testing.T) {
	if _, err := SuggestPrice(10, 0, 20, ProfitParams{}); err == nil {
		t.Error("want error for zero exchange rate")
	}
}
