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
	"gorm.io/gorm"

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

// profitAcc accumulates one row with exact decimal conversion. Revenue /
// cost feed in either per order line or as SQL group sums (rate × Σamount ≡
// Σ(rate × amount) exactly, so both paths agree). Grouped paths set
// orderCount directly; per-order paths track distinct IDs.
type profitAcc struct {
	table       *fxrate.Table
	label       string
	platform    string
	orderIDs    map[uuid.UUID]bool
	orderCount  int64
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

// orders is the accumulated order count (grouped paths set orderCount, the
// per-order path collects distinct IDs).
func (a *profitAcc) orders() int64 {
	if a.orderCount > 0 {
		return a.orderCount
	}
	return int64(len(a.orderIDs))
}

// addPairCost folds one (product, sku, quantity) line group into the
// accumulator: each grouped line keeps its per-line round2(unit × quantity)
// contribution, so the grouped sum matches per-line accumulation exactly.
func (a *profitAcc) addPairCost(p linePairAgg, refs map[pairKey]*float64) {
	a.quantity += int64(p.Quantity) * p.N
	if u := refs[pairKeyOf(p.ProductID, p.ProductSKUID)]; u != nil {
		line := fxrate.AmountRat(round2(*u * float64(p.Quantity)))
		a.costCNY.Add(a.costCNY, new(big.Rat).Mul(line, new(big.Rat).SetInt64(p.N)))
	} else {
		a.missing += int(p.N)
	}
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
			per := new(big.Rat).Mul(fxrate.AmountRat(f.Value), big.NewRat(a.orders(), 1))
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
// Revenue / line groups are aggregated in SQL (GROUP BY); currency
// conversion, reference-cost resolution and profit math stay in Go on the
// exact group sums, so the numbers match per-row accumulation exactly.
func (s *Service) ProfitReport(c *gin.Context, dimension string, r DateRange) (*ProfitReportDTO, error) {
	return s.profitReport(c, dimension, r, profitMaxRows)
}

// profitReport builds the profit report; maxRows > 0 truncates the rows to
// that many (Truncated flags the cut), maxRows <= 0 keeps all rows (the CSV
// export path).
func (s *Service) profitReport(c *gin.Context, dimension string, r DateRange, maxRows int) (*ProfitReportDTO, error) {
	if dimension != DimensionOrder && dimension != DimensionProduct && dimension != DimensionShop {
		return nil, fmt.Errorf("dimension 仅支持 order / product / shop")
	}
	ctx := c.Request.Context()
	tenantID, err := adminperm.TenantIDFromGin(c)
	if err != nil {
		return nil, err
	}
	build := func() (*gorm.DB, error) {
		tx := s.DB.WithContext(ctx).Model(&order.Order{}).
			Where("created_at >= ? AND created_at < ? AND payment_status = ?", r.Start, r.End, order.PaymentPaid)
		tx, _, err := adminperm.ApplyTenantScope(c, tx)
		if err != nil {
			return nil, err
		}
		return adminperm.ApplyStoreScope(c, s.DB, tx, "shop_id")
	}

	table := s.fxTable(ctx, tenantID)
	fees := s.feeItems(ctx, tenantID)
	var cnyRate *big.Rat
	if rate, ok := table.Rate("CNY"); ok {
		cnyRate = rate
	}

	orderKeyExpr, lineKeyExpr := "", ""
	switch dimension {
	case DimensionShop:
		orderKeyExpr = "COALESCE(shop_id, '" + KeyNoShop + "')"
		lineKeyExpr = "COALESCE(o.shop_id, '" + KeyNoShop + "')"
	case DimensionProduct:
		lineKeyExpr = "COALESCE(oi.product_id, '" + KeyUnmatchedProduct + "')"
	}
	curAggs, err := orderCurrencyAggs(build, orderKeyExpr)
	if err != nil {
		return nil, err
	}
	pairAggs, err := s.linePairAggs(ctx, build, lineKeyExpr)
	if err != nil {
		return nil, err
	}
	refs, err := s.resolvePairRefs(ctx, pairAggs)
	if err != nil {
		return nil, err
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

	// Summary covers all scoped orders regardless of dimension: re-summing
	// the keyed groups is exact (both revenue and per-line costs are
	// additive).
	summaryAcc := newProfitAcc(table)
	for _, g := range curAggs {
		summaryAcc.addRevenue(g.Currency, g.Amount)
		summaryAcc.orderCount += g.N
	}
	for _, p := range pairAggs {
		summaryAcc.addPairCost(p, refs)
	}
	revBase, costCNY, feeBase, costBase, profit, margin := summaryAcc.finish(fees, cnyRate)
	out.Summary = ProfitSummary{
		OrderCount:            summaryAcc.orders(),
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

	var rows []ProfitRow
	switch dimension {
	case DimensionOrder:
		rows, out.Truncated, err = s.profitOrderRows(ctx, build, table, fees, cnyRate, refs, maxRows)
	case DimensionShop:
		rows, err = s.profitShopRows(ctx, build, table, fees, cnyRate, refs, curAggs, pairAggs)
	case DimensionProduct:
		rows, err = s.profitProductRows(ctx, build, table, fees, cnyRate, refs, pairAggs)
	}
	if err != nil {
		return nil, err
	}
	if dimension != DimensionOrder {
		sort.SliceStable(rows, func(i, j int) bool { return rows[i].RevenueBase > rows[j].RevenueBase })
		if maxRows > 0 && len(rows) > maxRows {
			rows = rows[:maxRows]
			out.Truncated = true
		}
	}
	out.Rows = rows
	return out, nil
}

// profitRowOf finishes an accumulator into its DTO row.
func profitRowOf(key string, a *profitAcc, fees []FeeItem, cnyRate *big.Rat) ProfitRow {
	rb, cc, fb, cb, p, m := a.finish(fees, cnyRate)
	return ProfitRow{
		Key:                   key,
		Label:                 a.label,
		Platform:              a.platform,
		OrderCount:            a.orders(),
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
	}
}

// profitOrderRows builds the order-dimension rows with per-line cost lookups
// against the shared pair resolution. maxRows > 0 materializes only the
// newest maxRows orders; maxRows <= 0 loads every scoped order in bounded
// keyset pages (the CSV export path).
func (s *Service) profitOrderRows(ctx context.Context, build func() (*gorm.DB, error), table *fxrate.Table, fees []FeeItem, cnyRate *big.Rat, refs map[pairKey]*float64, maxRows int) ([]ProfitRow, bool, error) {
	orders, truncated, err := s.profitScopedOrders(build, maxRows)
	if err != nil {
		return nil, false, err
	}
	orderIDs := make([]uuid.UUID, 0, len(orders))
	for _, o := range orders {
		orderIDs = append(orderIDs, o.ID)
	}
	items, err := s.loadOrderItems(ctx, orderIDs)
	if err != nil {
		return nil, false, err
	}
	itemsByOrder := make(map[uuid.UUID][]order.OrderItem, len(orders))
	for _, it := range items {
		itemsByOrder[it.OrderID] = append(itemsByOrder[it.OrderID], it)
	}
	rows := make([]ProfitRow, 0, len(orders))
	for _, o := range orders {
		a := newProfitAcc(table)
		a.label = o.OrderNo
		a.platform = o.Platform
		a.orderIDs[o.ID] = true
		a.addRevenue(o.Currency, o.TotalAmount)
		for _, it := range itemsByOrder[o.ID] {
			a.quantity += int64(it.Quantity)
			if u := refs[pairKeyOf(it.ProductID, it.ProductSKUID)]; u != nil {
				a.addCostCNY(round2(*u * float64(it.Quantity)))
			} else {
				a.missing++
			}
		}
		rows = append(rows, profitRowOf(o.ID.String(), a, fees, cnyRate))
	}
	return rows, truncated, nil
}

// profitShopRows builds the shop-dimension rows from the keyed SQL groups,
// in first-seen (newest order first) iteration order like the previous
// per-order accumulation.
func (s *Service) profitShopRows(ctx context.Context, build func() (*gorm.DB, error), table *fxrate.Table, fees []FeeItem, cnyRate *big.Rat, refs map[pairKey]*float64, curAggs []orderCurrencyAgg, pairAggs []linePairAgg) ([]ProfitRow, error) {
	metas, err := s.shopFirstSeen(ctx, build)
	if err != nil {
		return nil, err
	}
	shopIDs := make([]uuid.UUID, 0, len(metas))
	for _, m := range metas {
		if id, err := uuid.Parse(m.Key); err == nil {
			shopIDs = append(shopIDs, id)
		}
	}
	shopNames, err := s.shopLabelsByID(ctx, shopIDs)
	if err != nil {
		return nil, err
	}
	curByKey := map[string][]orderCurrencyAgg{}
	for _, g := range curAggs {
		curByKey[g.Key] = append(curByKey[g.Key], g)
	}
	pairByKey := map[string][]linePairAgg{}
	for _, p := range pairAggs {
		pairByKey[p.Key] = append(pairByKey[p.Key], p)
	}
	rows := make([]ProfitRow, 0, len(metas))
	for _, m := range metas {
		a := newProfitAcc(table)
		a.platform = m.Platform
		a.label = "未绑定店铺"
		if m.Key != KeyNoShop {
			if id, err := uuid.Parse(m.Key); err == nil && shopNames[id] != "" {
				a.label = shopNames[id]
			} else {
				a.label = "店铺 " + m.Key[:8]
			}
		}
		for _, g := range curByKey[m.Key] {
			a.addRevenue(g.Currency, g.Amount)
			a.orderCount += g.N
		}
		for _, p := range pairByKey[m.Key] {
			a.addPairCost(p, refs)
		}
		rows = append(rows, profitRowOf(m.Key, a, fees, cnyRate))
	}
	return rows, nil
}

// profitProductRows builds the product-dimension rows from the keyed SQL
// groups, in first-seen (newest order first) iteration order like the
// previous per-line accumulation.
func (s *Service) profitProductRows(ctx context.Context, build func() (*gorm.DB, error), table *fxrate.Table, fees []FeeItem, cnyRate *big.Rat, refs map[pairKey]*float64, pairAggs []linePairAgg) ([]ProfitRow, error) {
	metas, err := s.productFirstSeen(ctx, build)
	if err != nil {
		return nil, err
	}
	itemRev, err := s.itemCurrencyAggs(ctx, build)
	if err != nil {
		return nil, err
	}
	counts, err := s.productOrderCounts(ctx, build)
	if err != nil {
		return nil, err
	}
	revByKey := map[string][]itemCurrencyAgg{}
	for _, g := range itemRev {
		revByKey[g.Key] = append(revByKey[g.Key], g)
	}
	pairByKey := map[string][]linePairAgg{}
	for _, p := range pairAggs {
		pairByKey[p.Key] = append(pairByKey[p.Key], p)
	}
	rows := make([]ProfitRow, 0, len(metas))
	for _, m := range metas {
		a := newProfitAcc(table)
		if m.Key == KeyUnmatchedProduct {
			a.label = "未匹配本地商品"
			if m.Title != "" {
				a.label = "未匹配：" + m.Title
			}
		} else {
			a.label = m.Title
		}
		a.orderCount = counts[m.Key]
		for _, g := range revByKey[m.Key] {
			a.addRevenue(g.Currency, g.Amount)
		}
		for _, p := range pairByKey[m.Key] {
			a.addPairCost(p, refs)
		}
		rows = append(rows, profitRowOf(m.Key, a, fees, cnyRate))
	}
	return rows, nil
}

// profitScopedOrders loads the scoped orders newest-first: the top maxRows
// when maxRows > 0 (second result flags a cut), otherwise all of them in
// bounded keyset pages so no single query materializes an unbounded result.
func (s *Service) profitScopedOrders(build func() (*gorm.DB, error), maxRows int) ([]order.Order, bool, error) {
	const cols = "id, order_no, shop_id, platform, currency, total_amount, created_at"
	if maxRows > 0 {
		tx, err := build()
		if err != nil {
			return nil, false, err
		}
		var orders []order.Order
		if err := tx.Select(cols).
			Order("created_at DESC, id DESC").Limit(maxRows + 1).Find(&orders).Error; err != nil {
			return nil, false, err
		}
		truncated := len(orders) > maxRows
		if truncated {
			orders = orders[:maxRows]
		}
		return orders, truncated, nil
	}
	const batch = 1000
	var (
		all    []order.Order
		cursor *order.Order
	)
	for {
		tx, err := build()
		if err != nil {
			return nil, false, err
		}
		if cursor != nil {
			tx = tx.Where("created_at < ? OR (created_at = ? AND id < ?)", cursor.CreatedAt, cursor.CreatedAt, cursor.ID)
		}
		var page []order.Order
		if err := tx.Select(cols).
			Order("created_at DESC, id DESC").Limit(batch).Find(&page).Error; err != nil {
			return nil, false, err
		}
		all = append(all, page...)
		if len(page) < batch {
			return all, false, nil
		}
		last := page[len(page)-1]
		cursor = &last
	}
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

// shopLabelsByID resolves shop display names for the shop dimension.
func (s *Service) shopLabelsByID(ctx context.Context, ids []uuid.UUID) (map[uuid.UUID]string, error) {
	out := map[uuid.UUID]string{}
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
