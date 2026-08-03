package sourcing

import (
	"errors"
	"net/http/httptest"
	"path/filepath"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"github.com/google/uuid"
	"gorm.io/gorm"

	"github.com/trademind-ai/trademind/backend/internal/modules/product"
	"github.com/trademind-ai/trademind/backend/internal/pkg/ctxkey"
)

// Regression (R71): product sources / price-history must verify the parent
// product's tenant scope and return 404 for cross-tenant access instead of
// leaking business data through bare IDs.
func TestSourcingSubresourcesScopedByTenant(t *testing.T) {
	dsn := filepath.Join(t.TempDir(), "scope.db")
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Skipf("sqlite unavailable: %v", err)
	}
	if err := db.AutoMigrate(&product.Product{}, &Supplier{}, &ProductSource{}, &ProductSourceSKU{}, &SourcePriceHistory{}); err != nil {
		t.Fatal(err)
	}

	prod := &product.Product{TenantID: 1, Source: "manual", Title: "p1", Status: "draft"}
	if err := db.Create(prod).Error; err != nil {
		t.Fatal(err)
	}
	sup := &Supplier{Name: "s1", Platform: "1688", Status: SupplierStatusActive}
	if err := db.Create(sup).Error; err != nil {
		t.Fatal(err)
	}
	src := &ProductSource{ProductID: prod.ID, SupplierID: sup.ID, Status: SourceStatusActive}
	if err := db.Create(src).Error; err != nil {
		t.Fatal(err)
	}
	sku := &ProductSourceSKU{ProductSourceID: src.ID, LocalSKUID: uuid.New(), Status: "active"}
	if err := db.Create(sku).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Create(&SourcePriceHistory{SourceSKUID: sku.ID, Price: 9.9, CapturedAt: time.Now().UTC(), CaptureSource: "manual"}).Error; err != nil {
		t.Fatal(err)
	}

	gin.SetMode(gin.TestMode)
	svc := &Service{DB: db}
	ginCtx := func(tid int64, adminID string) *gin.Context {
		c, _ := gin.CreateTestContext(httptest.NewRecorder())
		c.Request = httptest.NewRequest("GET", "/", nil)
		c.Set(ctxkey.TenantID, tid)
		if adminID != "" {
			c.Set(ctxkey.AdminID, adminID)
		}
		return c
	}

	// 角色1：同租户 admin —— 正常读取
	if items, err := svc.ListProductSources(ginCtx(1, ""), prod.ID); err != nil || len(items) != 1 {
		t.Fatalf("same-tenant list sources: items=%d err=%v", len(items), err)
	}
	if items, err := svc.PriceHistory(ginCtx(1, ""), sku.ID, 90); err != nil || len(items) != 1 {
		t.Fatalf("same-tenant price history: items=%d err=%v", len(items), err)
	}
	// 角色2：同租户 operator ——（商品无店铺归属，租户内可读）
	if _, err := svc.ListProductSources(ginCtx(1, uuid.New().String()), prod.ID); err != nil {
		t.Fatalf("same-tenant operator list sources: %v", err)
	}
	// 角色3：跨租户 —— 一律 ErrNotFound(404)，不泄露存在性
	if _, err := svc.ListProductSources(ginCtx(2, ""), prod.ID); !errors.Is(err, ErrNotFound) {
		t.Fatalf("cross-tenant list sources: want ErrNotFound, got %v", err)
	}
	if _, err := svc.PriceHistory(ginCtx(2, ""), sku.ID, 90); !errors.Is(err, ErrNotFound) {
		t.Fatalf("cross-tenant price history: want ErrNotFound, got %v", err)
	}
	// 不存在的父资源同样 404
	if _, err := svc.PriceHistory(ginCtx(1, ""), uuid.New(), 90); !errors.Is(err, ErrNotFound) {
		t.Fatalf("unknown sku price history: want ErrNotFound, got %v", err)
	}
}

// Regression (R73): supplier / product-source / switch-suggestion write paths
// were tenant-unscoped, so a foreign tenant could edit, re-prioritize and
// delete another tenant's sourcing data through bare IDs. Every one of them
// must now answer ErrNotFound (404) and leave the data untouched.
func TestSourcingWritesScopedByTenant(t *testing.T) {
	dsn := filepath.Join(t.TempDir(), "writes.db")
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Skipf("sqlite unavailable: %v", err)
	}
	if err := db.AutoMigrate(&product.Product{}, &Supplier{}, &ProductSource{},
		&ProductSourceSKU{}, &SourcePriceHistory{}, &SourceSwitchEvent{}); err != nil {
		t.Fatal(err)
	}

	prod := &product.Product{TenantID: 1, Source: "manual", Title: "p1", Status: "draft"}
	if err := db.Create(prod).Error; err != nil {
		t.Fatal(err)
	}
	sup := &Supplier{TenantID: 1, Name: "s1", Platform: "1688", Status: SupplierStatusActive}
	if err := db.Create(sup).Error; err != nil {
		t.Fatal(err)
	}
	src := &ProductSource{TenantID: 1, ProductID: prod.ID, SupplierID: sup.ID, Priority: 100, IsPrimary: true, Status: SourceStatusActive}
	if err := db.Create(src).Error; err != nil {
		t.Fatal(err)
	}
	sku := &ProductSourceSKU{TenantID: 1, ProductSourceID: src.ID, LocalSKUID: uuid.New(), Status: "active"}
	if err := db.Create(sku).Error; err != nil {
		t.Fatal(err)
	}
	ev := &SourceSwitchEvent{TenantID: 1, ProductID: prod.ID, ToSourceID: src.ID, Reason: SwitchReasonManual, Mode: SwitchModeSuggested, Status: SuggestionOpen}
	if err := db.Create(ev).Error; err != nil {
		t.Fatal(err)
	}

	gin.SetMode(gin.TestMode)
	svc := &Service{DB: db}
	foreign := func() *gin.Context {
		c, _ := gin.CreateTestContext(httptest.NewRecorder())
		c.Request = httptest.NewRequest("POST", "/", nil)
		c.Set(ctxkey.TenantID, int64(2))
		return c
	}

	newName := "HACKED"
	if _, err := svc.UpdateSupplier(foreign(), sup.ID, SupplierBody{Name: newName}, nil); !errors.Is(err, ErrNotFound) {
		t.Fatalf("cross-tenant update supplier: want ErrNotFound, got %v", err)
	}
	if err := svc.DeleteSupplier(foreign(), sup.ID, nil); !errors.Is(err, ErrNotFound) {
		t.Fatalf("cross-tenant delete supplier: want ErrNotFound, got %v", err)
	}
	prio := 1
	if _, err := svc.UpdateSource(foreign(), src.ID, UpdateSourceBody{Priority: &prio}, nil); !errors.Is(err, ErrNotFound) {
		t.Fatalf("cross-tenant update source: want ErrNotFound, got %v", err)
	}
	if _, err := svc.SetPrimary(foreign(), src.ID, nil); !errors.Is(err, ErrNotFound) {
		t.Fatalf("cross-tenant set-primary: want ErrNotFound, got %v", err)
	}
	if _, err := svc.SaveSKUMappings(foreign(), src.ID, []SKUMappingBody{{LocalSKUID: uuid.New().String()}}, nil); !errors.Is(err, ErrNotFound) {
		t.Fatalf("cross-tenant save sku mappings: want ErrNotFound, got %v", err)
	}
	if err := svc.DeleteSKUMapping(foreign(), sku.ID, nil); !errors.Is(err, ErrNotFound) {
		t.Fatalf("cross-tenant delete sku mapping: want ErrNotFound, got %v", err)
	}
	if err := svc.DeleteSource(foreign(), src.ID, nil); !errors.Is(err, ErrNotFound) {
		t.Fatalf("cross-tenant delete source: want ErrNotFound, got %v", err)
	}
	if _, err := svc.BindSource(foreign(), prod.ID, BindSourceBody{SupplierName: "x", SourceURL: "https://example.com/o"}, nil); !errors.Is(err, ErrNotFound) {
		t.Fatalf("cross-tenant bind source: want ErrNotFound, got %v", err)
	}
	if _, err := svc.RefreshProductSources(foreign(), prod.ID, nil); !errors.Is(err, ErrNotFound) {
		t.Fatalf("cross-tenant refresh sources: want ErrNotFound, got %v", err)
	}
	if _, err := svc.AdoptSwitchSuggestion(foreign(), ev.ID, nil); !errors.Is(err, ErrNotFound) {
		t.Fatalf("cross-tenant adopt suggestion: want ErrNotFound, got %v", err)
	}
	if err := svc.IgnoreSwitchSuggestion(foreign(), ev.ID, nil); !errors.Is(err, ErrNotFound) {
		t.Fatalf("cross-tenant ignore suggestion: want ErrNotFound, got %v", err)
	}

	// foreign tenant lists must not see the owner's rows at all
	if out, err := svc.ListSuppliers(foreign(), SupplierListQuery{}); err != nil || out.Total != 0 {
		t.Fatalf("cross-tenant supplier list: total=%d err=%v", out.Total, err)
	}
	if rows, err := svc.ListSourceAlerts(foreign()); err != nil || len(rows) != 0 {
		t.Fatalf("cross-tenant alerts: rows=%d err=%v", len(rows), err)
	}
	if rows, err := svc.ListOrphanSources(foreign()); err != nil || len(rows) != 0 {
		t.Fatalf("cross-tenant orphans: rows=%d err=%v", len(rows), err)
	}
	if out, err := svc.ListSwitchEvents(foreign(), nil, 1, 20); err != nil || out["total"].(int64) != 0 {
		t.Fatalf("cross-tenant switch events: out=%v err=%v", out, err)
	}

	// owner data untouched
	var after Supplier
	if err := db.First(&after, "id = ?", sup.ID).Error; err != nil {
		t.Fatal(err)
	}
	if after.Name != "s1" {
		t.Fatalf("supplier mutated cross-tenant: %s", after.Name)
	}
	var afterSrc ProductSource
	if err := db.First(&afterSrc, "id = ?", src.ID).Error; err != nil {
		t.Fatal(err)
	}
	if afterSrc.Priority != 100 {
		t.Fatalf("source mutated cross-tenant: priority=%d", afterSrc.Priority)
	}
	var skuCount int64
	if err := db.Model(&ProductSourceSKU{}).Where("id = ?", sku.ID).Count(&skuCount).Error; err != nil {
		t.Fatal(err)
	}
	if skuCount != 1 {
		t.Fatalf("sku mapping deleted cross-tenant")
	}
}
