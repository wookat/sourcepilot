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
