package orderexception

import (
	"context"
	"fmt"
	"testing"

	"github.com/glebarez/sqlite"
	"github.com/google/uuid"
	"github.com/trademind-ai/trademind/backend/internal/modules/order"
	"github.com/trademind-ai/trademind/backend/internal/modules/procurement"
	"github.com/trademind-ai/trademind/backend/internal/modules/product"
	"github.com/trademind-ai/trademind/backend/internal/modules/sourcing"
	"gorm.io/gorm"
)

func openProcBlockedTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	dsn := fmt.Sprintf("file:procblocked_%s?mode=memory&cache=shared&_pragma=busy_timeout(5000)", uuid.New().String())
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Skipf("sqlite unavailable: %v", err)
	}
	sqlDB, err := db.DB()
	if err != nil {
		t.Fatal(err)
	}
	sqlDB.SetMaxOpenConns(1)
	if err := db.AutoMigrate(
		&order.Order{},
		&order.OrderItem{},
		&product.Product{},
		&product.ProductSKU{},
		&sourcing.Supplier{},
		&sourcing.ProductSource{},
		&sourcing.ProductSourceSKU{},
		&procurement.PurchaseOrder{},
		&procurement.PurchaseOrderItem{},
		&OrderExceptionMark{},
	); err != nil {
		t.Fatal(err)
	}
	return db
}

type procBlockedFixture struct {
	orderID   uuid.UUID
	itemID    uuid.UUID
	productID uuid.UUID
	skuID     uuid.UUID
}

func seedPaidOrderWithSKU(t *testing.T, db *gorm.DB) procBlockedFixture {
	t.Helper()
	prod := product.Product{Title: "测试商品"}
	if err := db.Create(&prod).Error; err != nil {
		t.Fatal(err)
	}
	sku := product.ProductSKU{ProductID: prod.ID, SKUName: "红色/M", SKUCode: "SKU-RED-M"}
	if err := db.Create(&sku).Error; err != nil {
		t.Fatal(err)
	}
	o := order.Order{
		Platform:          "shopee",
		OrderNo:           "SO-" + uuid.NewString()[:8],
		CustomerName:      "买家",
		Status:            order.StatusPaid,
		PaymentStatus:     order.PaymentPaid,
		FulfillmentStatus: order.FulfillmentUnfulfilled,
		Currency:          "USD",
	}
	if err := db.Create(&o).Error; err != nil {
		t.Fatal(err)
	}
	pid := prod.ID
	sid := sku.ID
	it := order.OrderItem{
		OrderID:      o.ID,
		ProductID:    &pid,
		ProductSKUID: &sid,
		ProductTitle: prod.Title,
		SKUName:      sku.SKUName,
		Quantity:     2,
	}
	if err := db.Create(&it).Error; err != nil {
		t.Fatal(err)
	}
	return procBlockedFixture{orderID: o.ID, itemID: it.ID, productID: prod.ID, skuID: sku.ID}
}

func listProcBlocked(t *testing.T, svc *Service) []OrderExceptionDTO {
	t.Helper()
	res, err := svc.ListOrderExceptions(context.Background(), ListOrderExceptionsRequest{
		ExceptionType: TypeProcurementBlocked,
		Page:          1,
		PageSize:      50,
	})
	if err != nil {
		t.Fatal(err)
	}
	return res.List
}

func TestCollectProcurementBlockedSourceMissing(t *testing.T) {
	db := openProcBlockedTestDB(t)
	svc := &Service{DB: db}
	fx := seedPaidOrderWithSKU(t, db)

	rows := listProcBlocked(t, svc)
	if len(rows) != 1 {
		t.Fatalf("expected 1 blocked row, got %d", len(rows))
	}
	r := rows[0]
	if r.ExceptionType != TypeProcurementBlocked || r.SourceType != SourceOrderItem {
		t.Fatalf("unexpected type/source: %s/%s", r.ExceptionType, r.SourceType)
	}
	if r.OrderID != fx.orderID.String() || r.OrderItemID != fx.itemID.String() {
		t.Fatalf("order pointers mismatch: %s %s", r.OrderID, r.OrderItemID)
	}
	if r.SourcingURL == "" {
		t.Fatal("expected sourcingUrl for procurement_blocked")
	}
}

func TestCollectProcurementBlockedMappingMissingAndResolved(t *testing.T) {
	db := openProcBlockedTestDB(t)
	svc := &Service{DB: db}
	fx := seedPaidOrderWithSKU(t, db)

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

	rows := listProcBlocked(t, svc)
	if len(rows) != 1 {
		t.Fatalf("expected 1 mapping-missing row, got %d", len(rows))
	}
	if rows[0].ErrorMessage == "" || rows[0].SuggestedAction == "" {
		t.Fatal("expected message and suggested action")
	}

	mapping := sourcing.ProductSourceSKU{
		ProductSourceID: src.ID,
		LocalSKUID:      fx.skuID,
		ExternalSKUID:   "ext-1",
	}
	if err := db.Create(&mapping).Error; err != nil {
		t.Fatal(err)
	}

	if rows = listProcBlocked(t, svc); len(rows) != 0 {
		t.Fatalf("expected 0 rows after mapping added, got %d", len(rows))
	}
}

func TestCollectProcurementBlockedSkipsCoveredAndUnpaid(t *testing.T) {
	db := openProcBlockedTestDB(t)
	svc := &Service{DB: db}
	fx := seedPaidOrderWithSKU(t, db)

	// A live purchase order line covering this sales order + SKU hides the row.
	po := procurement.PurchaseOrder{Status: procurement.StatusPlacing}
	if err := db.Create(&po).Error; err != nil {
		t.Fatal(err)
	}
	oid := fx.orderID
	poi := procurement.PurchaseOrderItem{
		PurchaseOrderID: po.ID,
		SalesOrderID:    &oid,
		LocalSKUID:      fx.skuID,
		SourceSKUID:     uuid.New(),
		Quantity:        2,
	}
	if err := db.Create(&poi).Error; err != nil {
		t.Fatal(err)
	}
	if rows := listProcBlocked(t, svc); len(rows) != 0 {
		t.Fatalf("expected 0 rows when covered by live purchase order, got %d", len(rows))
	}

	// A cancelled purchase order does not count as coverage.
	if err := db.Model(&procurement.PurchaseOrder{}).Where("id = ?", po.ID).
		Update("status", procurement.StatusCancelled).Error; err != nil {
		t.Fatal(err)
	}
	if rows := listProcBlocked(t, svc); len(rows) != 1 {
		t.Fatalf("expected 1 row when purchase order cancelled, got %d", len(rows))
	}

	// Unpaid orders are not surfaced.
	if err := db.Model(&order.Order{}).Where("id = ?", fx.orderID).
		Update("payment_status", order.PaymentUnpaid).Error; err != nil {
		t.Fatal(err)
	}
	if rows := listProcBlocked(t, svc); len(rows) != 0 {
		t.Fatalf("expected 0 rows for unpaid order, got %d", len(rows))
	}
}

func TestProcurementBlockedMarkHandled(t *testing.T) {
	db := openProcBlockedTestDB(t)
	svc := &Service{DB: db}
	fx := seedPaidOrderWithSKU(t, db)

	if err := svc.UpsertMark(context.Background(), TypeProcurementBlocked, SourceOrderItem, fx.itemID.String(), MarkHandled, "已线下处理", nil); err != nil {
		t.Fatal(err)
	}
	if rows := listProcBlocked(t, svc); len(rows) != 0 {
		t.Fatalf("expected handled row hidden from default view, got %d", len(rows))
	}

	handled := true
	res, err := svc.ListOrderExceptions(context.Background(), ListOrderExceptionsRequest{
		ExceptionType: TypeProcurementBlocked,
		Handled:       &handled,
		Page:          1,
		PageSize:      50,
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(res.List) != 1 || !res.List[0].Handled {
		t.Fatalf("expected 1 handled row, got %d", len(res.List))
	}
}
