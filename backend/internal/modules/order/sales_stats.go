package order

import (
	"time"

	"github.com/gin-gonic/gin"
	"github.com/trademind-ai/trademind/backend/internal/pkg/adminperm"
	"gorm.io/gorm"
)

// SalesAmount is one currency bucket of paid sales in a window.
type SalesAmount struct {
	Currency string  `json:"currency"`
	Amount   float64 `json:"amount"`
	Orders   int64   `json:"orders"`
}

// SalesWindowStats summarizes orders created within one time window.
type SalesWindowStats struct {
	Key          string        `json:"key"`
	OrderCount   int64         `json:"orderCount"`
	PaidCount    int64         `json:"paidCount"`
	ShippedCount int64         `json:"shippedCount"`
	PaidAmounts  []SalesAmount `json:"paidAmounts"`
}

// SalesStatsDTO is GET /orders/stats/sales.
type SalesStatsDTO struct {
	GeneratedAt string             `json:"generatedAt"`
	Windows     []SalesWindowStats `json:"windows"`
}

// SalesStats aggregates order counts and paid sales amounts (grouped by
// currency) for the today / last-7-days / last-30-days windows.
func (s *Service) SalesStats(c *gin.Context) (*SalesStatsDTO, error) {
	now := time.Now()
	todayStart := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
	windows := []struct {
		key   string
		since time.Time
	}{
		{"today", todayStart},
		{"7d", todayStart.AddDate(0, 0, -6)},
		{"30d", todayStart.AddDate(0, 0, -29)},
	}
	out := &SalesStatsDTO{GeneratedAt: now.UTC().Format(time.RFC3339), Windows: []SalesWindowStats{}}
	for _, w := range windows {
		base := func() (*gorm.DB, error) {
			tx := s.DB.WithContext(c.Request.Context()).Model(&Order{}).Where("created_at >= ?", w.since)
			scoped, _, err := adminperm.ApplyTenantScope(c, tx)
			return scoped, err
		}
		st := SalesWindowStats{Key: w.key, PaidAmounts: []SalesAmount{}}
		tx, err := base()
		if err != nil {
			return nil, err
		}
		if err := tx.Count(&st.OrderCount).Error; err != nil {
			return nil, err
		}
		tx, err = base()
		if err != nil {
			return nil, err
		}
		if err := tx.Where("payment_status = ?", PaymentPaid).Count(&st.PaidCount).Error; err != nil {
			return nil, err
		}
		tx, err = base()
		if err != nil {
			return nil, err
		}
		if err := tx.Where("(status IN ? OR fulfillment_status IN ?)", []string{StatusShipped, StatusDelivered}, []string{FulfillmentFulfilled, FulfillmentPartial}).Count(&st.ShippedCount).Error; err != nil {
			return nil, err
		}
		tx, err = base()
		if err != nil {
			return nil, err
		}
		var rows []SalesAmount
		if err := tx.Where("payment_status = ?", PaymentPaid).
			Select("currency AS currency, COALESCE(SUM(total_amount),0) AS amount, COUNT(*) AS orders").
			Group("currency").Order("currency").Scan(&rows).Error; err != nil {
			return nil, err
		}
		if rows != nil {
			st.PaidAmounts = rows
		}
		out.Windows = append(out.Windows, st)
	}
	return out, nil
}
