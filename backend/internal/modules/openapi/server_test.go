package openapi_test

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"github.com/google/uuid"
	"github.com/trademind-ai/trademind/backend/internal/modules/mcpaudit"
	"github.com/trademind-ai/trademind/backend/internal/modules/mcptoken"
	"github.com/trademind-ai/trademind/backend/internal/modules/openapi"
	"github.com/trademind-ai/trademind/backend/internal/modules/order"
	"github.com/trademind-ai/trademind/backend/internal/modules/product"
	"gorm.io/gorm"
)

func openTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	dsn := fmt.Sprintf("file:openapi_%s?mode=memory&cache=shared&_pragma=busy_timeout(5000)", uuid.New().String())
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Skipf("sqlite unavailable: %v", err)
	}
	sqlDB, _ := db.DB()
	sqlDB.SetMaxOpenConns(1)
	if err := db.AutoMigrate(&mcptoken.Token{}, &mcpaudit.ToolCallLog{},
		&order.Order{}, &order.OrderItem{}, &product.Product{}, &product.ProductSKU{}); err != nil {
		t.Fatal(err)
	}
	return db
}

func newTestServer(t *testing.T, db *gorm.DB, rps float64, burst int) (*httptest.Server, *mcptoken.Service) {
	t.Helper()
	gin.SetMode(gin.TestMode)
	tokens := &mcptoken.Service{DB: db}
	r := gin.New()
	openapi.Register(r, &openapi.Deps{
		DB:        db,
		Tokens:    tokens,
		Audits:    &mcpaudit.Service{DB: db},
		RateRPS:   rps,
		RateBurst: burst,
	})
	srv := httptest.NewServer(r)
	t.Cleanup(srv.Close)
	return srv, tokens
}

func get(t *testing.T, srv *httptest.Server, path, token string) (*http.Response, map[string]any) {
	t.Helper()
	req, err := http.NewRequest(http.MethodGet, srv.URL+path, nil)
	if err != nil {
		t.Fatal(err)
	}
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	var body map[string]any
	_ = json.NewDecoder(res.Body).Decode(&body)
	return res, body
}

func seedOrders(t *testing.T, db *gorm.DB) {
	t.Helper()
	now := time.Now().UTC()
	rows := []order.Order{
		{TenantID: 1, Platform: "douyin", OrderNo: "T1-001", CustomerName: "张三丰", Status: "completed",
			PaymentStatus: "paid", FulfillmentStatus: "delivered", Currency: "CNY", TotalAmount: 199, PaidAt: &now},
		{TenantID: 1, Platform: "douyin", OrderNo: "T1-002", CustomerName: "李四", Status: "pending",
			PaymentStatus: "unpaid", FulfillmentStatus: "pending", Currency: "CNY", TotalAmount: 88},
		{TenantID: 2, Platform: "douyin", OrderNo: "T2-001", CustomerName: "王五", Status: "completed",
			PaymentStatus: "paid", FulfillmentStatus: "delivered", Currency: "USD", TotalAmount: 30},
	}
	for i := range rows {
		if err := db.Create(&rows[i]).Error; err != nil {
			t.Fatal(err)
		}
	}
	item := order.OrderItem{OrderID: rows[0].ID, ProductTitle: "演示商品", SKUCode: "SKU-1", Quantity: 2, UnitPrice: 99.5, TotalPrice: 199}
	if err := db.Create(&item).Error; err != nil {
		t.Fatal(err)
	}
}

func TestAuthRequiredAndPurpose(t *testing.T) {
	db := openTestDB(t)
	srv, tokens := newTestServer(t, db, 100, 100)

	res, body := get(t, srv, "/api/open/v1/orders", "")
	if res.StatusCode != http.StatusUnauthorized {
		t.Fatalf("missing token: want 401 got %d", res.StatusCode)
	}
	if code, _ := body["code"].(float64); code != 40101 {
		t.Fatalf("want business code 40101, got %v", body["code"])
	}

	// mcp-purpose tokens must not authenticate the Open API entry.
	mcpTok, err := tokens.Create(context.Background(), 1, "mcp-only", mcptoken.PurposeMCP, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	res, _ = get(t, srv, "/api/open/v1/orders", mcpTok.Plaintext)
	if res.StatusCode != http.StatusUnauthorized {
		t.Fatalf("mcp-purpose token: want 401 got %d", res.StatusCode)
	}

	// both-purpose tokens authenticate both entries.
	bothTok, err := tokens.Create(context.Background(), 1, "both", mcptoken.PurposeBoth, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	res, _ = get(t, srv, "/api/open/v1/orders", bothTok.Plaintext)
	if res.StatusCode != http.StatusOK {
		t.Fatalf("both-purpose token: want 200 got %d", res.StatusCode)
	}

	// revoked tokens stop authenticating.
	openTok, err := tokens.Create(context.Background(), 1, "revoked", mcptoken.PurposeOpenAPI, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := tokens.Revoke(context.Background(), 1, openTok.Token.ID); err != nil {
		t.Fatal(err)
	}
	res, _ = get(t, srv, "/api/open/v1/orders", openTok.Plaintext)
	if res.StatusCode != http.StatusUnauthorized {
		t.Fatalf("revoked token: want 401 got %d", res.StatusCode)
	}
}

func TestOrdersTenantScopeAndMasking(t *testing.T) {
	db := openTestDB(t)
	seedOrders(t, db)
	srv, tokens := newTestServer(t, db, 100, 100)
	tok, err := tokens.Create(context.Background(), 1, "t1", mcptoken.PurposeOpenAPI, nil, nil)
	if err != nil {
		t.Fatal(err)
	}

	res, body := get(t, srv, "/api/open/v1/orders", tok.Plaintext)
	if res.StatusCode != http.StatusOK {
		t.Fatalf("want 200 got %d", res.StatusCode)
	}
	data := body["data"].(map[string]any)
	if total := data["total"].(float64); total != 2 {
		t.Fatalf("tenant 1 should see 2 orders, got %v", total)
	}
	for _, it := range data["list"].([]any) {
		row := it.(map[string]any)
		if row["orderNo"] == "T2-001" {
			t.Fatal("cross-tenant order leaked")
		}
		if name, _ := row["customerName"].(string); name == "张三丰" || name == "李四" {
			t.Fatalf("customer name not masked: %q", name)
		}
	}
}

func TestOrderDetailCrossTenant404(t *testing.T) {
	db := openTestDB(t)
	seedOrders(t, db)
	srv, tokens := newTestServer(t, db, 100, 100)
	tok, err := tokens.Create(context.Background(), 1, "t1", mcptoken.PurposeOpenAPI, nil, nil)
	if err != nil {
		t.Fatal(err)
	}

	res, body := get(t, srv, "/api/open/v1/orders/T1-001", tok.Plaintext)
	if res.StatusCode != http.StatusOK {
		t.Fatalf("own order: want 200 got %d", res.StatusCode)
	}
	data := body["data"].(map[string]any)
	items := data["items"].([]any)
	if len(items) != 1 {
		t.Fatalf("want 1 line item, got %d", len(items))
	}

	// Another tenant's order number answers exactly like a missing one.
	res, body = get(t, srv, "/api/open/v1/orders/T2-001", tok.Plaintext)
	if res.StatusCode != http.StatusNotFound {
		t.Fatalf("cross-tenant order: want 404 got %d", res.StatusCode)
	}
	if code, _ := body["code"].(float64); code != 40401 {
		t.Fatalf("want business code 40401, got %v", body["code"])
	}
}

func TestReportsAndExceptionsEndpoints(t *testing.T) {
	db := openTestDB(t)
	seedOrders(t, db)
	srv, tokens := newTestServer(t, db, 100, 100)
	tok, err := tokens.Create(context.Background(), 1, "t1", mcptoken.PurposeOpenAPI, nil, nil)
	if err != nil {
		t.Fatal(err)
	}

	res, body := get(t, srv, "/api/open/v1/reports/summary", tok.Plaintext)
	if res.StatusCode != http.StatusOK {
		t.Fatalf("summary: want 200 got %d", res.StatusCode)
	}
	data := body["data"].(map[string]any)
	if oc := data["orderCount"].(float64); oc != 2 {
		t.Fatalf("tenant 1 summary should count 2 orders, got %v", oc)
	}
	if pc := data["paidOrderCount"].(float64); pc != 1 {
		t.Fatalf("tenant 1 summary should count 1 paid order, got %v", pc)
	}

	res, body = get(t, srv, "/api/open/v1/reports/summary?startDate=bogus", tok.Plaintext)
	if res.StatusCode != http.StatusBadRequest {
		t.Fatalf("bad date: want 400 got %d", res.StatusCode)
	}
	if code, _ := body["code"].(float64); code != 40001 {
		t.Fatalf("want business code 40001, got %v", body["code"])
	}
}

func TestInventoryTenantScope(t *testing.T) {
	db := openTestDB(t)
	srv, tokens := newTestServer(t, db, 100, 100)
	stock1, stock2 := 3, 500
	p1 := product.Product{TenantID: 1, Title: "T1 商品", Status: "draft",
		SKUs: []product.ProductSKU{{SKUCode: "T1-SKU", Stock: &stock1, WarningStock: 10, StockStatus: "low"}}}
	p2 := product.Product{TenantID: 2, Title: "T2 商品", Status: "draft",
		SKUs: []product.ProductSKU{{SKUCode: "T2-SKU", Stock: &stock2, WarningStock: 10}}}
	if err := db.Create(&p1).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Create(&p2).Error; err != nil {
		t.Fatal(err)
	}
	tok, err := tokens.Create(context.Background(), 1, "t1", mcptoken.PurposeOpenAPI, nil, nil)
	if err != nil {
		t.Fatal(err)
	}

	res, body := get(t, srv, "/api/open/v1/inventory?lowStockOnly=true", tok.Plaintext)
	if res.StatusCode != http.StatusOK {
		t.Fatalf("want 200 got %d", res.StatusCode)
	}
	data := body["data"].(map[string]any)
	list := data["list"].([]any)
	if len(list) != 1 {
		t.Fatalf("tenant 1 should see exactly its own low-stock SKU, got %d", len(list))
	}
	if sku := list[0].(map[string]any)["skuCode"]; sku != "T1-SKU" {
		t.Fatalf("unexpected SKU %v", sku)
	}
}

func TestReadonlySurfaceRejectsWrites(t *testing.T) {
	db := openTestDB(t)
	srv, tokens := newTestServer(t, db, 100, 100)
	tok, err := tokens.Create(context.Background(), 1, "t1", mcptoken.PurposeOpenAPI, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	for _, method := range []string{http.MethodPost, http.MethodPut, http.MethodPatch, http.MethodDelete} {
		req, err := http.NewRequest(method, srv.URL+"/api/open/v1/orders", nil)
		if err != nil {
			t.Fatal(err)
		}
		req.Header.Set("Authorization", "Bearer "+tok.Plaintext)
		res, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		res.Body.Close()
		if res.StatusCode != http.StatusNotFound && res.StatusCode != http.StatusMethodNotAllowed {
			t.Fatalf("%s must not be served, got %d", method, res.StatusCode)
		}
	}
}

func TestRateLimit429(t *testing.T) {
	db := openTestDB(t)
	srv, tokens := newTestServer(t, db, 1, 2)
	tok, err := tokens.Create(context.Background(), 1, "burst", mcptoken.PurposeOpenAPI, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	limited := false
	for i := 0; i < 10; i++ {
		res, body := get(t, srv, "/api/open/v1/orders", tok.Plaintext)
		if res.StatusCode == http.StatusTooManyRequests {
			if code, _ := body["code"].(float64); code != 42901 {
				t.Fatalf("want business code 42901, got %v", body["code"])
			}
			if res.Header.Get("Retry-After") == "" {
				t.Fatal("429 must carry Retry-After")
			}
			limited = true
			break
		}
	}
	if !limited {
		t.Fatal("expected a 429 within 10 rapid calls")
	}
}

func TestAuditTrailPerCall(t *testing.T) {
	db := openTestDB(t)
	seedOrders(t, db)
	srv, tokens := newTestServer(t, db, 100, 100)
	tok, err := tokens.Create(context.Background(), 1, "audited", mcptoken.PurposeOpenAPI, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	if res, _ := get(t, srv, "/api/open/v1/orders", tok.Plaintext); res.StatusCode != http.StatusOK {
		t.Fatalf("want 200 got %d", res.StatusCode)
	}
	if res, _ := get(t, srv, "/api/open/v1/orders/none-such", tok.Plaintext); res.StatusCode != http.StatusNotFound {
		t.Fatalf("want 404 got %d", res.StatusCode)
	}
	var logs []mcpaudit.ToolCallLog
	if err := db.Order("created_at").Find(&logs).Error; err != nil {
		t.Fatal(err)
	}
	if len(logs) != 2 {
		t.Fatalf("want 2 audit rows, got %d", len(logs))
	}
	if logs[0].Tool != "openapi:orders_list" || logs[0].Status != mcpaudit.StatusSuccess {
		t.Fatalf("unexpected first audit row: %+v", logs[0])
	}
	if logs[1].Tool != "openapi:orders_detail" || logs[1].Status != mcpaudit.StatusError {
		t.Fatalf("unexpected second audit row: %+v", logs[1])
	}
	for _, l := range logs {
		if l.TenantID != 1 {
			t.Fatalf("audit row not tenant scoped: %+v", l)
		}
	}
}
