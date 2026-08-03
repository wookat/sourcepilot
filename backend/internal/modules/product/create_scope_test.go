package product_test

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"github.com/google/uuid"
	"gorm.io/gorm"

	"github.com/trademind-ai/trademind/backend/internal/modules/admin"
	"github.com/trademind-ai/trademind/backend/internal/modules/product"
	"github.com/trademind-ai/trademind/backend/internal/pkg/ctxkey"
	"github.com/trademind-ai/trademind/backend/internal/pkg/model"
)

func newCreateScopeRouter(t *testing.T) (*gorm.DB, func(role string) *gin.Engine) {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Skipf("sqlite unavailable: %v", err)
	}
	if err := db.AutoMigrate(&admin.AdminUser{}, &admin.UserStorePermission{}, &product.Product{}, &product.ProductImage{}, &product.ProductSKU{}); err != nil {
		t.Fatal(err)
	}
	gin.SetMode(gin.TestMode)
	return db, func(role string) *gin.Engine {
		uid := uuid.New()
		if err := db.Create(&admin.AdminUser{
			Base:         model.Base{ID: uid},
			Username:     role + "-" + uid.String()[:8],
			Email:        role + "-" + uid.String()[:8] + "@example.com",
			PasswordHash: "x",
			Role:         role,
			Status:       "active",
		}).Error; err != nil {
			t.Fatalf("seed %s user: %v", role, err)
		}
		r := gin.New()
		r.Use(func(c *gin.Context) {
			c.Set(ctxkey.AdminID, uid.String())
			c.Set(ctxkey.TenantID, int64(0))
			c.Next()
		})
		product.Register(r.Group("/api/v1"), &product.Handler{Svc: &product.Service{DB: db}})
		return r
	}
}

func postDraft(r *gin.Engine) *httptest.ResponseRecorder {
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/v1/products", strings.NewReader(`{"title":"QA-Create-Scope"}`))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)
	return w
}

func countProducts(t *testing.T, db *gorm.DB) int64 {
	t.Helper()
	var n int64
	if err := db.Model(&product.Product{}).Count(&n).Error; err != nil {
		t.Fatal(err)
	}
	return n
}

// A store-scoped operator can never see an unassigned draft, so creation must
// be rejected up front (403) and must not leave an orphan row behind.
func TestCreateDraftOperatorRejectedWithoutDirtyRow(t *testing.T) {
	db, mkRouter := newCreateScopeRouter(t)

	w := postDraft(mkRouter("operator"))
	if w.Code != http.StatusForbidden {
		t.Fatalf("operator create: got %d, want 403 (body=%s)", w.Code, w.Body.String())
	}
	if n := countProducts(t, db); n != 0 {
		t.Fatalf("operator create left %d dirty product row(s), want 0", n)
	}
}

// Readonly write-guard path (403) must not persist a row either.
func TestCreateDraftReadonlyRejectedWithoutDirtyRow(t *testing.T) {
	db, mkRouter := newCreateScopeRouter(t)

	w := postDraft(mkRouter("readonly"))
	if w.Code != http.StatusForbidden {
		t.Fatalf("readonly create: got %d, want 403 (body=%s)", w.Code, w.Body.String())
	}
	if n := countProducts(t, db); n != 0 {
		t.Fatalf("readonly create left %d dirty product row(s), want 0", n)
	}
}

func TestCreateDraftAdminStillAllowed(t *testing.T) {
	db, mkRouter := newCreateScopeRouter(t)

	w := postDraft(mkRouter("admin"))
	if w.Code != http.StatusOK {
		t.Fatalf("admin create: got %d, want 200 (body=%s)", w.Code, w.Body.String())
	}
	if n := countProducts(t, db); n != 1 {
		t.Fatalf("admin create persisted %d row(s), want 1", n)
	}
}
