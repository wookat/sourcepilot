package order_test

import (
	"testing"

	"github.com/google/uuid"
	"github.com/trademind-ai/trademind/backend/internal/modules/order"
)

// Lines imported without matchSkus have no order_item_sku_matches row; the
// list must still expose the real orderItemId (never the zero UUID) so the
// frontend can load candidates and bind.
func TestListSKUMatchRowsForOrderWithoutMatchRecord(t *testing.T) {
	db := openImportTestDB(t)
	if err := db.AutoMigrate(&order.OrderItemSKUMatch{}); err != nil {
		t.Fatal(err)
	}
	svc := &order.Service{DB: db}
	c := importTestCtx(1)

	sum, err := svc.ImportOrders(c, order.ImportBody{Orders: []order.CreateBody{
		importOrderBody("SO-R10-1"),
	}}, nil)
	if err != nil || sum.Created != 1 {
		t.Fatalf("import failed: %v %+v", err, sum)
	}
	var o order.Order
	if err := db.First(&o, "order_no = ?", "SO-R10-1").Error; err != nil {
		t.Fatal(err)
	}
	var it order.OrderItem
	if err := db.First(&it, "order_id = ?", o.ID).Error; err != nil {
		t.Fatal(err)
	}

	rows, err := svc.ListSKUMatchRowsForOrder(c, o.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 1 {
		t.Fatalf("expected 1 row, got %d", len(rows))
	}
	r := rows[0]
	if r.OrderItemID == uuid.Nil {
		t.Fatalf("orderItemId must not be zero UUID")
	}
	if r.OrderItemID != it.ID {
		t.Fatalf("orderItemId = %s, want %s", r.OrderItemID, it.ID)
	}
	if r.OrderID != o.ID {
		t.Fatalf("orderId = %s, want %s", r.OrderID, o.ID)
	}
	if r.MatchStatus != order.MatchStatusUnmatched {
		t.Fatalf("matchStatus = %q, want unmatched", r.MatchStatus)
	}
	if r.MatchType != order.MatchTypeNone {
		t.Fatalf("matchType = %q, want none", r.MatchType)
	}
	if r.SKUCode != "SKU-1" {
		t.Fatalf("skuCode = %q, want SKU-1", r.SKUCode)
	}
}
