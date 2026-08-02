package order

import (
	"bytes"
	"encoding/csv"
	"fmt"
	"sort"

	"github.com/gin-gonic/gin"
)

// ExportDailyStatsCSV renders the daily report as one CSV row per day.
// Columns: 日期/订单数/已付款数/已发货数 plus one 已付款销售额 column per
// currency seen in the window (sorted by currency code). Scope and
// semantics match GET /orders/stats/daily.
func (s *Service) ExportDailyStatsCSV(c *gin.Context, days int) ([]byte, string, error) {
	if s == nil || s.DB == nil {
		return nil, "", fmt.Errorf("order: no db")
	}
	res, err := s.DailyStats(c, days)
	if err != nil {
		return nil, "", err
	}

	currencySet := map[string]bool{}
	for _, it := range res.Items {
		for _, a := range it.PaidAmounts {
			currencySet[a.Currency] = true
		}
	}
	currencies := make([]string, 0, len(currencySet))
	for cur := range currencySet {
		currencies = append(currencies, cur)
	}
	sort.Strings(currencies)

	header := []string{"日期", "订单数", "已付款数", "已发货数"}
	for _, cur := range currencies {
		header = append(header, "已付款销售额("+cur+")")
	}

	var buf bytes.Buffer
	buf.WriteString("\xEF\xBB\xBF") // UTF-8 BOM for Excel
	w := csv.NewWriter(&buf)
	if err := w.Write(header); err != nil {
		return nil, "", err
	}
	for _, it := range res.Items {
		byCurrency := make(map[string]float64, len(it.PaidAmounts))
		for _, a := range it.PaidAmounts {
			byCurrency[a.Currency] = a.Amount
		}
		row := []string{
			it.Date,
			fmt.Sprintf("%d", it.OrderCount),
			fmt.Sprintf("%d", it.PaidCount),
			fmt.Sprintf("%d", it.ShippedCount),
		}
		for _, cur := range currencies {
			row = append(row, fmt.Sprintf("%.2f", byCurrency[cur]))
		}
		if err := w.Write(row); err != nil {
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
