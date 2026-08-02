package product

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"github.com/google/uuid"
	"github.com/trademind-ai/trademind/backend/internal/pkg/adminperm"
	"github.com/trademind-ai/trademind/backend/internal/pkg/ctxkey"
	"gorm.io/gorm"
)

func openListingExportTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	dsn := fmt.Sprintf("file:product_export_%s?mode=memory&cache=shared&_pragma=busy_timeout(5000)", uuid.New().String())
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Skipf("sqlite unavailable: %v", err)
	}
	sqlDB, _ := db.DB()
	sqlDB.SetMaxOpenConns(1)
	if err := db.AutoMigrate(&Product{}, &ProductImage{}, &ProductSKU{}, &ProductPlatformPublishConfig{}); err != nil {
		t.Fatal(err)
	}
	if err := db.Exec(`CREATE TABLE IF NOT EXISTS product_publications (
		id char(36) PRIMARY KEY, product_id char(36), shop_id char(36), deleted_at datetime
	)`).Error; err != nil {
		t.Fatal(err)
	}
	return db
}

func listingExportTestCtx(tenantID int64, principal *adminperm.Principal) *gin.Context {
	gin.SetMode(gin.TestMode)
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Request = httptest.NewRequest(http.MethodGet, "/api/v1/products/listing-list/export.csv", nil)
	c.Set(ctxkey.TenantID, tenantID)
	if principal != nil {
		c.Set("adminperm.principal", principal)
	}
	return c
}

func createListingExportProduct(t *testing.T, db *gorm.DB, tenantID int64, title string) *Product {
	t.Helper()
	p := &Product{
		TenantID:  tenantID,
		Source:    "1688",
		SourceURL: "https://detail.1688.com/offer/123.html",
		Title:     title,
		AITitle:   title + " AI",
		Currency:  "USD",
		Status:    StatusDraft,
	}
	if err := db.Create(p).Error; err != nil {
		t.Fatal(err)
	}
	price := 12.5
	stock := 7
	if err := db.Create(&ProductSKU{ProductID: p.ID, SKUName: "Red", Price: &price, Stock: &stock}).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Create(&ProductImage{ProductID: p.ID, ImageType: ImageTypeMain, PublicURL: "https://img.example.com/" + p.ID.String() + ".jpg", SortOrder: 1}).Error; err != nil {
		t.Fatal(err)
	}
	return p
}

func TestExportListingListCSV(t *testing.T) {
	db := openListingExportTestDB(t)
	svc := &Service{DB: db}
	a := createListingExportProduct(t, db, 1, "Draft A")
	b := createListingExportProduct(t, db, 1, "Draft B")
	if err := db.Create(&ProductPlatformPublishConfig{ProductID: a.ID, Platform: "douyin_shop", CategoryPath: "Root / Leaf"}).Error; err != nil {
		t.Fatal(err)
	}

	c := listingExportTestCtx(1, nil)
	data, name, err := svc.ExportListingListCSV(c, []uuid.UUID{a.ID, b.ID})
	if err != nil {
		t.Fatal(err)
	}
	if name != "listing-list-2.csv" {
		t.Fatalf("unexpected name %s", name)
	}
	body := string(data)
	if !strings.HasPrefix(body, "\xEF\xBB\xBF") {
		t.Fatal("expected UTF-8 BOM")
	}
	for _, col := range []string{"标题", "副标题(AI标题)", "类目", "价格", "币种", "主图URL", "规格列表", "来源链接"} {
		if !strings.Contains(body, col) {
			t.Fatalf("missing header column %s: %s", col, body)
		}
	}
	if !strings.Contains(body, "Draft A") || !strings.Contains(body, "Draft B") {
		t.Fatalf("missing product rows: %s", body)
	}
	if !strings.Contains(body, "Root / Leaf") {
		t.Fatalf("missing category path: %s", body)
	}
	if !strings.Contains(body, "Red @12.50 x7") {
		t.Fatalf("missing spec list: %s", body)
	}
}

func TestExportListingListCSVCrossTenantDenied(t *testing.T) {
	db := openListingExportTestDB(t)
	svc := &Service{DB: db}
	a := createListingExportProduct(t, db, 1, "Draft T1")

	c := listingExportTestCtx(2, nil)
	_, _, err := svc.ExportListingListCSV(c, []uuid.UUID{a.ID})
	if err != gorm.ErrRecordNotFound {
		t.Fatalf("expected not found for cross-tenant export, got %v", err)
	}
}

func TestExportListingListCSVShopScopeDenied(t *testing.T) {
	db := openListingExportTestDB(t)
	svc := &Service{DB: db}
	a := createListingExportProduct(t, db, 1, "Draft Unassigned")

	// operator scoped to a shop that is not linked to the draft: same 404 as draft list scope.
	scoped := &adminperm.Principal{
		UserID: uuid.New(),
		Role:   adminperm.RoleOperator,
		StoreGrants: []adminperm.StoreGrant{
			{StoreID: uuid.New(), PermissionScope: "operate"},
		},
	}
	c := listingExportTestCtx(1, scoped)
	_, _, err := svc.ExportListingListCSV(c, []uuid.UUID{a.ID})
	if err != gorm.ErrRecordNotFound {
		t.Fatalf("expected not found for out-of-scope export, got %v", err)
	}
}

func TestExportListingListCSVShopScopeAllowed(t *testing.T) {
	db := openListingExportTestDB(t)
	svc := &Service{DB: db}
	a := createListingExportProduct(t, db, 1, "Draft Scoped")
	shopID := uuid.New()
	if err := db.Create(&ProductPlatformPublishConfig{ProductID: a.ID, Platform: "douyin_shop", ShopID: &shopID, CategoryPath: "Root / Leaf"}).Error; err != nil {
		t.Fatal(err)
	}

	scoped := &adminperm.Principal{
		UserID: uuid.New(),
		Role:   adminperm.RoleOperator,
		StoreGrants: []adminperm.StoreGrant{
			{StoreID: shopID, PermissionScope: "operate"},
		},
	}
	c := listingExportTestCtx(1, scoped)
	data, _, err := svc.ExportListingListCSV(c, []uuid.UUID{a.ID})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(data), "Draft Scoped") {
		t.Fatalf("expected scoped draft exported: %s", string(data))
	}
}

func newListingExportRouter(db *gorm.DB) *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	h := &Handler{Svc: &Service{DB: db}}
	r.GET("/api/v1/products/listing-list/export.csv", func(c *gin.Context) {
		c.Set(ctxkey.TenantID, int64(1))
		h.ExportListingListCSVHandler(c)
	})
	return r
}

func TestExportListingListCSVHandlerDedupesIDs(t *testing.T) {
	db := openListingExportTestDB(t)
	a := createListingExportProduct(t, db, 1, "Draft Dedupe")
	r := newListingExportRouter(db)

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/products/listing-list/export.csv?ids="+a.ID.String()+","+a.ID.String(), nil)
	r.ServeHTTP(w, req)
	if w.Code != 200 {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
	lines := strings.Split(strings.TrimRight(w.Body.String(), "\n"), "\n")
	if len(lines) != 2 {
		t.Fatalf("expected header + deduped single row, got %d lines: %s", len(lines), w.Body.String())
	}
	if cd := w.Header().Get("Content-Disposition"); !strings.Contains(cd, "listing-list-1.csv") {
		t.Fatalf("unexpected content disposition %q", cd)
	}
}

func TestExportListingListCSVHandlerLimits(t *testing.T) {
	db := openListingExportTestDB(t)
	r := newListingExportRouter(db)

	ids := make([]string, 0, MaxListingExportProducts+1)
	for i := 0; i < MaxListingExportProducts+1; i++ {
		ids = append(ids, uuid.New().String())
	}
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/products/listing-list/export.csv?ids="+strings.Join(ids, ","), nil)
	r.ServeHTTP(w, req)
	if w.Code != 400 {
		t.Fatalf("expected 400 for too many ids, got %d", w.Code)
	}

	w = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodGet, "/api/v1/products/listing-list/export.csv?ids=", nil)
	r.ServeHTTP(w, req)
	if w.Code != 400 {
		t.Fatalf("expected 400 for empty ids, got %d", w.Code)
	}

	w = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodGet, "/api/v1/products/listing-list/export.csv?ids=not-a-uuid", nil)
	r.ServeHTTP(w, req)
	if w.Code != 400 {
		t.Fatalf("expected 400 for invalid id, got %d", w.Code)
	}

	w = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodGet, "/api/v1/products/listing-list/export.csv?ids="+uuid.New().String(), nil)
	r.ServeHTTP(w, req)
	if w.Code != 404 {
		t.Fatalf("expected 404 for unknown id, got %d", w.Code)
	}
}
