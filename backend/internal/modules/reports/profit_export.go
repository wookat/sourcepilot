package reports

import (
	"bytes"
	"encoding/csv"
	"fmt"
	"sort"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/trademind-ai/trademind/backend/internal/pkg/csvsafe"
)

var profitDimensionLabels = map[string]string{
	DimensionOrder:   "订单",
	DimensionProduct: "商品",
	DimensionShop:    "店铺",
}

// ExportProfitCSV renders the profit report as one CSV row per aggregation
// row: original-currency revenue columns plus converted columns (blank when
// no manual rate), cost / fees / profit in the base currency. Scope and
// per-row numbers match GET /reports/profit; unlike the page (top
// profitMaxRows rows) the CSV carries every row.
func (s *Service) ExportProfitCSV(c *gin.Context, dimension string, r DateRange) ([]byte, string, error) {
	res, err := s.profitReport(c, dimension, r, 0)
	if err != nil {
		return nil, "", err
	}

	currencySet := map[string]bool{}
	for _, row := range res.Rows {
		for _, a := range row.Revenue {
			currencySet[a.Currency] = true
		}
	}
	currencies := make([]string, 0, len(currencySet))
	for cur := range currencySet {
		currencies = append(currencies, cur)
	}
	sort.Strings(currencies)

	dimLabel := profitDimensionLabels[res.Dimension]
	header := []string{dimLabel, "订单数", "件数"}
	for _, cur := range currencies {
		header = append(header, "收入("+cur+")", "折算收入("+cur+"→"+res.BaseCurrency+")")
	}
	header = append(header,
		"收入合计("+res.BaseCurrency+")",
		"采购成本(CNY)",
		"采购成本("+res.BaseCurrency+")",
		"费用("+res.BaseCurrency+")",
		"毛利("+res.BaseCurrency+")",
		"毛利率(%)",
		"缺进价行数",
		"未折算币种",
	)

	var buf bytes.Buffer
	buf.WriteString("\xEF\xBB\xBF") // UTF-8 BOM for Excel
	w := csv.NewWriter(&buf)
	if err := w.Write(header); err != nil {
		return nil, "", err
	}
	fmtPtr := func(v *float64) string {
		if v == nil {
			return ""
		}
		return fmt.Sprintf("%.2f", *v)
	}
	for _, row := range res.Rows {
		byCur := make(map[string]MoneyByCurrency, len(row.Revenue))
		for _, a := range row.Revenue {
			byCur[a.Currency] = a
		}
		rec := []string{
			csvsafe.Cell(row.Label),
			fmt.Sprintf("%d", row.OrderCount),
			fmt.Sprintf("%d", row.Quantity),
		}
		for _, cur := range currencies {
			a, ok := byCur[cur]
			if !ok {
				rec = append(rec, "0.00", "0.00")
				continue
			}
			rec = append(rec, fmt.Sprintf("%.2f", a.Amount))
			if a.BaseAmount != nil {
				rec = append(rec, fmt.Sprintf("%.2f", *a.BaseAmount))
			} else {
				rec = append(rec, "") // no manual rate: never fake a converted value
			}
		}
		rec = append(rec,
			fmt.Sprintf("%.2f", row.RevenueBase),
			fmt.Sprintf("%.2f", row.CostCNY),
			fmtPtr(row.CostBase),
			fmt.Sprintf("%.2f", row.FeeBase),
			fmtPtr(row.GrossProfitBase),
			fmtPtr(row.MarginPercent),
			fmt.Sprintf("%d", row.MissingCostLines),
			strings.Join(row.UnconvertedCurrencies, " "),
		)
		if err := w.Write(rec); err != nil {
			return nil, "", err
		}
	}
	w.Flush()
	if err := w.Error(); err != nil {
		return nil, "", err
	}
	filename := fmt.Sprintf("profit-report-%s-%s-%s.csv", res.Dimension, res.StartDate, res.EndDate)
	return buf.Bytes(), filename, nil
}
