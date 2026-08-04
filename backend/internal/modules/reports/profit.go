package reports

import (
	"context"
	"fmt"
	"math/big"
	"sort"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/trademind-ai/trademind/backend/internal/modules/order"
	"github.com/trademind-ai/trademind/backend/internal/pkg/adminperm"
	"github.com/trademind-ai/trademind/backend/internal/providers/fxrate"
)

// Profit dimensions.
const (
	DimensionOrder   = "order"
	DimensionProduct = "product"
	DimensionShop    = "shop"
)

// Row keys for lines that cannot be attributed.
const (
	KeyUnmatchedProduct = "__unmatched__"
	KeyNoShop           = "__no_shop__"
)

const profitMaxRows = 500

// MoneyByCurrency is an original-currency amount plus its base conversion
// (nil when the tenant has no manual rate for the currency: never faked).
type MoneyByCurrency struct {
	Currency   string   `json:"currency"`
	Amount     float64  `json:"amount"`
	BaseAmount *float64 `json:"baseAmount,omitempty"`
}

// ProfitRow is one aggregation row. RevenueBase sums only convertible
// amounts (UnconvertedCurrencies lists the rest). CostBase is nil when the
// CNY→base rate is missing; GrossProfitBase / MarginPercent are only
// computed when CostBase is known. MissingCostLines counts lines without a
// reference purchase price (cost is understated by those lines).
type ProfitRow struct {
	Key                   string            `json:"key"`
	Label                 string            `json:"label"`
	Platform              string            `json:"platform,omitempty"`
	OrderCount            int64             `json:"orderCount"`
	Quantity              int64             `json:"quantity,omitempty"`
	Revenue               []MoneyByCurrency `json:"revenue"`
	RevenueBase           float64           `json:"revenueBase"`
	UnconvertedCurrencies []string          `json:"unconvertedCurrencies,omitempty"`
	CostCNY               float64           `json:"costCny"`
	CostBase              *float64          `json:"costBase,omitempty"`
	MissingCostLines      int               `json:"missingCostLines"`
	FeeBase               float64           `json:"feeBase"`
	GrossProfitBase       *float64          `json:"grossProfitBase,omitempty"`
	MarginPercent         *float64          `json:"marginPercent,omitempty"`
}

// ProfitSummary aggregates the whole selection (computed over all orders,
// independent of the dimension grouping).
type ProfitSummary struct {
	OrderCount            int64             `json:"orderCount"`
	Revenue               []MoneyByCurrency `json:"revenue"`
	RevenueBase           float64           `json:"revenueBase"`
	UnconvertedCurrencies []string          `json:"unconvertedCurrencies,omitempty"`
	CostCNY               float64           `json:"costCny"`
	CostBase              *float64          `json:"costBase,omitempty"`
	MissingCostLines      int               `json:"missingCostLines"`
	FeeBase               float64           `json:"feeBase"`
	GrossProfitBase       *float64          `json:"grossProfitBase,omitempty"`
	MarginPercent         *float64          `json:"marginPercent,omitempty"`
}

// ProfitReportDTO is GET /reports/profit.
type ProfitReportDTO struct {
	GeneratedAt  string        `json:"generatedAt"`
	Dimension    string        `json:"dimension"`
	StartDate    string        `json:"startDate"`
	EndDate      string        `json:"endDate"`
	BaseCurrency string        `json:"baseCurrency"`
	FeeItems     []FeeItem     `json:"feeItems"`
	Summary      ProfitSummary `json:"summary"`
	Rows         []ProfitRow   `json:"rows"`
	Truncated    bool          `json:"truncated,omitempty"`
}

// profitAcc accumulates one row with exact decimal conversion.
type profitAcc struct {
	table       *fxrate.Table
	label       string
	platform    string
	orderIDs    map[uuid.UUID]bool
	quantity    int64
	byCurrency  map[string]*big.Rat // original amounts
	baseSum     *big.Rat            // convertible amounts in base
	baseByCur   map[string]*big.Rat
	unconverted map[string]bool
	costCNY     *big.Rat
	missing     int
}

func newProfitAcc(table *fxrate.Table) *profitAcc {
	return &profitAcc{
		table:       table,
		orderIDs:    map[uuid.UUID]bool{},
		byCurrency:  map[string]*big.Rat{},
		baseSum:     new(big.Rat),
		baseByCur:   map[string]*big.Rat{},
		unconverted: map[string]bool{},
		costCNY:     new(big.Rat),
	}
}

func (a *profitAcc) addRevenue(currency string, amount float64) {
	cur := a.byCurrency[currency]
	if cur == nil {
		cur = new(big.Rat)
		a.byCurrency[currency] = cur
	}
	cur.Add(cur, fxrate.AmountRat(amount))
	rate, ok := a.table.Rate(currency)
	if !ok {
		a.unconverted[currency] = true
		return
	}
	v := new(big.Rat).Mul(fxrate.AmountRat(amount), rate)
	a.baseSum.Add(a.baseSum, v)
	bc := a.baseByCur[currency]
	if bc == nil {
		bc = new(big.Rat)
		a.baseByCur[currency] = bc
	}
	bc.Add(bc, v)
}

func (a *profitAcc) addCostCNY(amount float64) {
	a.costCNY.Add(a.costCNY, fxrate.AmountRat(amount))
}

func (a *profitAcc) revenue() []MoneyByCurrency {
	out := make([]MoneyByCurrency, 0, len(a.byCurrency))
	for cur, amt := range a.byCurrency {
		m := MoneyByCurrency{Currency: cur, Amount: fxrate.Round2(amt)}
		if b, ok := a.baseByCur[cur]; ok {
			v := fxrate.Round2(b)
			m.BaseAmount = &v
		}
		out = append(out, m)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Currency < out[j].Currency })
	return out
}

func (a *profitAcc) unconvertedList() []string {
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

// finish computes fees / profit for a completed accumulator. cnyRate is the
// CNY→base rate (nil when the base has no CNY rate configured).
func (a *profitAcc) finish(fees []FeeItem, cnyRate *big.Rat) (revBase, costCNY, feeBase float64, costBase, profit, margin *float64) {
	revBase = fxrate.Round2(a.baseSum)
	costCNY = fxrate.Round2(a.costCNY)
	feeRat := new(big.Rat)
	for _, f := range fees {
		switch f.Mode {
		case FeeModePercent:
			pct := new(big.Rat).Mul(a.baseSum, fxrate.AmountRat(f.Value))
			feeRat.Add(feeRat, pct.Quo(pct, big.NewRat(100, 1)))
		case FeeModeFixedPerOrder:
			per := new(big.Rat).Mul(fxrate.AmountRat(f.Value), big.NewRat(int64(len(a.orderIDs)), 1))
			feeRat.Add(feeRat, per)
		}
	}
	feeBase = fxrate.Round2(feeRat)
	if cnyRate != nil {
		cb := fxrate.Round2(new(big.Rat).Mul(a.costCNY, cnyRate))
		costBase = &cb
		p := round2(revBase - cb - feeBase)
		profit = &p
		if revBase > 0 {
			m := round2(p / revBase * 100)
			margin = &m
		}
	}
	return
}

// ProfitReport aggregates paid orders in the range by the requested
// dimension. Scope matches the order list: current tenant, soft-deleted
// orders excluded, non-admin principals restricted to their granted shops.
func (s *Service) ProfitReport(c *gin.Context, dimension string, r DateRange) (*ProfitReportDTO, error) {
	if dimension != DimensionOrder && dimension != DimensionProduct && dimension != DimensionShop {
		return nil, fmt.Errorf("dimension 仅支持 order / product / shop")
	}
	ctx := c.Request.Context()

	tx := s.DB.WithContext(ctx).Model(&order.Order{}).
		Where("created_at >= ? AND created_at < ? AND payment_status = ?", r.Start, r.End, order.PaymentPaid)
	tx, tenantID, err := adminperm.ApplyTenantScope(c, tx)
	if err != nil {
		return nil, err
	}
	tx, err = adminperm.ApplyStoreScope(c, s.DB, tx, "shop_id")
	if err != nil {
		return nil, err
	}
	var orders []order.Order
	if err := tx.Select("id, order_no, shop_id, platform, currency, total_amount, created_at").
		Order("created_at DESC").Find(&orders).Error; err != nil {
		return nil, err
	}

	table := s.fxTable(ctx, tenantID)
	fees := s.feeItems(ctx, tenantID)
	var cnyRate *big.Rat
	if rate, ok := table.Rate("CNY"); ok {
		cnyRate = rate
	}

	orderIDs := make([]uuid.UUID, 0, len(orders))
	for _, o := range orders {
		orderIDs = append(orderIDs, o.ID)
	}
	items, err := s.loadOrderItems(ctx, orderIDs)
	if err != nil {
		return nil, err
	}
	costs, err := s.Proc.ResolveLineCostRefs(ctx, items)
	if err != nil {
		return nil, err
	}
	itemsByOrder := make(map[uuid.UUID][]order.OrderItem, len(orders))
	for _, it := range items {
		itemsByOrder[it.OrderID] = append(itemsByOrder[it.OrderID], it)
	}

	shopNames, err := s.shopLabels(ctx, orders, dimension)
	if err != nil {
		return nil, err
	}

	summaryAcc := newProfitAcc(table)
	rowsByKey := map[string]*profitAcc{}
	rowOrder := []string{}
	rowAcc := func(key, label, platform string) *profitAcc {
		a := rowsByKey[key]
		if a == nil {
			a = newProfitAcc(table)
			a.label = label
			a.platform = platform
			rowsByKey[key] = a
			rowOrder = append(rowOrder, key)
		}
		return a
	}

	for _, o := range orders {
		summaryAcc.orderIDs[o.ID] = true
		summaryAcc.addRevenue(o.Currency, o.TotalAmount)
		oItems := itemsByOrder[o.ID]
		for _, it := range oItems {
			cRef := costs[it.ID]
			if cRef.UnitCostCNY != nil {
				summaryAcc.addCostCNY(round2(*cRef.UnitCostCNY * float64(it.Quantity)))
			} else {
				summaryAcc.missing++
			}
		}

		switch dimension {
		case DimensionOrder:
			a := rowAcc(o.ID.String(), o.OrderNo, o.Platform)
			a.orderIDs[o.ID] = true
			a.addRevenue(o.Currency, o.TotalAmount)
			for _, it := range oItems {
				a.quantity += int64(it.Quantity)
				cRef := costs[it.ID]
				if cRef.UnitCostCNY != nil {
					a.addCostCNY(round2(*cRef.UnitCostCNY * float64(it.Quantity)))
				} else {
					a.missing++
				}
			}
		case DimensionShop:
			key, label, platform := KeyNoShop, "未绑定店铺", o.Platform
			if o.ShopID != nil {
				key = o.ShopID.String()
				if n, ok := shopNames[*o.ShopID]; ok && n != "" {
					label = n
				} else {
					label = "店铺 " + key[:8]
				}
			}
			a := rowAcc(key, label, platform)
			a.orderIDs[o.ID] = true
			a.addRevenue(o.Currency, o.TotalAmount)
			for _, it := range oItems {
				a.quantity += int64(it.Quantity)
				cRef := costs[it.ID]
				if cRef.UnitCostCNY != nil {
					a.addCostCNY(round2(*cRef.UnitCostCNY * float64(it.Quantity)))
				} else {
					a.missing++
				}
			}
		case DimensionProduct:
			for _, it := range oItems {
				key, label := KeyUnmatchedProduct, "未匹配本地商品"
				if it.ProductID != nil {
					key = it.ProductID.String()
					label = it.ProductTitle
				} else if it.ProductTitle != "" {
					label = "未匹配：" + it.ProductTitle
				}
				a := rowAcc(key, label, "")
				a.orderIDs[o.ID] = true
				a.quantity += int64(it.Quantity)
				a.addRevenue(o.Currency, it.TotalPrice)
				cRef := costs[it.ID]
				if cRef.UnitCostCNY != nil {
					a.addCostCNY(round2(*cRef.UnitCostCNY * float64(it.Quantity)))
				} else {
					a.missing++
				}
			}
		}
	}

	out := &ProfitReportDTO{
		GeneratedAt:  time.Now().UTC().Format(time.RFC3339),
		Dimension:    dimension,
		StartDate:    r.StartDate(),
		EndDate:      r.EndDate(),
		BaseCurrency: table.Base,
		FeeItems:     append([]FeeItem{}, fees...),
		Rows:         []ProfitRow{},
	}

	revBase, costCNY, feeBase, costBase, profit, margin := summaryAcc.finish(fees, cnyRate)
	out.Summary = ProfitSummary{
		OrderCount:            int64(len(summaryAcc.orderIDs)),
		Revenue:               summaryAcc.revenue(),
		RevenueBase:           revBase,
		UnconvertedCurrencies: summaryAcc.unconvertedList(),
		CostCNY:               costCNY,
		CostBase:              costBase,
		MissingCostLines:      summaryAcc.missing,
		FeeBase:               feeBase,
		GrossProfitBase:       profit,
		MarginPercent:         margin,
	}

	rows := make([]ProfitRow, 0, len(rowOrder))
	for _, key := range rowOrder {
		a := rowsByKey[key]
		rb, cc, fb, cb, p, m := a.finish(fees, cnyRate)
		rows = append(rows, ProfitRow{
			Key:                   key,
			Label:                 a.label,
			Platform:              a.platform,
			OrderCount:            int64(len(a.orderIDs)),
			Quantity:              a.quantity,
			Revenue:               a.revenue(),
			RevenueBase:           rb,
			UnconvertedCurrencies: a.unconvertedList(),
			CostCNY:               cc,
			CostBase:              cb,
			MissingCostLines:      a.missing,
			FeeBase:               fb,
			GrossProfitBase:       p,
			MarginPercent:         m,
		})
	}
	if dimension != DimensionOrder {
		sort.SliceStable(rows, func(i, j int) bool { return rows[i].RevenueBase > rows[j].RevenueBase })
	}
	if len(rows) > profitMaxRows {
		rows = rows[:profitMaxRows]
		out.Truncated = true
	}
	out.Rows = rows
	return out, nil
}

// loadOrderItems loads all line items for the given orders in bounded chunks.
func (s *Service) loadOrderItems(ctx context.Context, orderIDs []uuid.UUID) ([]order.OrderItem, error) {
	const chunk = 500
	var all []order.OrderItem
	for i := 0; i < len(orderIDs); i += chunk {
		j := i + chunk
		if j > len(orderIDs) {
			j = len(orderIDs)
		}
		var part []order.OrderItem
		if err := s.DB.WithContext(ctx).Where("order_id IN ?", orderIDs[i:j]).Find(&part).Error; err != nil {
			return nil, err
		}
		all = append(all, part...)
	}
	return all, nil
}

// shopLabels resolves shop display names for the shop dimension.
func (s *Service) shopLabels(ctx context.Context, orders []order.Order, dimension string) (map[uuid.UUID]string, error) {
	out := map[uuid.UUID]string{}
	if dimension != DimensionShop {
		return out, nil
	}
	idSet := map[uuid.UUID]struct{}{}
	ids := []uuid.UUID{}
	for _, o := range orders {
		if o.ShopID == nil {
			continue
		}
		if _, ok := idSet[*o.ShopID]; !ok {
			idSet[*o.ShopID] = struct{}{}
			ids = append(ids, *o.ShopID)
		}
	}
	if len(ids) == 0 {
		return out, nil
	}
	type shopRow struct {
		ID   uuid.UUID `gorm:"column:id"`
		Name string    `gorm:"column:name"`
	}
	var rows []shopRow
	if err := s.DB.WithContext(ctx).Table("shops").Select("id, shop_name AS name").Where("id IN ?", ids).Scan(&rows).Error; err != nil {
		return nil, err
	}
	for _, r := range rows {
		out[r.ID] = strings.TrimSpace(r.Name)
	}
	return out, nil
}
