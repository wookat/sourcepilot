package procurement

import (
	"context"
	"fmt"
	"testing"

	"github.com/glebarez/sqlite"
	"github.com/google/uuid"
	"github.com/trademind-ai/trademind/backend/internal/modules/inventory"
	"github.com/trademind-ai/trademind/backend/internal/modules/order"
	"github.com/trademind-ai/trademind/backend/internal/modules/product"
	"github.com/trademind-ai/trademind/backend/internal/modules/sourcing"
	"github.com/trademind-ai/trademind/backend/internal/providers/trade"
	"gorm.io/gorm"
)

func openTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	dsn := fmt.Sprintf("file:procurement_%s?mode=memory&cache=shared", uuid.NewString())
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Skipf("sqlite unavailable: %v", err)
	}
	if err := db.AutoMigrate(
		&sourcing.Supplier{}, &sourcing.ProductSource{}, &sourcing.ProductSourceSKU{},
		&sourcing.SourcePriceHistory{}, &sourcing.SourceSwitchEvent{},
		&order.Order{}, &order.OrderItem{},
		&product.Product{}, &product.ProductSKU{},
		&PurchaseOrder{}, &PurchaseOrderItem{}, &PurchaseOrderEvent{}, &PurchaseLogistics{},
		&inventory.InventoryChangeLog{},
		&inventory.Warehouse{}, &inventory.WarehouseStock{},
	); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	return db
}

type fixture struct {
	svc     *Service
	orderID uuid.UUID
}

// setupFixture creates a sales order with one item mapped to a primary
// source (supplier + SKU mapping ready for aggregation).
func setupFixture(t *testing.T) fixture {
	t.Helper()
	db := openTestDB(t)
	svc := &Service{DB: db, Provider: trade.NewMock1688()}

	sup := sourcing.Supplier{Platform: "1688", Name: "supplier-a", Status: "active"}
	if err := db.Create(&sup).Error; err != nil {
		t.Fatal(err)
	}
	productID := uuid.New()
	localSKU := uuid.New()
	src := sourcing.ProductSource{
		ProductID: productID, SupplierID: sup.ID, IsPrimary: true, Priority: 10,
		Status: sourcing.SourceStatusActive, SourceOfferID: "111",
		SourceURL: "https://detail.1688.com/offer/111.html",
	}
	if err := db.Create(&src).Error; err != nil {
		t.Fatal(err)
	}
	price := 9.9
	mapping := sourcing.ProductSourceSKU{
		ProductSourceID: src.ID, LocalSKUID: localSKU, ExternalSKUID: "ext-1",
		Currency: "CNY", Status: "active", CurrentPrice: &price,
	}
	if err := db.Create(&mapping).Error; err != nil {
		t.Fatal(err)
	}
	o := order.Order{TenantID: 0, Platform: "tiktok", OrderNo: "SO-1", Status: "paid", Currency: "USD"}
	if err := db.Create(&o).Error; err != nil {
		t.Fatal(err)
	}
	item := order.OrderItem{
		OrderID: o.ID, ProductID: &productID, ProductSKUID: &localSKU,
		ProductTitle: "demo product", SKUName: "red / L", Quantity: 3,
	}
	if err := db.Create(&item).Error; err != nil {
		t.Fatal(err)
	}
	return fixture{svc: svc, orderID: o.ID}
}

func generate(t *testing.T, f fixture, key string) *GenerateResult {
	t.Helper()
	res, err := f.svc.Generate(context.Background(), GenerateBody{
		OrderIDs:       []string{f.orderID.String()},
		IdempotencyKey: key,
	}, nil)
	if err != nil {
		t.Fatalf("generate: %v", err)
	}
	return res
}

func TestGenerateAggregatesBySupplierAndIsIdempotent(t *testing.T) {
	f := setupFixture(t)
	res := generate(t, f, "key-1")
	if len(res.Orders) != 1 || len(res.Blockers) != 0 {
		t.Fatalf("expected 1 purchase order, got %+v", res)
	}
	po := res.Orders[0]
	if po.Status != StatusDraft || po.SupplierName != "supplier-a" {
		t.Fatalf("unexpected po %+v", po)
	}
	if po.TotalAmount != 29.7 {
		t.Fatalf("expected total 29.7, got %v", po.TotalAmount)
	}
	// idempotent replay
	res2 := generate(t, f, "key-1")
	if len(res2.Orders) != 1 || res2.Orders[0].ID != po.ID {
		t.Fatalf("expected idempotent replay, got %+v", res2.Orders)
	}
}

func TestDetailFillsSalesOrderNo(t *testing.T) {
	f := setupFixture(t)
	res := generate(t, f, "key-detail-no")
	if len(res.Orders) != 1 {
		t.Fatalf("expected 1 purchase order, got %+v", res)
	}
	po, err := f.svc.Detail(context.Background(), res.Orders[0].ID)
	if err != nil {
		t.Fatalf("detail: %v", err)
	}
	if len(po.Items) == 0 {
		t.Fatal("expected items")
	}
	for _, it := range po.Items {
		if it.SalesOrderID == nil {
			continue
		}
		if it.SalesOrderNo != "SO-1" {
			t.Fatalf("expected salesOrderNo SO-1, got %q", it.SalesOrderNo)
		}
	}
}

func TestGenerateSkipsLinesCoveredByActivePO(t *testing.T) {
	f := setupFixture(t)
	first := generate(t, f, "key-covered-1")
	if len(first.Orders) != 1 {
		t.Fatalf("expected 1 purchase order, got %+v", first)
	}
	// second request (different idempotency key) must not duplicate covered lines
	second := generate(t, f, "key-covered-2")
	if len(second.Orders) != 0 {
		t.Fatalf("expected no duplicate purchase order, got %+v", second.Orders)
	}
	if len(second.Warnings) == 0 || second.Warnings[0].Code != "line.covered" {
		t.Fatalf("expected line.covered warning, got %+v", second.Warnings)
	}
	// cancelling the original purchase order frees the lines again
	if err := f.svc.DB.Model(&PurchaseOrder{}).
		Where("id = ?", first.Orders[0].ID).
		Update("status", StatusCancelled).Error; err != nil {
		t.Fatal(err)
	}
	third := generate(t, f, "key-covered-3")
	if len(third.Orders) != 1 {
		t.Fatalf("expected regeneration after cancel, got %+v", third)
	}
}

func TestGenerateReportsBlockers(t *testing.T) {
	f := setupFixture(t)
	// an order item without local SKU link
	o := order.Order{Platform: "tiktok", OrderNo: "SO-2", Status: "paid"}
	if err := f.svc.DB.Create(&o).Error; err != nil {
		t.Fatal(err)
	}
	item := order.OrderItem{OrderID: o.ID, SKUName: "unmatched", Quantity: 1}
	if err := f.svc.DB.Create(&item).Error; err != nil {
		t.Fatal(err)
	}
	res, err := f.svc.Generate(context.Background(), GenerateBody{OrderIDs: []string{o.ID.String()}}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Orders) != 0 || len(res.Blockers) != 1 || res.Blockers[0].Code != "sku.unmatched" {
		t.Fatalf("expected sku.unmatched blocker, got %+v", res)
	}
}

func TestGenerateBlockersCarryStructuredIDs(t *testing.T) {
	f := setupFixture(t)
	db := f.svc.DB

	// source.missing: item linked to a product without any primary source
	noSourceProduct := uuid.New()
	noSourceSKU := uuid.New()
	o1 := order.Order{Platform: "tiktok", OrderNo: "SO-3", Status: "paid"}
	if err := db.Create(&o1).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Create(&order.OrderItem{
		OrderID: o1.ID, ProductID: &noSourceProduct, ProductSKUID: &noSourceSKU,
		SKUName: "no-source", Quantity: 1,
	}).Error; err != nil {
		t.Fatal(err)
	}
	res, err := f.svc.Generate(context.Background(), GenerateBody{OrderIDs: []string{o1.ID.String()}}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Blockers) != 1 || res.Blockers[0].Code != "source.missing" {
		t.Fatalf("expected source.missing blocker, got %+v", res)
	}
	b := res.Blockers[0]
	if b.OrderID != o1.ID.String() || b.ProductID != noSourceProduct.String() || b.LocalSKUID != noSourceSKU.String() {
		t.Fatalf("expected structured ids on source.missing blocker, got %+v", b)
	}

	// mapping.missing: primary source exists but no SKU mapping for the line
	sup := sourcing.Supplier{Platform: "1688", Name: "supplier-b", Status: "active"}
	if err := db.Create(&sup).Error; err != nil {
		t.Fatal(err)
	}
	noMappingProduct := uuid.New()
	noMappingSKU := uuid.New()
	if err := db.Create(&sourcing.ProductSource{
		ProductID: noMappingProduct, SupplierID: sup.ID, IsPrimary: true, Priority: 10,
		Status: sourcing.SourceStatusActive, SourceOfferID: "222",
		SourceURL: "https://detail.1688.com/offer/222.html",
	}).Error; err != nil {
		t.Fatal(err)
	}
	o2 := order.Order{Platform: "tiktok", OrderNo: "SO-4", Status: "paid"}
	if err := db.Create(&o2).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Create(&order.OrderItem{
		OrderID: o2.ID, ProductID: &noMappingProduct, ProductSKUID: &noMappingSKU,
		SKUName: "no-mapping", Quantity: 1,
	}).Error; err != nil {
		t.Fatal(err)
	}
	res, err = f.svc.Generate(context.Background(), GenerateBody{OrderIDs: []string{o2.ID.String()}}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Blockers) != 1 || res.Blockers[0].Code != "mapping.missing" {
		t.Fatalf("expected mapping.missing blocker, got %+v", res)
	}
	b = res.Blockers[0]
	if b.OrderID != o2.ID.String() || b.ProductID != noMappingProduct.String() || b.LocalSKUID != noMappingSKU.String() {
		t.Fatalf("expected structured ids on mapping.missing blocker, got %+v", b)
	}

	// sku.unmatched keeps productId empty (unknown before matching)
	o3 := order.Order{Platform: "tiktok", OrderNo: "SO-5", Status: "paid"}
	if err := db.Create(&o3).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Create(&order.OrderItem{OrderID: o3.ID, SKUName: "unmatched", Quantity: 1}).Error; err != nil {
		t.Fatal(err)
	}
	res, err = f.svc.Generate(context.Background(), GenerateBody{OrderIDs: []string{o3.ID.String()}}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Blockers) != 1 || res.Blockers[0].Code != "sku.unmatched" {
		t.Fatalf("expected sku.unmatched blocker, got %+v", res)
	}
	if res.Blockers[0].ProductID != "" || res.Blockers[0].OrderID != o3.ID.String() {
		t.Fatalf("expected empty productId on sku.unmatched, got %+v", res.Blockers[0])
	}
}

func TestManualFlowHappyPath(t *testing.T) {
	f := setupFixture(t)
	po := generate(t, f, "key-flow").Orders[0]
	ctx := context.Background()

	if _, err := f.svc.Submit(ctx, po.ID, nil); err != nil {
		t.Fatalf("submit: %v", err)
	}
	if _, err := f.svc.Confirm(ctx, po.ID, nil); err != nil {
		t.Fatalf("confirm: %v", err)
	}
	got, err := f.svc.MarkPlaced(ctx, po.ID, MarkPlacedBody{ExternalOrderID: "1688-ORDER-1"}, nil)
	if err != nil {
		t.Fatalf("mark placed: %v", err)
	}
	if got.ExternalOrderID != "1688-ORDER-1" || got.Status != StatusPlaced {
		t.Fatalf("unexpected po %+v", got)
	}
	if got, err = f.svc.MarkPaid(ctx, po.ID, MarkPaidBody{PayChannel: "alipay"}, nil); err != nil || got.PayStatus != PayStatusPaid {
		t.Fatalf("mark paid: %v %+v", err, got)
	}
	if got, err = f.svc.FillLogistics(ctx, po.ID, LogisticsBody{TrackingNo: "SF123", Carrier: "SF"}, nil); err != nil || got.Status != StatusShipped {
		t.Fatalf("fill logistics: %v %+v", err, got)
	}
	if got, err = f.svc.MarkDelivered(ctx, po.ID, nil, nil); err != nil || got.Status != StatusDelivered {
		t.Fatalf("mark delivered: %v %+v", err, got)
	}

	detail, err := f.svc.Detail(ctx, po.ID)
	if err != nil {
		t.Fatal(err)
	}
	// draft→pending_confirm→placing→placed→paid→shipped→delivered plus creation event
	if len(detail.Events) != 7 {
		t.Fatalf("expected 7 events, got %d", len(detail.Events))
	}
	if len(detail.Logistics) != 1 || detail.Logistics[0].TrackingNo != "SF123" || detail.Logistics[0].Status != "delivered" {
		t.Fatalf("unexpected logistics %+v", detail.Logistics)
	}
	if detail.Logistics[0].Carrier != "SF" {
		t.Fatalf("carrier not persisted: %+v", detail.Logistics[0])
	}
	if detail.Logistics[0].TenantID != detail.TenantID {
		t.Fatalf("logistics tenant %d != po tenant %d", detail.Logistics[0].TenantID, detail.TenantID)
	}
	// mock provider knows the manual order
	pay, err := f.svc.Provider.GetPayStatus(ctx, "1688-ORDER-1")
	if err != nil || pay.Status != "paid" {
		t.Fatalf("mock pay status: %v %+v", err, pay)
	}
	lg, err := f.svc.Provider.GetLogistics(ctx, "1688-ORDER-1")
	if err != nil || lg.TrackingNo != "SF123" {
		t.Fatalf("mock logistics: %v %+v", err, lg)
	}
}

// FillLogistics must persist the operator-entered carrier and stamp the
// purchase order's tenant on the logistics row so tenant-scoped reads see it.
func TestFillLogisticsPersistsCarrierAndTenant(t *testing.T) {
	db := openTestDB(t)
	svc := &Service{DB: db, Provider: trade.NewMock1688()}
	po := PurchaseOrder{
		TenantID: 7, SupplierID: uuid.New(), SupplierName: "supplier-b",
		SourcePlatform: "1688", Status: StatusPaid, Currency: "CNY",
		IdempotencyKey: "key-carrier", ExternalOrderID: "1688-ORDER-C",
	}
	if err := db.Create(&po).Error; err != nil {
		t.Fatal(err)
	}
	if _, err := svc.FillLogistics(context.Background(), po.ID, LogisticsBody{TrackingNo: "ZT123", Carrier: " 中通 "}, nil); err != nil {
		t.Fatalf("fill logistics: %v", err)
	}
	detail, err := svc.Detail(context.Background(), po.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(detail.Logistics) != 1 {
		t.Fatalf("expected 1 logistics row, got %d", len(detail.Logistics))
	}
	lg := detail.Logistics[0]
	if lg.Carrier != "中通" {
		t.Fatalf("carrier = %q, want 中通", lg.Carrier)
	}
	if lg.TenantID != 7 {
		t.Fatalf("logistics tenant = %d, want 7", lg.TenantID)
	}
}

func TestIllegalTransitionsRejected(t *testing.T) {
	f := setupFixture(t)
	po := generate(t, f, "key-illegal").Orders[0]
	ctx := context.Background()

	// draft → placed is illegal
	if _, err := f.svc.MarkPlaced(ctx, po.ID, MarkPlacedBody{ExternalOrderID: "X"}, nil); err == nil {
		t.Fatalf("draft→placed must be rejected")
	}
	// draft → paid is illegal
	if _, err := f.svc.MarkPaid(ctx, po.ID, MarkPaidBody{}, nil); err == nil {
		t.Fatalf("draft→paid must be rejected")
	}
	// cancel from draft is legal
	if _, err := f.svc.Cancel(ctx, po.ID, "test", nil); err != nil {
		t.Fatalf("cancel: %v", err)
	}
	// cancelled is terminal
	if _, err := f.svc.Submit(ctx, po.ID, nil); err == nil {
		t.Fatalf("cancelled→pending_confirm must be rejected")
	}
}

func TestExportCSVContains1688Link(t *testing.T) {
	f := setupFixture(t)
	po := generate(t, f, "key-csv").Orders[0]
	data, name, err := f.svc.ExportCSV(context.Background(), po.ID)
	if err != nil {
		t.Fatal(err)
	}
	if name == "" {
		t.Fatalf("expected filename")
	}
	s := string(data)
	for _, want := range []string{"https://detail.1688.com/offer/111.html", "ext-1", "demo product", "9.90", "29.70"} {
		if !containsStr(s, want) {
			t.Fatalf("csv missing %q:\n%s", want, s)
		}
	}
}

func TestExportBatchCSVMergesOrders(t *testing.T) {
	f := setupFixture(t)
	po := generate(t, f, "key-batch-csv").Orders[0]
	data, name, err := f.svc.ExportBatchCSV(context.Background(), []uuid.UUID{po.ID})
	if err != nil {
		t.Fatal(err)
	}
	if name != "purchase-lists-1.csv" {
		t.Fatalf("unexpected filename %q", name)
	}
	s := string(data)
	for _, want := range []string{po.ID.String(), "https://detail.1688.com/offer/111.html", "demo product", "29.70"} {
		if !containsStr(s, want) {
			t.Fatalf("csv missing %q:\n%s", want, s)
		}
	}
	// unknown id fails cleanly
	if _, _, err := f.svc.ExportBatchCSV(context.Background(), []uuid.UUID{uuid.New()}); err == nil {
		t.Fatalf("unknown id must error")
	}
}

func containsStr(s, sub string) bool {
	return len(s) >= len(sub) && (func() bool {
		for i := 0; i+len(sub) <= len(s); i++ {
			if s[i:i+len(sub)] == sub {
				return true
			}
		}
		return false
	})()
}

// setupNoPriceFixture is like setupFixture but the SKU mapping has no price.
func setupNoPriceFixture(t *testing.T, withHistory bool) fixture {
	t.Helper()
	db := openTestDB(t)
	svc := &Service{DB: db, Provider: trade.NewMock1688()}

	sup := sourcing.Supplier{Platform: "1688", Name: "supplier-np", Status: "active"}
	if err := db.Create(&sup).Error; err != nil {
		t.Fatal(err)
	}
	productID := uuid.New()
	localSKU := uuid.New()
	src := sourcing.ProductSource{
		ProductID: productID, SupplierID: sup.ID, IsPrimary: true, Priority: 10,
		Status: sourcing.SourceStatusActive, SourceOfferID: "222",
		SourceURL: "https://detail.1688.com/offer/222.html",
	}
	if err := db.Create(&src).Error; err != nil {
		t.Fatal(err)
	}
	mapping := sourcing.ProductSourceSKU{
		ProductSourceID: src.ID, LocalSKUID: localSKU, ExternalSKUID: "ext-np",
		Currency: "CNY", Status: "active",
	}
	if err := db.Create(&mapping).Error; err != nil {
		t.Fatal(err)
	}
	if withHistory {
		h := sourcing.SourcePriceHistory{
			SourceSKUID: mapping.ID, Price: 18.5,
			CaptureSource: sourcing.CaptureSourceManual,
		}
		if err := db.Create(&h).Error; err != nil {
			t.Fatal(err)
		}
	}
	o := order.Order{TenantID: 0, Platform: "tiktok", OrderNo: "SO-NP", Status: "paid", Currency: "USD"}
	if err := db.Create(&o).Error; err != nil {
		t.Fatal(err)
	}
	item := order.OrderItem{
		OrderID: o.ID, ProductID: &productID, ProductSKUID: &localSKU,
		ProductTitle: "no-price product", SKUName: "blue / M", Quantity: 2,
	}
	if err := db.Create(&item).Error; err != nil {
		t.Fatal(err)
	}
	return fixture{svc: svc, orderID: o.ID}
}

func TestGenerateFallsBackToPriceHistory(t *testing.T) {
	f := setupNoPriceFixture(t, true)
	res := generate(t, f, "key-hist")
	if len(res.Orders) != 1 || len(res.Warnings) != 0 {
		t.Fatalf("expected 1 order without warnings, got %+v", res)
	}
	if res.Orders[0].TotalAmount != 37.0 {
		t.Fatalf("expected total 37.0 from history price, got %v", res.Orders[0].TotalAmount)
	}
}

func TestGenerateWarnsOnMissingPrice(t *testing.T) {
	f := setupNoPriceFixture(t, false)
	res := generate(t, f, "key-noprice")
	if len(res.Orders) != 1 {
		t.Fatalf("expected 1 order, got %+v", res)
	}
	if len(res.Warnings) != 1 || res.Warnings[0].Code != "price.missing" {
		t.Fatalf("expected price.missing warning, got %+v", res.Warnings)
	}
	if res.Orders[0].TotalAmount != 0 {
		t.Fatalf("expected total 0, got %v", res.Orders[0].TotalAmount)
	}
}

func TestUpdateItemPriceRecomputesTotal(t *testing.T) {
	f := setupNoPriceFixture(t, false)
	ctx := context.Background()
	po := generate(t, f, "key-editprice").Orders[0]
	detail, err := f.svc.Detail(ctx, po.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(detail.Items) != 1 {
		t.Fatalf("expected 1 item, got %d", len(detail.Items))
	}
	itemID := detail.Items[0].ID

	// invalid price rejected
	if _, err := f.svc.UpdateItemPrice(ctx, po.ID, itemID, 0, nil); err == nil {
		t.Fatalf("zero price must be rejected")
	}
	got, err := f.svc.UpdateItemPrice(ctx, po.ID, itemID, 18.5, nil)
	if err != nil {
		t.Fatalf("update price: %v", err)
	}
	if got.TotalAmount != 37.0 {
		t.Fatalf("expected total 37.0, got %v", got.TotalAmount)
	}
	if len(got.Items) != 1 || got.Items[0].ExpectedPrice == nil || *got.Items[0].ExpectedPrice != 18.5 {
		t.Fatalf("expected item price 18.5, got %+v", got.Items)
	}

	// only draft/pending_confirm may edit
	if _, err := f.svc.Submit(ctx, po.ID, nil); err != nil {
		t.Fatal(err)
	}
	if _, err := f.svc.UpdateItemPrice(ctx, po.ID, itemID, 20, nil); err != nil {
		t.Fatalf("pending_confirm should allow edit: %v", err)
	}
	if _, err := f.svc.Confirm(ctx, po.ID, nil); err != nil {
		t.Fatal(err)
	}
	if _, err := f.svc.UpdateItemPrice(ctx, po.ID, itemID, 21, nil); err == nil {
		t.Fatalf("placing must reject price edit")
	}
}

func TestStateMachineTable(t *testing.T) {
	cases := []struct {
		from, to string
		ok       bool
	}{
		{StatusDraft, StatusPendingConfirm, true},
		{StatusPendingConfirm, StatusPlacing, true},
		{StatusPlacing, StatusPlaced, true},
		{StatusPlaced, StatusPaid, true},
		{StatusPaid, StatusShipped, true},
		{StatusShipped, StatusDelivered, true},
		{StatusFailed, StatusPlacing, true},
		{StatusDraft, StatusPaid, false},
		{StatusDelivered, StatusDraft, false},
		{StatusCancelled, StatusPlacing, false},
	}
	for _, c := range cases {
		if got := CanTransition(c.from, c.to); got != c.ok {
			t.Fatalf("%s→%s expected %v", c.from, c.to, c.ok)
		}
	}
}

func TestListFilterBySalesOrderID(t *testing.T) {
	f := setupFixture(t)
	generate(t, f, "key-sales-filter")

	res, err := f.svc.List(context.Background(), ListQuery{SalesOrderID: f.orderID.String()})
	if err != nil {
		t.Fatalf("list by salesOrderId: %v", err)
	}
	if res.Total != 1 || len(res.Items) != 1 {
		t.Fatalf("expected 1 purchase order for sales order, got %+v", res)
	}

	other, err := f.svc.List(context.Background(), ListQuery{SalesOrderID: uuid.NewString()})
	if err != nil {
		t.Fatalf("list by unknown salesOrderId: %v", err)
	}
	if other.Total != 0 || len(other.Items) != 0 {
		t.Fatalf("expected empty result for unrelated sales order, got %+v", other)
	}

	if _, err := f.svc.List(context.Background(), ListQuery{SalesOrderID: "not-a-uuid"}); err == nil {
		t.Fatal("expected error for invalid salesOrderId")
	}
}
