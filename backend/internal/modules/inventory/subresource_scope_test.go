package inventory

import (
	"errors"
	"net/http/httptest"
	"path/filepath"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"github.com/google/uuid"
	"gorm.io/gorm"

	"github.com/trademind-ai/trademind/backend/internal/modules/product"
	"github.com/trademind-ai/trademind/backend/internal/pkg/ctxkey"
)

// Regression (R70): product subresources (inventory logs, publication SKU
// rows) must 404 for cross-tenant parents instead of returning 200 with an
// empty list.
func TestProductSubresourcesScopedByTenant(t *testing.T) {
	dsn := filepath.Join(t.TempDir(), "scope.db")
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Skipf("sqlite unavailable: %v", err)
	}
	if err := db.AutoMigrate(&product.Product{}, &InventoryChangeLog{}); err != nil {
		t.Fatal(err)
	}
	p := &product.Product{TenantID: 1, Title: "p1", Status: "draft"}
	if err := db.Create(p).Error; err != nil {
		t.Fatal(err)
	}
	skuID := uuid.New()
	if err := db.Create(&InventoryChangeLog{TenantID: 1, ProductID: p.ID, ProductSKUID: skuID, ChangeType: "manual_adjust", BusinessEventKey: uuid.NewString()}).Error; err != nil {
		t.Fatal(err)
	}

	gin.SetMode(gin.TestMode)
	svc := &Service{DB: db}
	ginCtx := func(tid int64) *gin.Context {
		c, _ := gin.CreateTestContext(httptest.NewRecorder())
		c.Request = httptest.NewRequest("GET", "/", nil)
		c.Set(ctxkey.TenantID, tid)
		return c
	}

	if res, err := svc.ListSKUChangeLogs(ginCtx(1), p.ID, skuID, 1, 20); err != nil || res.Total != 1 {
		t.Fatalf("same-tenant logs: got %v %+v", err, res)
	}
	if _, err := svc.ListSKUChangeLogs(ginCtx(2), p.ID, skuID, 1, 20); !errors.Is(err, gorm.ErrRecordNotFound) {
		t.Fatalf("cross-tenant logs: want ErrRecordNotFound, got %v", err)
	}
	if _, err := svc.ListPublicationSkus(ginCtx(2), p.ID, nil); !errors.Is(err, gorm.ErrRecordNotFound) {
		t.Fatalf("cross-tenant publication skus: want ErrRecordNotFound, got %v", err)
	}
}
