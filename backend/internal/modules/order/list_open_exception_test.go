package order_test

import (
	"testing"

	"github.com/trademind-ai/trademind/backend/internal/modules/order"
	"github.com/trademind-ai/trademind/backend/internal/modules/orderexception"
)

// The order-list exception badge must follow the exception workbench's
// open-row semantics: rows marked handled/ignored in order_exception_marks
// no longer count, so a positive badge always opens a non-empty workbench.
func TestListOpenExceptionCountExcludesMarkedRows(t *testing.T) {
	db := openImportTestDB(t)
	if err := db.AutoMigrate(&order.OrderItemSKUMatch{}, &orderexception.OrderExceptionMark{}); err != nil {
		t.Fatal(err)
	}
	svc := &order.Service{DB: db}

	o := order.Order{TenantID: 1, Platform: "douyin_shop", OrderNo: "SO-EXC-1", Status: "paid", Currency: "USD"}
	if err := db.Create(&o).Error; err != nil {
		t.Fatal(err)
	}
	it := order.OrderItem{OrderID: o.ID, ProductTitle: "商品", SKUCode: "SKU-X", Quantity: 1}
	if err := db.Create(&it).Error; err != nil {
		t.Fatal(err)
	}
	m := order.OrderItemSKUMatch{
		OrderID: o.ID, OrderItemID: it.ID, Platform: "douyin_shop",
		MatchStatus: order.MatchStatusUnmatched, MatchType: order.MatchTypeNone,
	}
	if err := db.Create(&m).Error; err != nil {
		t.Fatal(err)
	}

	badge := func() int {
		t.Helper()
		res, err := svc.List(importTestCtx(1), order.ListQuery{Page: 1, PageSize: 10})
		if err != nil {
			t.Fatal(err)
		}
		for _, r := range res.Items {
			if r.ID == o.ID {
				return r.OpenExceptionCount
			}
		}
		t.Fatalf("order %s not in list", o.ID)
		return -1
	}

	if got := badge(); got != 1 {
		t.Fatalf("open badge = %d, want 1", got)
	}

	mark := orderexception.OrderExceptionMark{
		ExceptionType: orderexception.TypeSKUUnmatched,
		SourceType:    orderexception.SourceOrderItemSKUMatch,
		SourceID:      m.ID.String(),
		MarkType:      orderexception.MarkHandled,
		OrderID:       &o.ID,
	}
	if err := db.Create(&mark).Error; err != nil {
		t.Fatal(err)
	}

	if got := badge(); got != 0 {
		t.Fatalf("badge after handled mark = %d, want 0", got)
	}

	if err := db.Unscoped().Delete(&mark).Error; err != nil {
		t.Fatal(err)
	}
	if got := badge(); got != 1 {
		t.Fatalf("badge after unmark = %d, want 1", got)
	}
}
