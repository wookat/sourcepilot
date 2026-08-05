package procurement

import (
	"context"
	"testing"

	"github.com/trademind-ai/trademind/backend/internal/modules/order"
)

// Orders pending review / held must not generate purchase orders.
func TestGenerateBlockedByOrderReviewStatus(t *testing.T) {
	fx := setupFixture(t)

	for _, st := range []string{order.ReviewStatusPending, order.ReviewStatusHeld} {
		if err := fx.svc.DB.Model(&order.Order{}).Where("id = ?", fx.orderID).
			Update("review_status", st).Error; err != nil {
			t.Fatal(err)
		}
		res, err := fx.svc.Generate(context.Background(), GenerateBody{OrderIDs: []string{fx.orderID.String()}}, nil)
		if err != nil {
			t.Fatal(err)
		}
		if len(res.Orders) != 0 {
			t.Fatalf("status %s: expected no purchase orders, got %d", st, len(res.Orders))
		}
		if len(res.Blockers) != 1 || res.Blockers[0].Code != "review.blocked" {
			t.Fatalf("status %s: expected review.blocked blocker, got %+v", st, res.Blockers)
		}
	}

	// Approved order generates normally.
	if err := fx.svc.DB.Model(&order.Order{}).Where("id = ?", fx.orderID).
		Update("review_status", order.ReviewStatusApproved).Error; err != nil {
		t.Fatal(err)
	}
	res, err := fx.svc.Generate(context.Background(), GenerateBody{OrderIDs: []string{fx.orderID.String()}}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Orders) == 0 {
		t.Fatalf("approved order should generate purchase orders, blockers=%+v", res.Blockers)
	}
}
