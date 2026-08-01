package orderexception

import (
	"context"
	"strings"
	"testing"

	"github.com/trademind-ai/trademind/backend/internal/modules/order"
	"github.com/trademind-ai/trademind/backend/internal/modules/procurement"
	"github.com/trademind-ai/trademind/backend/internal/modules/sourcing"
	"gorm.io/gorm"
)

func newNegMarginService(db *gorm.DB) *Service {
	return &Service{DB: db, Cost: &procurement.Service{DB: db}}
}

// seedPricedOrder seeds a paid CNY order (amount) whose single line has a
// primary source SKU mapping priced at unitCost, qty 2.
func seedPricedOrder(t *testing.T, db *gorm.DB, amount, unitCost float64) procBlockedFixture {
	t.Helper()
	fx := seedPaidOrderWithSKU(t, db)
	if err := db.Model(&order.Order{}).Where("id = ?", fx.orderID).
		Updates(map[string]any{"currency": "CNY", "total_amount": amount}).Error; err != nil {
		t.Fatal(err)
	}
	sup := sourcing.Supplier{Name: "1688 供应商"}
	if err := db.Create(&sup).Error; err != nil {
		t.Fatal(err)
	}
	src := sourcing.ProductSource{
		ProductID:  fx.productID,
		SupplierID: sup.ID,
		IsPrimary:  true,
		Status:     sourcing.SourceStatusActive,
	}
	if err := db.Create(&src).Error; err != nil {
		t.Fatal(err)
	}
	uc := unitCost
	mapping := sourcing.ProductSourceSKU{
		ProductSourceID: src.ID,
		LocalSKUID:      fx.skuID,
		ExternalSKUID:   "ext-1",
		CurrentPrice:    &uc,
	}
	if err := db.Create(&mapping).Error; err != nil {
		t.Fatal(err)
	}
	return fx
}

func listNegMargin(t *testing.T, svc *Service) []OrderExceptionDTO {
	t.Helper()
	res, err := svc.ListOrderExceptions(context.Background(), ListOrderExceptionsRequest{
		ExceptionType: TypeNegativeMargin,
		Page:          1,
		PageSize:      50,
	})
	if err != nil {
		t.Fatal(err)
	}
	return res.List
}

func TestCollectNegativeMarginSurfacesLoss(t *testing.T) {
	db := openProcBlockedTestDB(t)
	svc := newNegMarginService(db)
	// Sale 10 CNY, cost 2 × 10 CNY = 20 CNY → profit -10.
	fx := seedPricedOrder(t, db, 10, 10)

	rows := listNegMargin(t, svc)
	if len(rows) != 1 {
		t.Fatalf("expected 1 negative-margin row, got %d", len(rows))
	}
	r := rows[0]
	if r.ExceptionType != TypeNegativeMargin || r.SourceType != SourceOrder {
		t.Fatalf("unexpected type/source: %s/%s", r.ExceptionType, r.SourceType)
	}
	if r.OrderID != fx.orderID.String() {
		t.Fatalf("order pointer mismatch: %s", r.OrderID)
	}
	if !strings.Contains(r.ErrorMessage, "毛利") {
		t.Fatalf("expected profit message, got %q", r.ErrorMessage)
	}
}

func TestCollectNegativeMarginSkipsProfitableAndNonCandidates(t *testing.T) {
	db := openProcBlockedTestDB(t)
	svc := newNegMarginService(db)
	// Sale 100 CNY, cost 2 × 10 = 20 CNY → profitable.
	fx := seedPricedOrder(t, db, 100, 10)

	if rows := listNegMargin(t, svc); len(rows) != 0 {
		t.Fatalf("expected 0 rows for profitable order, got %d", len(rows))
	}

	// Make it loss-making, then move it out of the pre-ship window.
	if err := db.Model(&order.Order{}).Where("id = ?", fx.orderID).
		Update("total_amount", 5).Error; err != nil {
		t.Fatal(err)
	}
	if rows := listNegMargin(t, svc); len(rows) != 1 {
		t.Fatalf("expected 1 row after price drop, got %d", len(rows))
	}
	if err := db.Model(&order.Order{}).Where("id = ?", fx.orderID).
		Update("fulfillment_status", order.FulfillmentFulfilled).Error; err != nil {
		t.Fatal(err)
	}
	if rows := listNegMargin(t, svc); len(rows) != 0 {
		t.Fatalf("expected 0 rows for fulfilled order, got %d", len(rows))
	}
}

func TestNegativeMarginMarkIgnoredHides(t *testing.T) {
	db := openProcBlockedTestDB(t)
	svc := newNegMarginService(db)
	fx := seedPricedOrder(t, db, 10, 10)

	if err := svc.UpsertMark(context.Background(), TypeNegativeMargin, SourceOrder, fx.orderID.String(), MarkIgnored, "已知亏损清库存", nil); err != nil {
		t.Fatal(err)
	}
	if rows := listNegMargin(t, svc); len(rows) != 0 {
		t.Fatalf("expected ignored row hidden, got %d", len(rows))
	}
	ignored := true
	res, err := svc.ListOrderExceptions(context.Background(), ListOrderExceptionsRequest{
		ExceptionType: TypeNegativeMargin,
		Ignored:       &ignored,
		Page:          1,
		PageSize:      50,
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(res.List) != 1 || !res.List[0].Ignored {
		t.Fatalf("expected 1 ignored row, got %d", len(res.List))
	}
}
