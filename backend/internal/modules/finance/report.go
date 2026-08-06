package finance

import (
	"bytes"
	"encoding/csv"
	"fmt"
	"math/big"
	"sort"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"github.com/trademind-ai/trademind/backend/internal/modules/order"
	"github.com/trademind-ai/trademind/backend/internal/modules/reports"
	"github.com/trademind-ai/trademind/backend/internal/pkg/adminperm"
	"github.com/trademind-ai/trademind/backend/internal/pkg/csvsafe"
	"github.com/trademind-ai/trademind/backend/internal/pkg/opslabels"
	"github.com/trademind-ai/trademind/backend/internal/providers/fxrate"
)

// ReconciliationDTO is the差异工作台 payload: orders whose settlement is
// abnormal (未回款/少款/多款) or whose actual profit deviates strongly from
// the estimate.
type ReconciliationDTO struct {
	GeneratedAt  time.Time      `json:"generatedAt"`
	StartDate    string         `json:"startDate"`
	EndDate      string         `json:"endDate"`
	BaseCurrency string         `json:"baseCurrency"`
	Summary      ReconSummary   `json:"summary"`
	Rows         []OrderFinance `json:"rows"`
	Truncated    bool           `json:"truncated,omitempty"`
}

// ReconSummary counts the workbench rows per anomaly type.
type ReconSummary struct {
	OrderCount   int `json:"orderCount"`
	UnpaidCount  int `json:"unpaidCount"`
	ShortCount   int `json:"shortCount"`
	OverCount    int `json:"overCount"`
	SettledCount int `json:"settledCount"`
	LargeDiffs   int `json:"largeDiffs"`
	FlaggedCount int `json:"flaggedCount"`
}

const maxReconRows = 500

// Reconciliation builds the差异工作台 rows for the page: at most
// maxReconRows rows (Truncated flags the cut; the CSV export carries the
// full set). status filters by settlement status ("flagged" keeps only异常
// rows: not settled or large profit diff).
func (s *Service) Reconciliation(c *gin.Context, r reports.DateRange, status string) (*ReconciliationDTO, error) {
	return s.reconciliation(c, r, status, maxReconRows)
}

// reconciliation builds the workbench rows; maxRows > 0 truncates the sorted
// result to that many rows, maxRows <= 0 keeps all rows.
func (s *Service) reconciliation(c *gin.Context, r reports.DateRange, status string, maxRows int) (*ReconciliationDTO, error) {
	orders, tid, err := s.scopedOrdersInRange(c, r.Start, r.End)
	if err != nil {
		return nil, err
	}
	rows, table, err := s.computeOrderFinance(c.Request.Context(), tid, orders)
	if err != nil {
		return nil, err
	}
	out := &ReconciliationDTO{
		GeneratedAt:  time.Now(),
		StartDate:    r.Start.Format("2006-01-02"),
		EndDate:      r.End.Format("2006-01-02"),
		BaseCurrency: table.Base,
	}
	filter := strings.TrimSpace(status)
	kept := make([]OrderFinance, 0, len(rows))
	for _, row := range rows {
		out.Summary.OrderCount++
		switch row.SettlementStatus {
		case SettlementUnpaid:
			out.Summary.UnpaidCount++
		case SettlementShort:
			out.Summary.ShortCount++
		case SettlementOver:
			out.Summary.OverCount++
		case SettlementSettled:
			out.Summary.SettledCount++
		}
		flagged := row.SettlementStatus != SettlementSettled || row.LargeDiff
		if row.LargeDiff {
			out.Summary.LargeDiffs++
		}
		if flagged {
			out.Summary.FlaggedCount++
		}
		switch filter {
		case "":
			kept = append(kept, row)
		case "flagged":
			if flagged {
				kept = append(kept, row)
			}
		case "large_diff":
			if row.LargeDiff {
				kept = append(kept, row)
			}
		case SettlementUnpaid, SettlementShort, SettlementOver, SettlementSettled:
			if row.SettlementStatus == filter {
				kept = append(kept, row)
			}
		default:
			return nil, fmt.Errorf("%w: status 无效", ErrBadRequest)
		}
	}
	sort.SliceStable(kept, func(i, j int) bool {
		pi, pj := 0.0, 0.0
		if kept[i].ProfitDiffBase != nil {
			pi = *kept[i].ProfitDiffBase
			if pi < 0 {
				pi = -pi
			}
		}
		if kept[j].ProfitDiffBase != nil {
			pj = *kept[j].ProfitDiffBase
			if pj < 0 {
				pj = -pj
			}
		}
		return pi > pj
	})
	if maxRows > 0 && len(kept) > maxRows {
		kept = kept[:maxRows]
		out.Truncated = true
	}
	out.Rows = kept
	return out, nil
}

var settlementLabels = map[string]string{
	SettlementUnpaid:  "未回款",
	SettlementShort:   "少款",
	SettlementOver:    "多款",
	SettlementSettled: "已结清",
}

// ExportReconciliationCSV renders the workbench rows as CSV. Unlike the
// page (top maxReconRows rows) it carries every matching row, in the same
// order and with the same per-row numbers.
func (s *Service) ExportReconciliationCSV(c *gin.Context, r reports.DateRange, status string) ([]byte, string, error) {
	res, err := s.reconciliation(c, r, status, 0)
	if err != nil {
		return nil, "", err
	}
	base := res.BaseCurrency
	var buf bytes.Buffer
	buf.WriteString("\xEF\xBB\xBF")
	w := csv.NewWriter(&buf)
	header := []string{
		"订单号", "平台", "店铺", "币种", "应收", "已回款", "手续费", "回款差异", "对账状态",
		"实收(" + base + ")", "采购实付(" + base + ")", "费用(" + base + ")",
		"实算毛利(" + base + ")", "估算毛利(" + base + ")", "毛利差异(" + base + ")", "差异较大", "缺实付行数",
	}
	if err := w.Write(header); err != nil {
		return nil, "", err
	}
	for _, row := range res.Rows {
		rec := []string{
			csvsafe.Cell(row.OrderNo),
			csvsafe.Cell(opslabels.PlatformLabel(row.Platform)),
			csvsafe.Cell(row.ShopName),
			csvsafe.Cell(row.Currency),
			fmt.Sprintf("%.2f", row.Receivable),
			fmt.Sprintf("%.2f", row.Received),
			fmt.Sprintf("%.2f", row.FeeTotal),
			fmt.Sprintf("%.2f", row.DiffAmount),
			settlementLabels[row.SettlementStatus],
			fmtBase(row.ReceivedBase),
			fmtBase(row.ActualCostBase),
			fmtBase(row.ExpenseBase),
			fmtBase(row.ActualProfitBase),
			fmtBase(row.EstimatedProfitBase),
			fmtBase(row.ProfitDiffBase),
			boolLabel(row.LargeDiff),
			fmt.Sprintf("%d", row.MissingActualLines),
		}
		if err := w.Write(rec); err != nil {
			return nil, "", err
		}
	}
	w.Flush()
	if err := w.Error(); err != nil {
		return nil, "", err
	}
	name := fmt.Sprintf("finance-reconciliation-%s-%s.csv", res.StartDate, res.EndDate)
	return buf.Bytes(), name, nil
}

// unconvertedCell marks base-currency cells whose currency has no manual
// rate, matching the pages' explicit「未折算」rendering (never a fake 0).
const unconvertedCell = "未折算"

func fmtPtr(v *float64) string {
	if v == nil {
		return ""
	}
	return fmt.Sprintf("%.2f", *v)
}

func fmtBase(v *float64) string {
	if v == nil {
		return unconvertedCell
	}
	return fmt.Sprintf("%.2f", *v)
}

func boolLabel(v bool) string {
	if v {
		return "是"
	}
	return ""
}

// ReportRow is one shop × month aggregation of the reconciliation report.
type ReportRow struct {
	ShopID             *uuid.UUID `json:"shopId,omitempty"`
	ShopName           string     `json:"shopName"`
	Month              string     `json:"month"`
	OrderCount         int        `json:"orderCount"`
	ReceivableBase     *float64   `json:"receivableBase,omitempty"`
	ReceivedBase       *float64   `json:"receivedBase,omitempty"`
	ReturnRatePercent  *float64   `json:"returnRatePercent,omitempty"`
	FeesByType         []FeePart  `json:"feesByType"`
	ExpenseBase        *float64   `json:"expenseBase,omitempty"`
	ShopExpenseBase    *float64   `json:"shopExpenseBase,omitempty"`
	ActualCostBase     *float64   `json:"actualCostBase,omitempty"`
	ActualProfitBase   *float64   `json:"actualProfitBase,omitempty"`
	EstimatedProfit    *float64   `json:"estimatedProfitBase,omitempty"`
	ProfitDiffBase     *float64   `json:"profitDiffBase,omitempty"`
	UnpaidCount        int        `json:"unpaidCount"`
	ShortCount         int        `json:"shortCount"`
	OverCount          int        `json:"overCount"`
	SettledCount       int        `json:"settledCount"`
	LargeDiffCount     int        `json:"largeDiffCount"`
	MissingActualLines int        `json:"missingActualLines"`
}

// FeePart is one expense-type share of a report row.
type FeePart struct {
	TypeCode  string  `json:"typeCode"`
	TypeLabel string  `json:"typeLabel"`
	Base      float64 `json:"base"`
}

// ReportDTO is the shop × month reconciliation report payload.
type ReportDTO struct {
	GeneratedAt  time.Time   `json:"generatedAt"`
	StartDate    string      `json:"startDate"`
	EndDate      string      `json:"endDate"`
	BaseCurrency string      `json:"baseCurrency"`
	Rows         []ReportRow `json:"rows"`
}

type reportAcc struct {
	shopID     *uuid.UUID
	shopName   string
	month      string
	orderCount int

	receivable   *big.Rat
	received     *big.Rat
	convertible  bool
	actualCost   *big.Rat
	costOK       bool
	expense      *big.Rat
	expenseOK    bool
	actualProfit *big.Rat
	profitOK     bool
	estProfit    *big.Rat
	estOK        bool

	unpaid, short, over, settled, largeDiff, missing int
}

func newReportAcc(shopID *uuid.UUID, shopName, month string) *reportAcc {
	return &reportAcc{
		shopID: shopID, shopName: shopName, month: month,
		receivable: new(big.Rat), received: new(big.Rat), convertible: true,
		actualCost: new(big.Rat), costOK: true,
		expense: new(big.Rat), expenseOK: true,
		actualProfit: new(big.Rat), profitOK: true,
		estProfit: new(big.Rat), estOK: true,
	}
}

// Report aggregates the reconciliation numbers by shop × month, including
// shop-monthly expenses of the covered months, with回款率 and fee构成.
func (s *Service) Report(c *gin.Context, r reports.DateRange) (*ReportDTO, error) {
	ctx := c.Request.Context()
	orders, tid, err := s.scopedOrdersInRange(c, r.Start, r.End)
	if err != nil {
		return nil, err
	}
	rows, table, err := s.computeOrderFinance(ctx, tid, orders)
	if err != nil {
		return nil, err
	}
	orderMonth := map[uuid.UUID]string{}
	for _, o := range orders {
		orderMonth[o.ID] = o.CreatedAt.Format("2006-01")
	}

	accs := map[string]*reportAcc{}
	keyOf := func(shopID *uuid.UUID, month string) string {
		if shopID == nil {
			return "none|" + month
		}
		return shopID.String() + "|" + month
	}
	for _, row := range rows {
		month := orderMonth[row.OrderID]
		k := keyOf(row.ShopID, month)
		acc, ok := accs[k]
		if !ok {
			name := row.ShopName
			if name == "" {
				name = "未关联店铺"
			}
			acc = newReportAcc(row.ShopID, name, month)
			accs[k] = acc
		}
		acc.orderCount++
		acc.missing += row.MissingActualLines
		switch row.SettlementStatus {
		case SettlementUnpaid:
			acc.unpaid++
		case SettlementShort:
			acc.short++
		case SettlementOver:
			acc.over++
		case SettlementSettled:
			acc.settled++
		}
		if row.LargeDiff {
			acc.largeDiff++
		}
		if rate, ok := table.Rate(row.Currency); ok {
			acc.receivable.Add(acc.receivable, new(big.Rat).Mul(fxrate.AmountRat(row.Receivable), rate))
		} else {
			acc.convertible = false
		}
		addPtr(acc.received, &acc.convertible, row.ReceivedBase)
		addPtr(acc.actualCost, &acc.costOK, row.ActualCostBase)
		addPtr(acc.expense, &acc.expenseOK, row.ExpenseBase)
		addPtr(acc.actualProfit, &acc.profitOK, row.ActualProfitBase)
		addPtr(acc.estProfit, &acc.estOK, row.EstimatedProfitBase)
	}

	// Order-level expense构成 by type (converted rows only).
	feeParts, err := s.expensePartsByGroup(c, orders, orderMonth, table, tid)
	if err != nil {
		return nil, err
	}

	// Shop-monthly expenses of covered months join their (shop, month) row.
	shopExp, err := s.shopMonthlyBase(c, r, table)
	if err != nil {
		return nil, err
	}

	labels := s.expenseTypeLabels(ctx, tid)
	out := &ReportDTO{
		GeneratedAt:  time.Now(),
		StartDate:    r.Start.Format("2006-01-02"),
		EndDate:      r.End.Format("2006-01-02"),
		BaseCurrency: table.Base,
	}
	keys := sortedKeys(accs)
	for _, k := range keys {
		acc := accs[k]
		row := ReportRow{
			ShopID: acc.shopID, ShopName: acc.shopName, Month: acc.month,
			OrderCount:  acc.orderCount,
			UnpaidCount: acc.unpaid, ShortCount: acc.short, OverCount: acc.over,
			SettledCount: acc.settled, LargeDiffCount: acc.largeDiff, MissingActualLines: acc.missing,
		}
		if acc.convertible {
			recv := fxrate.Round2(acc.receivable)
			row.ReceivableBase = &recv
			got := fxrate.Round2(acc.received)
			row.ReceivedBase = &got
			if recv > 0 {
				rate := fxrate.Round2(new(big.Rat).Mul(new(big.Rat).Quo(acc.received, acc.receivable), big.NewRat(100, 1)))
				row.ReturnRatePercent = &rate
			}
		}
		setPtr(&row.ActualCostBase, acc.actualCost, acc.costOK)
		setPtr(&row.ExpenseBase, acc.expense, acc.expenseOK)
		setPtr(&row.ActualProfitBase, acc.actualProfit, acc.profitOK)
		setPtr(&row.EstimatedProfit, acc.estProfit, acc.estOK)
		if row.ActualProfitBase != nil && row.EstimatedProfit != nil {
			d := fxrate.Round2(new(big.Rat).Sub(fxrate.AmountRat(*row.ActualProfitBase), fxrate.AmountRat(*row.EstimatedProfit)))
			row.ProfitDiffBase = &d
		}
		parts := feeParts[k]
		row.FeesByType = make([]FeePart, 0, len(parts))
		for _, code := range sortedKeys(parts) {
			label := labels[code]
			if label == "" {
				label = code
			}
			row.FeesByType = append(row.FeesByType, FeePart{TypeCode: code, TypeLabel: label, Base: fxrate.Round2(parts[code])})
		}
		if se, ok := shopExp[k]; ok {
			v := fxrate.Round2(se)
			row.ShopExpenseBase = &v
			// Shop-monthly expenses reduce the actual profit of their row.
			if row.ActualProfitBase != nil {
				p := fxrate.Round2(new(big.Rat).Sub(fxrate.AmountRat(*row.ActualProfitBase), se))
				row.ActualProfitBase = &p
				if row.EstimatedProfit != nil {
					d := fxrate.Round2(new(big.Rat).Sub(fxrate.AmountRat(p), fxrate.AmountRat(*row.EstimatedProfit)))
					row.ProfitDiffBase = &d
				}
			}
		}
		out.Rows = append(out.Rows, row)
	}
	sort.SliceStable(out.Rows, func(i, j int) bool {
		if out.Rows[i].Month != out.Rows[j].Month {
			return out.Rows[i].Month > out.Rows[j].Month
		}
		return out.Rows[i].ShopName < out.Rows[j].ShopName
	})
	if out.Rows == nil {
		out.Rows = []ReportRow{}
	}
	return out, nil
}

func addPtr(sum *big.Rat, ok *bool, v *float64) {
	if v == nil {
		*ok = false
		return
	}
	sum.Add(sum, fxrate.AmountRat(*v))
}

func setPtr(dst **float64, sum *big.Rat, ok bool) {
	if !ok {
		return
	}
	v := fxrate.Round2(sum)
	*dst = &v
}

// expensePartsByGroup sums order-level expenses per (shop, month, type) in
// the base currency (unconvertible rows are skipped: their orders already
// surface nil ExpenseBase in the workbench).
func (s *Service) expensePartsByGroup(c *gin.Context, orders []order.Order, orderMonth map[uuid.UUID]string, table *fxrate.Table, tenantID int64) (map[string]map[string]*big.Rat, error) {
	out := map[string]map[string]*big.Rat{}
	ids := make([]uuid.UUID, 0, len(orders))
	shopOf := map[uuid.UUID]*uuid.UUID{}
	for _, o := range orders {
		ids = append(ids, o.ID)
		shopOf[o.ID] = o.ShopID
	}
	if len(ids) == 0 {
		return out, nil
	}
	type typedExpenseAgg struct {
		OrderID  uuid.UUID `gorm:"column:order_id"`
		TypeCode string    `gorm:"column:type_code"`
		Currency string    `gorm:"column:currency"`
		Amount   float64   `gorm:"column:amount"`
	}
	for _, chunk := range chunkOrderIDs(ids) {
		var groups []typedExpenseAgg
		if err := s.DB.WithContext(c.Request.Context()).Model(&OrderExpense{}).
			Select("order_id, type_code, currency, SUM(amount) AS amount").
			Where("tenant_id = ? AND order_id IN ?", tenantID, chunk).
			Group("order_id, type_code, currency").Scan(&groups).Error; err != nil {
			return nil, err
		}
		for _, e := range groups {
			rate, ok := table.Rate(e.Currency)
			if !ok {
				continue
			}
			sid := shopOf[e.OrderID]
			k := "none|" + orderMonth[e.OrderID]
			if sid != nil {
				k = sid.String() + "|" + orderMonth[e.OrderID]
			}
			if out[k] == nil {
				out[k] = map[string]*big.Rat{}
			}
			if out[k][e.TypeCode] == nil {
				out[k][e.TypeCode] = new(big.Rat)
			}
			out[k][e.TypeCode].Add(out[k][e.TypeCode], new(big.Rat).Mul(fxrate.AmountRat(e.Amount), rate))
		}
	}
	return out, nil
}

// shopMonthlyBase sums shop-monthly expenses per (shop, month) key for the
// months covered by the range, under tenant + store scope.
func (s *Service) shopMonthlyBase(c *gin.Context, r reports.DateRange, table *fxrate.Table) (map[string]*big.Rat, error) {
	months := map[string]bool{}
	for cur := time.Date(r.Start.Year(), r.Start.Month(), 1, 0, 0, 0, 0, r.Start.Location()); !cur.After(r.End); cur = cur.AddDate(0, 1, 0) {
		months[cur.Format("2006-01")] = true
	}
	list := sortedKeys(months)
	tx := s.DB.WithContext(c.Request.Context()).Model(&ShopMonthlyExpense{}).Where("month IN ?", list)
	tx, _, err := adminperm.ApplyTenantScope(c, tx)
	if err != nil {
		return nil, err
	}
	tx, err = adminperm.ApplyStoreScope(c, s.DB, tx, "shop_id")
	if err != nil {
		return nil, err
	}
	var rows []ShopMonthlyExpense
	if err := tx.Find(&rows).Error; err != nil {
		return nil, err
	}
	out := map[string]*big.Rat{}
	for _, e := range rows {
		rate, ok := table.Rate(e.Currency)
		if !ok {
			continue
		}
		k := e.ShopID.String() + "|" + e.Month
		if out[k] == nil {
			out[k] = new(big.Rat)
		}
		out[k].Add(out[k], new(big.Rat).Mul(fxrate.AmountRat(e.Amount), rate))
	}
	return out, nil
}

// ExportReportCSV renders the shop × month report as CSV.
func (s *Service) ExportReportCSV(c *gin.Context, r reports.DateRange) ([]byte, string, error) {
	res, err := s.Report(c, r)
	if err != nil {
		return nil, "", err
	}
	base := res.BaseCurrency
	var buf bytes.Buffer
	buf.WriteString("\xEF\xBB\xBF")
	w := csv.NewWriter(&buf)
	header := []string{
		"月份", "店铺", "订单数",
		"应收(" + base + ")", "已回款(" + base + ")", "回款率(%)",
		"订单费用(" + base + ")", "店铺月度费用(" + base + ")", "费用构成",
		"采购实付(" + base + ")", "实算毛利(" + base + ")", "估算毛利(" + base + ")", "毛利差异(" + base + ")",
		"未回款单数", "少款单数", "多款单数", "已结清单数", "差异较大单数", "缺实付行数",
	}
	if err := w.Write(header); err != nil {
		return nil, "", err
	}
	for _, row := range res.Rows {
		parts := make([]string, 0, len(row.FeesByType))
		for _, p := range row.FeesByType {
			parts = append(parts, fmt.Sprintf("%s %.2f", p.TypeLabel, p.Base))
		}
		rec := []string{
			row.Month,
			csvsafe.Cell(row.ShopName),
			fmt.Sprintf("%d", row.OrderCount),
			fmtBase(row.ReceivableBase),
			fmtBase(row.ReceivedBase),
			fmtBase(row.ReturnRatePercent),
			fmtBase(row.ExpenseBase),
			fmtPtr(row.ShopExpenseBase),
			csvsafe.Cell(strings.Join(parts, "；")),
			fmtBase(row.ActualCostBase),
			fmtBase(row.ActualProfitBase),
			fmtBase(row.EstimatedProfit),
			fmtBase(row.ProfitDiffBase),
			fmt.Sprintf("%d", row.UnpaidCount),
			fmt.Sprintf("%d", row.ShortCount),
			fmt.Sprintf("%d", row.OverCount),
			fmt.Sprintf("%d", row.SettledCount),
			fmt.Sprintf("%d", row.LargeDiffCount),
			fmt.Sprintf("%d", row.MissingActualLines),
		}
		if err := w.Write(rec); err != nil {
			return nil, "", err
		}
	}
	w.Flush()
	if err := w.Error(); err != nil {
		return nil, "", err
	}
	name := fmt.Sprintf("finance-report-%s-%s.csv", res.StartDate, res.EndDate)
	return buf.Bytes(), name, nil
}
