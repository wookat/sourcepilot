package order

import (
	"bytes"
	"encoding/csv"
	"fmt"
	"sort"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/trademind-ai/trademind/backend/internal/pkg/csvsafe"
)

// ExportDailyStatsCSV renders the daily report as one CSV row per day.
// Columns: 日期/订单数/已付款数/已发货数, then per currency seen in the window
// (sorted by currency code) an original-amount column and a converted-to-base
// column (blank when no manual rate is configured), followed by the converted
// total in the base currency and the list of unconverted currencies. Scope
// and semantics match GET /orders/stats/daily.
func (s *Service) ExportDailyStatsCSV(c *gin.Context, days int) ([]byte, string, error) {
	if s == nil || s.DB == nil {
		return nil, "", fmt.Errorf("order: no db")
	}
	res, err := s.DailyStats(c, days)
	if err != nil {
		return nil, "", err
	}

	currencySet := map[string]bool{}
	noRate := map[string]bool{}
	for _, it := range res.Items {
		for _, a := range it.PaidAmounts {
			currencySet[a.Currency] = true
		}
		for _, cur := range it.UnconvertedCurrencies {
			noRate[cur] = true
		}
	}
	currencies := make([]string, 0, len(currencySet))
	for cur := range currencySet {
		currencies = append(currencies, cur)
	}
	sort.Strings(currencies)

	header := []string{"日期", "订单数", "已付款数", "已发货数"}
	for _, cur := range currencies {
		header = append(header, "已付款销售额("+cur+")", "折算金额("+cur+"→"+res.BaseCurrency+")")
	}
	header = append(header, "已付款销售额合计("+res.BaseCurrency+")", "未折算币种")

	var buf bytes.Buffer
	buf.WriteString("\xEF\xBB\xBF") // UTF-8 BOM for Excel
	w := csv.NewWriter(&buf)
	if err := w.Write(header); err != nil {
		return nil, "", err
	}
	for _, it := range res.Items {
		byCurrency := make(map[string]SalesAmount, len(it.PaidAmounts))
		for _, a := range it.PaidAmounts {
			byCurrency[a.Currency] = a
		}
		row := []string{
			it.Date,
			fmt.Sprintf("%d", it.OrderCount),
			fmt.Sprintf("%d", it.PaidCount),
			fmt.Sprintf("%d", it.ShippedCount),
		}
		for _, cur := range currencies {
			a := byCurrency[cur]
			row = append(row, fmt.Sprintf("%.2f", a.Amount))
			switch {
			case a.BaseAmount != nil:
				row = append(row, fmt.Sprintf("%.2f", *a.BaseAmount))
			case noRate[cur]:
				row = append(row, "") // no manual rate: never fake a converted value
			default:
				row = append(row, "0.00")
			}
		}
		row = append(row, fmt.Sprintf("%.2f", it.PaidAmountBase), strings.Join(it.UnconvertedCurrencies, " "))
		if err := w.Write(csvsafe.Row(row)); err != nil {
			return nil, "", err
		}
	}
	w.Flush()
	if err := w.Error(); err != nil {
		return nil, "", err
	}
	name := fmt.Sprintf("daily-report-%dd.csv", res.Days)
	return buf.Bytes(), name, nil
}
