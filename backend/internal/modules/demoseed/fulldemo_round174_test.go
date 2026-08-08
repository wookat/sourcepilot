package demoseed

import (
	"context"
	"testing"
	"time"

	"github.com/trademind-ai/trademind/backend/internal/modules/order"
)

// Round 174: seeded delivered timestamps must not be in the future, so a
// fresh "new events only" delivered-node rule never matches stale seed rows
// as if they were new deliveries (R173-line2 P2-3).
func TestFullDemoSeedDeliveredAtNotInFuture(t *testing.T) {
	db := openFullDemoTestDB(t)
	s := &FullDemoSeeder{DB: db, TenantID: 1, AppEnv: "development"}

	if _, err := s.Seed(context.Background()); err != nil {
		t.Fatal(err)
	}

	now := time.Now().UTC()
	var orders []order.Order
	if err := db.Where("delivered_at IS NOT NULL").Find(&orders).Error; err != nil {
		t.Fatal(err)
	}
	if len(orders) == 0 {
		t.Fatal("expected at least one seeded delivered order")
	}
	for _, o := range orders {
		if o.DeliveredAt.After(now) {
			t.Errorf("order %s delivered_at %s is in the future (now %s)", o.OrderNo, o.DeliveredAt, now)
		}
	}

	var shipments []order.OrderShipment
	if err := db.Where("delivered_at IS NOT NULL").Find(&shipments).Error; err != nil {
		t.Fatal(err)
	}
	for _, sh := range shipments {
		if sh.DeliveredAt.After(now) {
			t.Errorf("shipment %s delivered_at %s is in the future (now %s)", sh.TrackingNo, sh.DeliveredAt, now)
		}
	}
}
