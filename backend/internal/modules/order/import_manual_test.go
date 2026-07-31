package order_test

import (
	"fmt"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"github.com/google/uuid"
	"github.com/trademind-ai/trademind/backend/internal/modules/order"
	"github.com/trademind-ai/trademind/backend/internal/modules/product"
	"github.com/trademind-ai/trademind/backend/internal/modules/shop"
	"github.com/trademind-ai/trademind/backend/internal/pkg/ctxkey"
	"gorm.io/gorm"
)

func openImportTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	dsn := fmt.Sprintf("file:order_import_%s?mode=memory&cache=shared&_pragma=busy_timeout(5000)", uuid.New().String())
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Skipf("sqlite unavailable: %v", err)
	}
	sqlDB, _ := db.DB()
	sqlDB.SetMaxOpenConns(1)
	if err := db.AutoMigrate(&order.Order{}, &order.OrderItem{}, &order.OrderShipment{}, &shop.Shop{}); err != nil {
		t.Fatal(err)
	}
	return db
}

func importTestCtx(tenantID int64) *gin.Context {
	gin.SetMode(gin.TestMode)
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Request = httptest.NewRequest("POST", "/api/v1/orders/import", nil)
	c.Set(ctxkey.TenantID, tenantID)
	return c
}

func importOrderBody(orderNo string) order.CreateBody {
	return order.CreateBody{
		OrderNo:      orderNo,
		CustomerName: "客户A",
		Currency:     "USD",
		TotalAmount:  20,
		Items: []order.OrderItemInput{
			{ProductTitle: "测试商品", SKUCode: "SKU-1", Quantity: 2, UnitPrice: 10, TotalPrice: 20},
		},
	}
}

func TestImportOrdersCreatesAndDedupes(t *testing.T) {
	db := openImportTestDB(t)
	svc := &order.Service{DB: db}
	c := importTestCtx(1)

	sum, err := svc.ImportOrders(c, order.ImportBody{Orders: []order.CreateBody{
		importOrderBody("SO-001"),
		importOrderBody("SO-002"),
		importOrderBody("SO-001"), // duplicate within batch
	}}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if sum.Created != 2 || sum.Duplicate != 1 || sum.Failed != 0 {
		t.Fatalf("unexpected summary: %+v", sum)
	}

	// Re-import: everything already in DB is skipped.
	sum2, err := svc.ImportOrders(c, order.ImportBody{Orders: []order.CreateBody{
		importOrderBody("SO-001"),
		importOrderBody("SO-002"),
	}}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if sum2.Created != 0 || sum2.Duplicate != 2 {
		t.Fatalf("unexpected re-import summary: %+v", sum2)
	}

	var count int64
	db.Model(&order.Order{}).Count(&count)
	if count != 2 {
		t.Fatalf("expected 2 orders, got %d", count)
	}
}

func TestImportOrdersRowFailureDoesNotAbort(t *testing.T) {
	db := openImportTestDB(t)
	svc := &order.Service{DB: db}
	c := importTestCtx(1)

	bad := importOrderBody("SO-BAD")
	bad.CustomerName = ""
	sum, err := svc.ImportOrders(c, order.ImportBody{Orders: []order.CreateBody{
		bad,
		importOrderBody("SO-OK"),
	}}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if sum.Created != 1 || sum.Failed != 1 {
		t.Fatalf("unexpected summary: %+v", sum)
	}
	if sum.Results[0].Status != order.ImportRowFailed || sum.Results[0].Error == "" {
		t.Fatalf("expected failed row with error, got %+v", sum.Results[0])
	}
}

func TestImportOrdersMatchSkusManualOrder(t *testing.T) {
	db := openImportTestDB(t)
	if err := db.AutoMigrate(&product.ProductSKU{}, &order.OrderItemSKUMatch{}); err != nil {
		t.Fatal(err)
	}
	if err := db.Create(&product.ProductSKU{ProductID: uuid.New(), SKUCode: "SKU-1"}).Error; err != nil {
		t.Fatal(err)
	}
	svc := &order.Service{DB: db}
	sum, err := svc.ImportOrders(importTestCtx(1), order.ImportBody{
		Orders:    []order.CreateBody{importOrderBody("SO-MATCH")},
		MatchSKUs: true,
	}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if sum.Created != 1 {
		t.Fatalf("unexpected summary: %+v", sum)
	}
	if sum.Results[0].ItemsMatched != 1 {
		t.Fatalf("expected 1 matched item for manual order, got %+v", sum.Results[0])
	}
}

func TestMatchOrderItemsRematchUpdatesExistingRow(t *testing.T) {
	db := openImportTestDB(t)
	if err := db.AutoMigrate(&product.ProductSKU{}, &order.OrderItemSKUMatch{}); err != nil {
		t.Fatal(err)
	}
	svc := &order.Service{DB: db}
	c := importTestCtx(1)
	sum, err := svc.ImportOrders(c, order.ImportBody{
		Orders:    []order.CreateBody{importOrderBody("SO-REMATCH")},
		MatchSKUs: true,
	}, nil)
	if err != nil || sum.Created != 1 {
		t.Fatalf("import failed: %v %+v", err, sum)
	}
	if sum.Results[0].ItemsMatched != 0 {
		t.Fatalf("expected no match before SKU exists, got %+v", sum.Results[0])
	}
	// SKU now exists; re-match must update the existing skipped/unmatched row.
	if err := db.Create(&product.ProductSKU{ProductID: uuid.New(), SKUCode: "SKU-1"}).Error; err != nil {
		t.Fatal(err)
	}
	ms, err := svc.MatchOrderItemsForOrder(c.Request.Context(), *sum.Results[0].OrderID, order.MatchOrderItemsOptions{Source: "rematch_test"})
	if err != nil {
		t.Fatal(err)
	}
	if ms.Matched != 1 {
		t.Fatalf("expected rematch to update existing row to matched, got %+v", ms)
	}
	var row order.OrderItemSKUMatch
	if err := db.First(&row, "order_id = ?", *sum.Results[0].OrderID).Error; err != nil {
		t.Fatal(err)
	}
	if row.MatchStatus != order.MatchStatusMatched || row.ProductSKUID == nil {
		t.Fatalf("expected persisted matched row, got %+v", row)
	}
}

func TestImportOrdersEmptyRejected(t *testing.T) {
	db := openImportTestDB(t)
	svc := &order.Service{DB: db}
	if _, err := svc.ImportOrders(importTestCtx(1), order.ImportBody{}, nil); err == nil {
		t.Fatal("expected error for empty orders")
	}
}
