package product

import (
	"errors"
	"net/http/httptest"
	"path/filepath"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"gorm.io/gorm"

	"github.com/trademind-ai/trademind/backend/internal/pkg/ctxkey"
)

// Regression (R70): AI task listing under a product must 404 for
// cross-tenant parents instead of leaking task data by raw product id.
func TestListRecentAITasksScopedByTenant(t *testing.T) {
	dsn := filepath.Join(t.TempDir(), "scope.db")
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Skipf("sqlite unavailable: %v", err)
	}
	if err := db.AutoMigrate(&Product{}); err != nil {
		t.Fatal(err)
	}
	p := &Product{TenantID: 1, Title: "p1", Status: "draft"}
	if err := db.Create(p).Error; err != nil {
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

	if items, err := svc.ListRecentAITasks(ginCtx(1), p.ID, 15); err != nil || items == nil {
		t.Fatalf("same-tenant: got %v %v", err, items)
	}
	if _, err := svc.ListRecentAITasks(ginCtx(2), p.ID, 15); !errors.Is(err, gorm.ErrRecordNotFound) {
		t.Fatalf("cross-tenant: want ErrRecordNotFound, got %v", err)
	}
}
