package finance

import (
	"context"
	"math/big"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

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
	var full order.Order
	if err := s.DB.WithContext(ctx).Preload("Items").First(&full, "id = ?", o.ID).Error; err != nil {
		return nil, err
	}
	rows, table, err := s.computeOrderFinance(ctx, o.TenantID, []order.Order{full})
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

// actualCostRow is one purchase line attributed to a sales order.
type actualCostRow struct {
	SalesOrderID uuid.UUID `gorm:"column:sales_order_id"`
	Quantity     int       `gorm:"column:quantity"`
	ActualPrice  *float64  `gorm:"column:actual_price"`
	Currency     string    `gorm:"column:currency"`
}

// actualCosts loads purchase lines linked to the sales orders, joining the
// purchase order for its currency (empty degrades to CNY).
func (s *Service) actualCosts(ctx context.Context, tenantID int64, orderIDs []uuid.UUID) (map[uuid.UUID][]actualCostRow, error) {
	out := map[uuid.UUID][]actualCostRow{}
	if len(orderIDs) == 0 {
		return out, nil
	}
	var rows []actualCostRow
	if err := s.DB.WithContext(ctx).
		Table("purchase_order_items poi").
		Select("poi.sales_order_id, poi.quantity, poi.actual_price, po.currency").
		Joins("JOIN purchase_orders po ON po.id = poi.purchase_order_id AND po.deleted_at IS NULL").
		Where("poi.tenant_id = ? AND poi.sales_order_id IN ? AND po.status <> ?", tenantID, orderIDs, "cancelled").
		Scan(&rows).Error; err != nil {
		return nil, err
	}
	for _, r := range rows {
		out[r.SalesOrderID] = append(out[r.SalesOrderID], r)
	}
	return out, nil
}

// computeOrderFinance derives per-order settlement + actual/estimated profit
// for orders (with Items preloaded). All arithmetic uses big.Rat; conversion
// to the base currency only happens for currencies present in the fx table
// (missing rates yield nil Base amounts, never fabricated values).
func (s *Service) computeOrderFinance(ctx context.Context, tenantID int64, orders []order.Order) ([]OrderFinance, *fxrate.Table, error) {
	table := s.fxTable(ctx, tenantID)
	ids := make([]uuid.UUID, 0, len(orders))
	for _, o := range orders {
		ids = append(ids, o.ID)
	}
	var payments []PaymentRecord
	if len(ids) > 0 {
		if err := s.DB.WithContext(ctx).Where("order_id IN ?", ids).Find(&payments).Error; err != nil {
			return nil, nil, err
		}
	}
	payByOrder := map[uuid.UUID][]PaymentRecord{}
	for _, p := range payments {
		payByOrder[p.OrderID] = append(payByOrder[p.OrderID], p)
	}
	var expenses []OrderExpense
	if len(ids) > 0 {
		if err := s.DB.WithContext(ctx).Where("order_id IN ?", ids).Find(&expenses).Error; err != nil {
			return nil, nil, err
		}
	}
	expByOrder := map[uuid.UUID][]OrderExpense{}
	for _, e := range expenses {
		expByOrder[e.OrderID] = append(expByOrder[e.OrderID], e)
	}
	costs, err := s.actualCosts(ctx, tenantID, ids)
	if err != nil {
		return nil, nil, err
	}
	allItems := make([]order.OrderItem, 0)
	for _, o := range orders {
		allItems = append(allItems, o.Items...)
	}
	refByItem := map[uuid.UUID]*float64{}
	if s.Proc != nil {
		refs, err := s.Proc.ResolveLineCostRefs(ctx, allItems)
		if err == nil {
			for id, ref := range refs {
				refByItem[id] = ref.UnitCostCNY
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
		recvConvertible := true
		for _, p := range payByOrder[o.ID] {
			recvSum.Add(recvSum, fxrate.AmountRat(p.Amount))
			net := new(big.Rat).Sub(fxrate.AmountRat(p.Amount), fxrate.AmountRat(p.FeeAmount))
			if rate, ok := table.Rate(p.Currency); ok {
				recvBase.Add(recvBase, new(big.Rat).Mul(net, rate))
			} else {
				recvConvertible = false
			}
		}
		fin.PaymentCount = len(payByOrder[o.ID])
		fin.Received = fxrate.Round2(recvSum)
		feeSum := new(big.Rat)
		for _, p := range payByOrder[o.ID] {
			feeSum.Add(feeSum, fxrate.AmountRat(p.FeeAmount))
		}
		fin.FeeTotal = fxrate.Round2(feeSum)
		fin.SettlementStatus, fin.DiffAmount = settlementOf(o.TotalAmount, fin.Received)

		// Actual purchase cost (registered actual prices only).
		costBase := new(big.Rat)
		costConvertible := true
		for _, cr := range costs[o.ID] {
			if cr.ActualPrice == nil {
				fin.MissingActualLines++
				continue
			}
			amount := new(big.Rat).Mul(fxrate.AmountRat(*cr.ActualPrice), new(big.Rat).SetInt64(int64(cr.Quantity)))
			cur := strings.ToUpper(strings.TrimSpace(cr.Currency))
			if cur == "" {
				cur = "CNY"
			}
			if rate, ok := table.Rate(cur); ok {
				costBase.Add(costBase, new(big.Rat).Mul(amount, rate))
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
		}
		fin.ExpenseCount = len(expByOrder[o.ID])

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
		fin.EstimatedProfitBase = estimateProfitBase(o, refByItem, feeItems, table)

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
func estimateProfitBase(o order.Order, refByItem map[uuid.UUID]*float64, feeItems []reports.FeeItem, table *fxrate.Table) *float64 {
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
	for _, it := range o.Items {
		if p := refByItem[it.ID]; p != nil {
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

// scopedOrdersInRange loads paid orders in [start, end] under tenant + store
// scope with Items preloaded (finance口径 matches the profit report:已付款).
func (s *Service) scopedOrdersInRange(c *gin.Context, start, end time.Time) ([]order.Order, int64, error) {
	tx := s.DB.WithContext(c.Request.Context()).Model(&order.Order{}).
		Where("created_at >= ? AND created_at < ?", start, end.AddDate(0, 0, 1)).
		Where("payment_status = ?", order.PaymentPaid)
	tx, tid, err := adminperm.ApplyTenantScope(c, tx)
	if err != nil {
		return nil, 0, err
	}
	tx, err = adminperm.ApplyStoreScope(c, s.DB, tx, "shop_id")
	if err != nil {
		return nil, 0, err
	}
	var orders []order.Order
	if err := tx.Preload("Items").Order("created_at DESC").Limit(5000).Find(&orders).Error; err != nil {
		return nil, 0, err
	}
	return orders, tid, nil
}
