package order

import (
	"sort"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/trademind-ai/trademind/backend/internal/pkg/adminperm"
)

const (
	dailyStatsDefaultDays = 30
	dailyStatsMaxDays     = 90
)

// DailyStat summarizes orders created on one local calendar day.
type DailyStat struct {
	Date        string        `json:"date"`
	OrderCount  int64         `json:"orderCount"`
	PaidCount   int64         `json:"paidCount"`
	PaidAmounts []SalesAmount `json:"paidAmounts"`
}

// DailyStatsDTO is GET /orders/stats/daily.
type DailyStatsDTO struct {
	GeneratedAt string      `json:"generatedAt"`
	Days        int         `json:"days"`
	Items       []DailyStat `json:"items"`
}

// DailyStats aggregates per-day order counts, paid counts and paid sales
// amounts (grouped by currency) for the last N local calendar days. Scope
// matches the order list: current tenant, soft-deleted orders excluded, and
// non-admin principals restricted to their granted shops.
func (s *Service) DailyStats(c *gin.Context, days int) (*DailyStatsDTO, error) {
	if days <= 0 {
		days = dailyStatsDefaultDays
	}
	if days > dailyStatsMaxDays {
		days = dailyStatsMaxDays
	}
	now := time.Now()
	todayStart := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
	since := todayStart.AddDate(0, 0, -(days - 1))

	tx := s.DB.WithContext(c.Request.Context()).Model(&Order{}).Where("created_at >= ?", since)
	tx, _, err := adminperm.ApplyTenantScope(c, tx)
	if err != nil {
		return nil, err
	}
	tx, err = adminperm.ApplyStoreScope(c, s.DB, tx, "shop_id")
	if err != nil {
		return nil, err
	}
	var rows []struct {
		CreatedAt     time.Time
		PaymentStatus string
		Currency      string
		TotalAmount   float64
	}
	if err := tx.Select("created_at, payment_status, currency, total_amount").Scan(&rows).Error; err != nil {
		return nil, err
	}

	orderCounts := make(map[string]int64, days)
	paidCounts := make(map[string]int64, days)
	paidAmounts := make(map[string]map[string]*SalesAmount, days)
	for _, r := range rows {
		d := r.CreatedAt.In(now.Location()).Format("2006-01-02")
		orderCounts[d]++
		if r.PaymentStatus != PaymentPaid {
			continue
		}
		paidCounts[d]++
		byCurrency := paidAmounts[d]
		if byCurrency == nil {
			byCurrency = map[string]*SalesAmount{}
			paidAmounts[d] = byCurrency
		}
		a := byCurrency[r.Currency]
		if a == nil {
			a = &SalesAmount{Currency: r.Currency}
			byCurrency[r.Currency] = a
		}
		a.Amount += r.TotalAmount
		a.Orders++
	}

	out := &DailyStatsDTO{GeneratedAt: now.UTC().Format(time.RFC3339), Days: days, Items: make([]DailyStat, 0, days)}
	for i := 0; i < days; i++ {
		d := since.AddDate(0, 0, i).Format("2006-01-02")
		st := DailyStat{Date: d, OrderCount: orderCounts[d], PaidCount: paidCounts[d], PaidAmounts: []SalesAmount{}}
		for _, a := range paidAmounts[d] {
			st.PaidAmounts = append(st.PaidAmounts, *a)
		}
		sort.Slice(st.PaidAmounts, func(x, y int) bool { return st.PaidAmounts[x].Currency < st.PaidAmounts[y].Currency })
		out.Items = append(out.Items, st)
	}
	return out, nil
}
