package demoseed

import (
	"context"
	"testing"
	"time"

	"github.com/trademind-ai/trademind/backend/internal/modules/order"
)

// Round 156: today's multi-currency big-screen samples — a USD order that
// converts through the seeded manual rate plus an EUR order without a rate
// (explicit unconverted fallback), both cleaned with zero residue.
func TestFullDemoSeedRound156ScreenFXOrders(t *testing.T) {
	db := openFullDemoTestDB(t)
	s := &FullDemoSeeder{DB: db, TenantID: 1, AppEnv: "development"}

	if _, err := s.Seed(context.Background()); err != nil {
		t.Fatal(err)
	}

	now := time.Now()
	todayStart := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
	for _, p := range round156ScreenFXOrders {
		var o order.Order
		if err := db.First(&o, "order_no = ?", p.orderNo).Error; err != nil {
			t.Fatalf("expected round156 fx order %s: %v", p.orderNo, err)
		}
		if o.Currency != p.currency || o.TotalAmount != p.amount {
			t.Fatalf("unexpected %s currency/amount: %s %v", p.orderNo, o.Currency, o.TotalAmount)
		}
		if o.PaymentStatus != order.PaymentPaid {
			t.Fatalf("round156 fx order %s must be paid", p.orderNo)
		}
		if o.OrderedAt == nil || o.OrderedAt.Before(todayStart) {
			t.Fatalf("round156 fx order %s must be stamped today, got %v", p.orderNo, o.OrderedAt)
		}
		var items int64
		db.Model(&order.OrderItem{}).Where("order_id = ?", o.ID).Count(&items)
		if items == 0 {
			t.Fatalf("round156 fx order %s missing order item", p.orderNo)
		}
	}

	// Cleanup removes the samples with zero residue (DEMO- prefix).
	if _, err := s.Cleanup(context.Background()); err != nil {
		t.Fatal(err)
	}
	var n int64
	db.Model(&order.Order{}).Unscoped().Where("order_no LIKE ?", "DEMO-FX-%").Count(&n)
	if n != 0 {
		t.Fatalf("expected round156 fx orders cleaned, got %d rows", n)
	}
}
