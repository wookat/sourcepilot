package mcpserver_test

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"github.com/google/uuid"
	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/trademind-ai/trademind/backend/internal/modules/mcpserver"
	"github.com/trademind-ai/trademind/backend/internal/modules/mcptoken"
	"github.com/trademind-ai/trademind/backend/internal/modules/order"
	"github.com/trademind-ai/trademind/backend/internal/modules/product"
	"gorm.io/gorm"
)

func openTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	dsn := fmt.Sprintf("file:mcpserver_%s?mode=memory&cache=shared&_pragma=busy_timeout(5000)", uuid.New().String())
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Skipf("sqlite unavailable: %v", err)
	}
	sqlDB, _ := db.DB()
	sqlDB.SetMaxOpenConns(1)
	if err := db.AutoMigrate(&mcptoken.Token{}, &order.Order{}, &product.Product{}, &product.ProductSKU{}); err != nil {
		t.Fatal(err)
	}
	return db
}

func newTestServer(t *testing.T, db *gorm.DB, rps float64, burst int) (*httptest.Server, *mcptoken.Service) {
	t.Helper()
	gin.SetMode(gin.TestMode)
	tokens := &mcptoken.Service{DB: db}
	r := gin.New()
	r.POST("/api/mcp", mcpserver.GinHandler(&mcpserver.Deps{
		DB:        db,
		Tokens:    tokens,
		RateRPS:   rps,
		RateBurst: burst,
		Version:   "test",
	}))
	srv := httptest.NewServer(r)
	t.Cleanup(srv.Close)
	return srv, tokens
}

type authTransport struct {
	token string
}

func (a authTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	req.Header.Set("Authorization", "Bearer "+a.token)
	return http.DefaultTransport.RoundTrip(req)
}

func connect(t *testing.T, url, token string) *mcp.ClientSession {
	t.Helper()
	client := mcp.NewClient(&mcp.Implementation{Name: "test-client", Version: "0.0.1"}, nil)
	sess, err := client.Connect(context.Background(), &mcp.StreamableClientTransport{
		Endpoint:   url + "/api/mcp",
		HTTPClient: &http.Client{Transport: authTransport{token: token}},
		MaxRetries: -1,
	}, nil)
	if err != nil {
		t.Fatalf("connect: %v", err)
	}
	t.Cleanup(func() { _ = sess.Close() })
	return sess
}

func seedOrders(t *testing.T, db *gorm.DB) {
	t.Helper()
	rows := []order.Order{
		{TenantID: 1, OrderNo: "T1-A", Platform: "douyin", Status: "pending", PaymentStatus: "paid", Currency: "CNY", TotalAmount: 100, CustomerName: "张三"},
		{TenantID: 1, OrderNo: "T1-B", Platform: "douyin", Status: "completed", PaymentStatus: "unpaid", Currency: "CNY", TotalAmount: 50, CustomerName: "李四"},
		{TenantID: 2, OrderNo: "T2-A", Platform: "shopee", Status: "pending", PaymentStatus: "paid", Currency: "USD", TotalAmount: 30, CustomerName: "Alice"},
	}
	for i := range rows {
		if err := db.Create(&rows[i]).Error; err != nil {
			t.Fatal(err)
		}
	}
}

func TestUnauthorizedRejected(t *testing.T) {
	srv, tokens := newTestServer(t, openTestDB(t), 100, 100)

	// Missing token.
	resp, err := http.Post(srv.URL+"/api/mcp", "application/json", strings.NewReader("{}"))
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("missing token: status %d, want 401", resp.StatusCode)
	}

	// Malformed token.
	req, _ := http.NewRequest(http.MethodPost, srv.URL+"/api/mcp", strings.NewReader("{}"))
	req.Header.Set("Authorization", "Bearer not-a-token")
	resp, err = http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("malformed token: status %d, want 401", resp.StatusCode)
	}

	// Revoked token.
	res, err := tokens.Create(context.Background(), 1, "revoked", nil)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := tokens.Revoke(context.Background(), 1, res.Token.ID); err != nil {
		t.Fatal(err)
	}
	req, _ = http.NewRequest(http.MethodPost, srv.URL+"/api/mcp", strings.NewReader("{}"))
	req.Header.Set("Authorization", "Bearer "+res.Plaintext)
	resp, err = http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("revoked token: status %d, want 401", resp.StatusCode)
	}
}

func TestToolsAreReadOnly(t *testing.T) {
	srv, tokens := newTestServer(t, openTestDB(t), 100, 100)
	res, err := tokens.Create(context.Background(), 1, "list", nil)
	if err != nil {
		t.Fatal(err)
	}
	sess := connect(t, srv.URL, res.Plaintext)
	tools, err := sess.ListTools(context.Background(), nil)
	if err != nil {
		t.Fatal(err)
	}
	want := map[string]bool{
		"orders_query":       false,
		"inventory_query":    false,
		"report_summary":     false,
		"exceptions_pending": false,
	}
	for _, tool := range tools.Tools {
		if _, ok := want[tool.Name]; !ok {
			t.Fatalf("unexpected tool exposed: %s", tool.Name)
		}
		want[tool.Name] = true
	}
	for name, seen := range want {
		if !seen {
			t.Fatalf("tool %s not exposed", name)
		}
	}
}

func decodeStructured[T any](t *testing.T, out *mcp.CallToolResult) T {
	t.Helper()
	var v T
	raw, err := json.Marshal(out.StructuredContent)
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(raw, &v); err != nil {
		t.Fatal(err)
	}
	return v
}

func TestOrdersQueryTenantScopeAndMasking(t *testing.T) {
	db := openTestDB(t)
	seedOrders(t, db)
	srv, tokens := newTestServer(t, db, 100, 100)
	res, err := tokens.Create(context.Background(), 1, "orders", nil)
	if err != nil {
		t.Fatal(err)
	}
	sess := connect(t, srv.URL, res.Plaintext)
	out, err := sess.CallTool(context.Background(), &mcp.CallToolParams{
		Name:      "orders_query",
		Arguments: map[string]any{},
	})
	if err != nil {
		t.Fatal(err)
	}
	got := decodeStructured[mcpserver.OrdersQueryOut](t, out)
	if got.Total != 2 {
		t.Fatalf("total = %d, want 2 (tenant 1 only)", got.Total)
	}
	for _, item := range got.Items {
		if strings.HasPrefix(item.OrderNo, "T2-") {
			t.Fatalf("cross-tenant order leaked: %s", item.OrderNo)
		}
		if item.CustomerName == "张三" || item.CustomerName == "李四" {
			t.Fatalf("customer name not masked: %s", item.CustomerName)
		}
	}
}

func TestReportSummaryTenantScope(t *testing.T) {
	db := openTestDB(t)
	seedOrders(t, db)
	srv, tokens := newTestServer(t, db, 100, 100)
	res, err := tokens.Create(context.Background(), 2, "report", nil)
	if err != nil {
		t.Fatal(err)
	}
	sess := connect(t, srv.URL, res.Plaintext)
	today := time.Now().UTC().Format("2006-01-02")
	out, err := sess.CallTool(context.Background(), &mcp.CallToolParams{
		Name:      "report_summary",
		Arguments: map[string]any{"startDate": today, "endDate": today},
	})
	if err != nil {
		t.Fatal(err)
	}
	got := decodeStructured[mcpserver.ReportSummaryOut](t, out)
	if got.OrderCount != 1 || got.PaidOrderCount != 1 {
		t.Fatalf("counts = %d/%d, want 1/1 (tenant 2 only)", got.OrderCount, got.PaidOrderCount)
	}
	if len(got.SalesByCurrency) != 1 || got.SalesByCurrency[0].Currency != "USD" || got.SalesByCurrency[0].PaidAmount != 30 {
		t.Fatalf("sales = %+v, want USD 30", got.SalesByCurrency)
	}
}

func TestInventoryQueryTenantScope(t *testing.T) {
	db := openTestDB(t)
	p1 := product.Product{TenantID: 1, Title: "货品一"}
	p2 := product.Product{TenantID: 2, Title: "货品二"}
	if err := db.Create(&p1).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Create(&p2).Error; err != nil {
		t.Fatal(err)
	}
	stock1, stock2 := 3, 50
	if err := db.Create(&product.ProductSKU{ProductID: p1.ID, SKUCode: "SKU-1", Stock: &stock1, WarningStock: 5}).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Create(&product.ProductSKU{ProductID: p2.ID, SKUCode: "SKU-2", Stock: &stock2, WarningStock: 5}).Error; err != nil {
		t.Fatal(err)
	}
	srv, tokens := newTestServer(t, db, 100, 100)
	res, err := tokens.Create(context.Background(), 1, "inventory", nil)
	if err != nil {
		t.Fatal(err)
	}
	sess := connect(t, srv.URL, res.Plaintext)
	out, err := sess.CallTool(context.Background(), &mcp.CallToolParams{
		Name:      "inventory_query",
		Arguments: map[string]any{"lowStockOnly": true},
	})
	if err != nil {
		t.Fatal(err)
	}
	got := decodeStructured[mcpserver.InventoryQueryOut](t, out)
	if got.Total != 1 || len(got.Items) != 1 || got.Items[0].SKUCode != "SKU-1" {
		t.Fatalf("inventory = %+v, want only SKU-1", got)
	}
}

func TestRateLimit(t *testing.T) {
	srv, tokens := newTestServer(t, openTestDB(t), 1, 2)
	res, err := tokens.Create(context.Background(), 1, "burst", nil)
	if err != nil {
		t.Fatal(err)
	}
	saw429 := false
	for i := 0; i < 6; i++ {
		req, _ := http.NewRequest(http.MethodPost, srv.URL+"/api/mcp", strings.NewReader("{}"))
		req.Header.Set("Authorization", "Bearer "+res.Plaintext)
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		resp.Body.Close()
		if resp.StatusCode == http.StatusTooManyRequests {
			saw429 = true
			break
		}
	}
	if !saw429 {
		t.Fatal("rate limit never triggered")
	}
}
