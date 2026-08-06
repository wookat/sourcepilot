package finance

import (
	"context"
	"math/big"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"gorm.io/gorm"

	"github.com/trademind-ai/trademind/backend/internal/modules/order"
	"github.com/trademind-ai/trademind/backend/internal/modules/reports"
	"github.com/trademind-ai/trademind/backend/internal/pkg/adminperm"
	"github.com/trademind-ai/trademind/backend/internal/providers/fxrate"
)

// Large-diff rule for the reconciliation workbench: an order is flagged when
// |actual - estimated| exceeds max(LargeDiffRatio × |estimated|, LargeDiffMinBase).
const (
	LargeDiffRatio   = 0.10
	LargeDiffMinBase = 10.0
)

// OrderFinance is the actual-vs-estimated finance view of one order (all
// *Base amounts in the tenant base currency; nil means一部分金额未折算).
type OrderFinance struct {
	OrderID          uuid.UUID  `json:"orderId"`
	OrderNo          string     `json:"orderNo"`
	Platform         string     `json:"platform,omitempty"`
	ShopID           *uuid.UUID `json:"shopId,omitempty"`
	ShopName         string     `json:"shopName,omitempty"`
	Currency         string     `json:"currency"`
	Receivable       float64    `json:"receivable"`
	Received         float64    `json:"received"`
	FeeTotal         float64    `json:"feeTotal"`
	DiffAmount       float64    `json:"diffAmount"`
	SettlementStatus string     `json:"settlementStatus"`

	ReceivedBase        *float64 `json:"receivedBase,omitempty"`
	ActualCostBase      *float64 `json:"actualCostBase,omitempty"`
	ExpenseBase         *float64 `json:"expenseBase,omitempty"`
	ActualProfitBase    *float64 `json:"actualProfitBase,omitempty"`
	EstimatedProfitBase *float64 `json:"estimatedProfitBase,omitempty"`
	ProfitDiffBase      *float64 `json:"profitDiffBase,omitempty"`
	LargeDiff           bool     `json:"largeDiff"`
	MissingActualLines  int      `json:"missingActualLines"`
	PaymentCount        int      `json:"paymentCount"`
	ExpenseCount        int      `json:"expenseCount"`
}

// OrderFinanceSummary is the order-detail finance panel payload.
type OrderFinanceSummary struct {
	BaseCurrency string        `json:"baseCurrency"`
	Finance      OrderFinance  `json:"finance"`
	Payments     []PaymentDTO  `json:"payments"`
	Expenses     []expenseDTO  `json:"expenses"`
	ExpenseTypes []ExpenseType `json:"expenseTypes"`
}

type expenseDTO struct {
	OrderExpense
	TypeLabel string `json:"typeLabel,omitempty"`
}

// fxTable loads the tenant fx table, degrading to an empty CNY table (never
// fabricates rates: unconvertible amounts stay nil upstream).
func (s *Service) fxTable(ctx context.Context, tenantID int64) *fxrate.Table {
	p := &fxrate.ManualProvider{Settings: s.Settings}
	t, err := p.Table(ctx, tenantID)
	if err != nil || t == nil {
		return fxrate.NewTable("", nil)
	}
	return t
}

// OrderSummary builds the order-detail finance panel: settlement status,
// payment / expense rows and actual-vs-estimated profit for one order.
func (s *Service) OrderSummary(c *gin.Context, orderID uuid.UUID) (*OrderFinanceSummary, error) {
	o, err := s.loadScopedOrder(c, orderID)
	if err != nil {
		return nil, err
	}
	ctx := c.Request.Context()
	rows, table, err := s.computeOrderFinance(ctx, o.TenantID, []order.Order{*o})
	if err != nil {
		return nil, err
	}
	fin := OrderFinance{OrderID: o.ID, OrderNo: o.OrderNo, Currency: o.Currency, Receivable: o.TotalAmount, SettlementStatus: SettlementUnpaid}
	if len(rows) > 0 {
		fin = rows[0]
	}
	var payments []PaymentRecord
	if err := s.DB.WithContext(ctx).Where("order_id = ?", o.ID).
		Order("received_at DESC, created_at DESC").Find(&payments).Error; err != nil {
		return nil, err
	}
	pdtos := make([]PaymentDTO, 0, len(payments))
	for _, p := range payments {
		pdtos = append(pdtos, PaymentDTO{
			PaymentRecord: p, OrderNo: o.OrderNo, OrderAmount: o.TotalAmount, OrderCurrency: o.Currency,
			SettlementStatus: fin.SettlementStatus, DiffAmount: fin.DiffAmount,
		})
	}
	var expenses []OrderExpense
	if err := s.DB.WithContext(ctx).Where("order_id = ?", o.ID).
		Order("created_at DESC").Find(&expenses).Error; err != nil {
		return nil, err
	}
	labels := s.expenseTypeLabels(ctx, o.TenantID)
	edtos := make([]expenseDTO, 0, len(expenses))
	for _, e := range expenses {
		edtos = append(edtos, expenseDTO{OrderExpense: e, TypeLabel: labels[e.TypeCode]})
	}
	return &OrderFinanceSummary{
		BaseCurrency: table.Base,
		Finance:      fin,
		Payments:     pdtos,
		Expenses:     edtos,
		ExpenseTypes: s.ExpenseTypes(ctx, o.TenantID),
	}, nil
}

// aggChunk bounds the order-id IN lists of the grouped finance queries.
const aggChunk = 1000

func chunkOrderIDs(ids []uuid.UUID) [][]uuid.UUID {
	var out [][]uuid.UUID
	for i := 0; i < len(ids); i += aggChunk {
		j := i + aggChunk
		if j > len(ids) {
			j = len(ids)
		}
		out = append(out, ids[i:j])
	}
	return out
}

// paymentAgg is one (order, currency) payment aggregate. Amount / Fee are
// exact decimal(18,4) sums (sums of 4-decimal values stay 4-decimal, so the
// float64 round-trip through AmountRat is lossless).
type paymentAgg struct {
	OrderID  uuid.UUID `gorm:"column:order_id"`
	Currency string    `gorm:"column:currency"`
	Amount   float64   `gorm:"column:amount"`
	Fee      float64   `gorm:"column:fee"`
	N        int       `gorm:"column:n"`
}

// paymentAggs sums payments per (order, currency) in SQL.
func (s *Service) paymentAggs(ctx context.Context, tenantID int64, orderIDs []uuid.UUID) (map[uuid.UUID][]paymentAgg, error) {
	out := map[uuid.UUID][]paymentAgg{}
	for _, chunk := range chunkOrderIDs(orderIDs) {
		var rows []paymentAgg
		if err := s.DB.WithContext(ctx).Model(&PaymentRecord{}).
			Select("order_id, currency, SUM(amount) AS amount, SUM(fee_amount) AS fee, COUNT(*) AS n").
			Where("tenant_id = ? AND order_id IN ?", tenantID, chunk).
			Group("order_id, currency").Scan(&rows).Error; err != nil {
			return nil, err
		}
		for _, r := range rows {
			out[r.OrderID] = append(out[r.OrderID], r)
		}
	}
	return out, nil
}

// expenseAgg is one (order, currency) order-expense aggregate.
type expenseAgg struct {
	OrderID  uuid.UUID `gorm:"column:order_id"`
	Currency string    `gorm:"column:currency"`
	Amount   float64   `gorm:"column:amount"`
	N        int       `gorm:"column:n"`
}

// expenseAggs sums order-level expenses per (order, currency) in SQL.
func (s *Service) expenseAggs(ctx context.Context, tenantID int64, orderIDs []uuid.UUID) (map[uuid.UUID][]expenseAgg, error) {
	out := map[uuid.UUID][]expenseAgg{}
	for _, chunk := range chunkOrderIDs(orderIDs) {
		var rows []expenseAgg
		if err := s.DB.WithContext(ctx).Model(&OrderExpense{}).
			Select("order_id, currency, SUM(amount) AS amount, COUNT(*) AS n").
			Where("tenant_id = ? AND order_id IN ?", tenantID, chunk).
			Group("order_id, currency").Scan(&rows).Error; err != nil {
			return nil, err
		}
		for _, r := range rows {
			out[r.OrderID] = append(out[r.OrderID], r)
		}
	}
	return out, nil
}

// actualCostAgg is one (order, purchase currency) actual-cost aggregate.
// Priced counts lines with a registered actual price (Amount sums those
// lines' price × quantity); Missing counts lines without one.
type actualCostAgg struct {
	OrderID  uuid.UUID `gorm:"column:order_id"`
	Currency string    `gorm:"column:currency"`
	Amount   float64   `gorm:"column:amount"`
	Priced   int       `gorm:"column:priced"`
	Missing  int       `gorm:"column:missing"`
}

// actualCostAggs sums purchase lines attributed to the sales orders per
// (order, purchase-order currency) in SQL.
func (s *Service) actualCostAggs(ctx context.Context, tenantID int64, orderIDs []uuid.UUID) (map[uuid.UUID][]actualCostAgg, error) {
	out := map[uuid.UUID][]actualCostAgg{}
	for _, chunk := range chunkOrderIDs(orderIDs) {
		var rows []actualCostAgg
		if err := s.DB.WithContext(ctx).
			Table("purchase_order_items poi").
			Select("poi.sales_order_id AS order_id, po.currency AS currency, "+
				"SUM(CASE WHEN poi.actual_price IS NULL THEN 0 ELSE poi.actual_price * poi.quantity END) AS amount, "+
				"SUM(CASE WHEN poi.actual_price IS NULL THEN 0 ELSE 1 END) AS priced, "+
				"SUM(CASE WHEN poi.actual_price IS NULL THEN 1 ELSE 0 END) AS missing").
			Joins("JOIN purchase_orders po ON po.id = poi.purchase_order_id AND po.deleted_at IS NULL").
			Where("poi.tenant_id = ? AND poi.sales_order_id IN ? AND po.status <> ?", tenantID, chunk, "cancelled").
			Group("poi.sales_order_id, po.currency").Scan(&rows).Error; err != nil {
			return nil, err
		}
		for _, r := range rows {
			out[r.OrderID] = append(out[r.OrderID], r)
		}
	}
	return out, nil
}

// itemPairKey identifies a (product, sku) reference-cost pair: the shared
// procurement resolver's result only depends on this pair, so one synthetic
// line per distinct pair resolves costs for every grouped line exactly.
type itemPairKey struct {
	product uuid.UUID
	sku     uuid.UUID
}

func itemPairKeyOf(productID, skuID *uuid.UUID) itemPairKey {
	k := itemPairKey{}
	if productID != nil {
		k.product = *productID
	}
	if skuID != nil {
		k.sku = *skuID
	}
	return k
}

// orderItemAgg is one (order, product, sku) line group used by the estimated
// profit path (the reference cost only depends on the product/sku pair, so
// same-pair lines aggregate without changing the estimate).
type orderItemAgg struct {
	OrderID      uuid.UUID  `gorm:"column:order_id"`
	ProductID    *uuid.UUID `gorm:"column:product_id"`
	ProductSKUID *uuid.UUID `gorm:"column:product_sku_id"`
	Quantity     int        `gorm:"column:quantity"`
}

// orderItemAggs groups order lines per (order, product, sku) in SQL.
func (s *Service) orderItemAggs(ctx context.Context, orderIDs []uuid.UUID) (map[uuid.UUID][]order.OrderItem, error) {
	out := map[uuid.UUID][]order.OrderItem{}
	for _, chunk := range chunkOrderIDs(orderIDs) {
		var rows []orderItemAgg
		if err := s.DB.WithContext(ctx).Model(&order.OrderItem{}).
			Select("order_id, product_id, product_sku_id, SUM(quantity) AS quantity").
			Where("order_id IN ?", chunk).
			Group("order_id, product_id, product_sku_id").Scan(&rows).Error; err != nil {
			return nil, err
		}
		for _, r := range rows {
			it := order.OrderItem{OrderID: r.OrderID, ProductID: r.ProductID, ProductSKUID: r.ProductSKUID, Quantity: r.Quantity}
			it.ID = uuid.New()
			out[r.OrderID] = append(out[r.OrderID], it)
		}
	}
	return out, nil
}

// computeOrderFinance derives per-order settlement + actual/estimated profit
// for orders. Payments / expenses / actual costs / order lines are summed per
// (order, currency) group in SQL; conversion and profit math stay in big.Rat
// on the group sums (sums of fixed-decimal columns convert losslessly, and
// rate × Σamount ≡ Σ(rate × amount) exactly). Currencies missing from the fx
// table still yield nil Base amounts, never fabricated values.
func (s *Service) computeOrderFinance(ctx context.Context, tenantID int64, orders []order.Order) ([]OrderFinance, *fxrate.Table, error) {
	table := s.fxTable(ctx, tenantID)
	ids := make([]uuid.UUID, 0, len(orders))
	for _, o := range orders {
		ids = append(ids, o.ID)
	}
	payByOrder, err := s.paymentAggs(ctx, tenantID, ids)
	if err != nil {
		return nil, nil, err
	}
	expByOrder, err := s.expenseAggs(ctx, tenantID, ids)
	if err != nil {
		return nil, nil, err
	}
	costs, err := s.actualCostAggs(ctx, tenantID, ids)
	if err != nil {
		return nil, nil, err
	}
	itemsByOrder, err := s.orderItemAggs(ctx, ids)
	if err != nil {
		return nil, nil, err
	}
	refByPair := map[itemPairKey]*float64{}
	if s.Proc != nil {
		seen := map[itemPairKey]bool{}
		synth := make([]order.OrderItem, 0)
		byID := map[uuid.UUID]itemPairKey{}
		for _, its := range itemsByOrder {
			for _, it := range its {
				k := itemPairKeyOf(it.ProductID, it.ProductSKUID)
				if seen[k] {
					continue
				}
				seen[k] = true
				pair := order.OrderItem{ProductID: it.ProductID, ProductSKUID: it.ProductSKUID, Quantity: 1}
				pair.ID = uuid.New()
				byID[pair.ID] = k
				synth = append(synth, pair)
			}
		}
		refs, err := s.Proc.ResolveLineCostRefs(ctx, synth)
		if err == nil {
			for id, ref := range refs {
				refByPair[byID[id]] = ref.UnitCostCNY
			}
		}
	}
	feeItems := reports.LoadFeeItems(ctx, s.Settings, tenantID)

	shopIDs := []uuid.UUID{}
	seenShop := map[uuid.UUID]bool{}
	for _, o := range orders {
		if o.ShopID != nil && !seenShop[*o.ShopID] {
			seenShop[*o.ShopID] = true
			shopIDs = append(shopIDs, *o.ShopID)
		}
	}
	shopNames, err := s.shopNamesByID(ctx, shopIDs)
	if err != nil {
		return nil, nil, err
	}

	out := make([]OrderFinance, 0, len(orders))
	for _, o := range orders {
		fin := OrderFinance{
			OrderID:    o.ID,
			OrderNo:    o.OrderNo,
			Platform:   o.Platform,
			ShopID:     o.ShopID,
			Currency:   o.Currency,
			Receivable: o.TotalAmount,
		}
		if o.ShopID != nil {
			fin.ShopName = shopNames[*o.ShopID]
		}

		// Received: order-currency sum for settlement, base sum for profit.
		recvSum := new(big.Rat)
		recvBase := new(big.Rat)
		feeSum := new(big.Rat)
		recvConvertible := true
		for _, p := range payByOrder[o.ID] {
			recvSum.Add(recvSum, fxrate.AmountRat(p.Amount))
			feeSum.Add(feeSum, fxrate.AmountRat(p.Fee))
			net := new(big.Rat).Sub(fxrate.AmountRat(p.Amount), fxrate.AmountRat(p.Fee))
			if rate, ok := table.Rate(p.Currency); ok {
				recvBase.Add(recvBase, new(big.Rat).Mul(net, rate))
			} else {
				recvConvertible = false
			}
			fin.PaymentCount += p.N
		}
		fin.Received = fxrate.Round2(recvSum)
		fin.FeeTotal = fxrate.Round2(feeSum)
		fin.SettlementStatus, fin.DiffAmount = settlementOf(o.TotalAmount, fin.Received)

		// Actual purchase cost (registered actual prices only).
		costBase := new(big.Rat)
		costConvertible := true
		for _, cr := range costs[o.ID] {
			fin.MissingActualLines += cr.Missing
			if cr.Priced == 0 {
				continue
			}
			cur := strings.ToUpper(strings.TrimSpace(cr.Currency))
			if cur == "" {
				cur = "CNY"
			}
			if rate, ok := table.Rate(cur); ok {
				costBase.Add(costBase, new(big.Rat).Mul(fxrate.AmountRat(cr.Amount), rate))
			} else {
				costConvertible = false
			}
		}

		// Recorded expenses.
		expBase := new(big.Rat)
		expConvertible := true
		for _, e := range expByOrder[o.ID] {
			if rate, ok := table.Rate(e.Currency); ok {
				expBase.Add(expBase, new(big.Rat).Mul(fxrate.AmountRat(e.Amount), rate))
			} else {
				expConvertible = false
			}
			fin.ExpenseCount += e.N
		}

		if recvConvertible {
			v := fxrate.Round2(recvBase)
			fin.ReceivedBase = &v
		}
		if costConvertible {
			v := fxrate.Round2(costBase)
			fin.ActualCostBase = &v
		}
		if expConvertible {
			v := fxrate.Round2(expBase)
			fin.ExpenseBase = &v
		}
		if recvConvertible && costConvertible && expConvertible {
			profit := new(big.Rat).Sub(recvBase, costBase)
			profit.Sub(profit, expBase)
			v := fxrate.Round2(profit)
			fin.ActualProfitBase = &v
		}

		// Estimated profit: reference-cost口径 (same as the profit report).
		fin.EstimatedProfitBase = estimateProfitBase(o, itemsByOrder[o.ID], refByPair, feeItems, table)

		if fin.ActualProfitBase != nil && fin.EstimatedProfitBase != nil {
			d := fxrate.Round2(new(big.Rat).Sub(fxrate.AmountRat(*fin.ActualProfitBase), fxrate.AmountRat(*fin.EstimatedProfitBase)))
			fin.ProfitDiffBase = &d
			threshold := LargeDiffMinBase
			if r := *fin.EstimatedProfitBase * LargeDiffRatio; r > threshold || -r > threshold {
				if r < 0 {
					r = -r
				}
				threshold = r
			}
			if d > threshold || d < -threshold {
				fin.LargeDiff = true
			}
		}
		out = append(out, fin)
	}
	return out, table, nil
}

// estimateProfitBase mirrors the profit report口径: revenueBase − reference
// cost (CNY, converted) − configured估算 fees. nil when a needed rate is
// missing.
func estimateProfitBase(o order.Order, items []order.OrderItem, refByPair map[itemPairKey]*float64, feeItems []reports.FeeItem, table *fxrate.Table) *float64 {
	rate, ok := table.Rate(o.Currency)
	if !ok {
		return nil
	}
	revBase := new(big.Rat).Mul(fxrate.AmountRat(o.TotalAmount), rate)
	cnyRate, ok := table.Rate("CNY")
	if !ok {
		return nil
	}
	costCNY := new(big.Rat)
	for _, it := range items {
		if p := refByPair[itemPairKeyOf(it.ProductID, it.ProductSKUID)]; p != nil {
			costCNY.Add(costCNY, new(big.Rat).Mul(fxrate.AmountRat(*p), new(big.Rat).SetInt64(int64(it.Quantity))))
		}
	}
	costBase := new(big.Rat).Mul(costCNY, cnyRate)
	fee := new(big.Rat)
	for _, f := range feeItems {
		switch f.Mode {
		case reports.FeeModePercent:
			part := new(big.Rat).Mul(revBase, fxrate.AmountRat(f.Value))
			part.Quo(part, big.NewRat(100, 1))
			fee.Add(fee, part)
		case reports.FeeModeFixedPerOrder:
			fee.Add(fee, fxrate.AmountRat(f.Value))
		}
	}
	profit := new(big.Rat).Sub(revBase, costBase)
	profit.Sub(profit, fee)
	v := fxrate.Round2(profit)
	return &v
}

// orderLoadBatch bounds each keyset page of the scoped order load.
const orderLoadBatch = 1000

// scopedOrdersInRange loads all paid orders in [start, end] under tenant +
// store scope (finance口径 matches the profit report:已付款), in bounded
// keyset pages so no single query materializes an unbounded result. Only the
// columns the finance views read are selected; line items are aggregated
// separately.
func (s *Service) scopedOrdersInRange(c *gin.Context, start, end time.Time) ([]order.Order, int64, error) {
	build := func() (*gorm.DB, int64, error) {
		tx := s.DB.WithContext(c.Request.Context()).Model(&order.Order{}).
			Where("created_at >= ? AND created_at < ?", start, end.AddDate(0, 0, 1)).
			Where("payment_status = ?", order.PaymentPaid)
		tx, tid, err := adminperm.ApplyTenantScope(c, tx)
		if err != nil {
			return nil, 0, err
		}
		tx, err = adminperm.ApplyStoreScope(c, s.DB, tx, "shop_id")
		return tx, tid, err
	}
	var (
		all    []order.Order
		tid    int64
		cursor *order.Order
	)
	for {
		tx, id, err := build()
		if err != nil {
			return nil, 0, err
		}
		tid = id
		if cursor != nil {
			tx = tx.Where("created_at < ? OR (created_at = ? AND id < ?)", cursor.CreatedAt, cursor.CreatedAt, cursor.ID)
		}
		var page []order.Order
		if err := tx.Select("id, order_no, platform, shop_id, currency, total_amount, created_at").
			Order("created_at DESC, id DESC").Limit(orderLoadBatch).Find(&page).Error; err != nil {
			return nil, 0, err
		}
		all = append(all, page...)
		if len(page) < orderLoadBatch {
			return all, tid, nil
		}
		last := page[len(page)-1]
		cursor = &last
	}
}
