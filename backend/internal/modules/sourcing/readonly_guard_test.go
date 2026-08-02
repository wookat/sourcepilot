package sourcing

import (
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"github.com/google/uuid"
	"gorm.io/gorm"

	"github.com/trademind-ai/trademind/backend/internal/modules/admin"
	"github.com/trademind-ai/trademind/backend/internal/pkg/ctxkey"
	"github.com/trademind-ai/trademind/backend/internal/pkg/model"
)

// readonly admins must be rejected at the route level for every sourcing
// write endpoint while reads stay allowed.
func TestSourcingWriteRoutesRejectReadonly(t *testing.T) {
	dsn := filepath.Join(t.TempDir(), "guard.db")
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Skipf("sqlite unavailable: %v", err)
	}
	if err := db.AutoMigrate(&admin.AdminUser{}, &admin.UserStorePermission{}); err != nil {
		t.Fatal(err)
	}
	uid := uuid.New()
	if err := db.Create(&admin.AdminUser{
		Base:         model.Base{ID: uid},
		Username:     "ro-" + uid.String()[:8],
		Email:        "ro-" + uid.String()[:8] + "@example.com",
		PasswordHash: "x",
		Role:         "readonly",
		Status:       "active",
	}).Error; err != nil {
		t.Fatalf("seed readonly user: %v", err)
	}

	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(func(c *gin.Context) {
		c.Set(ctxkey.AdminID, uid.String())
		c.Set(ctxkey.TenantID, int64(0))
		c.Next()
	})
	Register(r.Group("/api/v1"), &Handler{Svc: &Service{DB: db}})

	id := uuid.New().String()
	cases := []struct{ method, path string }{
		{http.MethodPost, "/api/v1/suppliers"},
		{http.MethodPut, "/api/v1/suppliers/" + id},
		{http.MethodDelete, "/api/v1/suppliers/" + id},
		{http.MethodPost, "/api/v1/products/" + id + "/sources"},
		{http.MethodPost, "/api/v1/products/" + id + "/sources/refresh"},
		{http.MethodPut, "/api/v1/product-sources/" + id},
		{http.MethodDelete, "/api/v1/product-sources/" + id},
		{http.MethodPost, "/api/v1/product-sources/" + id + "/set-primary"},
		{http.MethodPost, "/api/v1/product-sources/" + id + "/sku-mappings"},
		{http.MethodDelete, "/api/v1/product-source-skus/" + id},
		{http.MethodPost, "/api/v1/source-switch-events/" + id + "/adopt"},
		{http.MethodPost, "/api/v1/source-switch-events/" + id + "/ignore"},
	}
	for _, tc := range cases {
		w := httptest.NewRecorder()
		req := httptest.NewRequest(tc.method, tc.path, strings.NewReader("{}"))
		req.Header.Set("Content-Type", "application/json")
		r.ServeHTTP(w, req)
		if w.Code != http.StatusForbidden {
			t.Fatalf("%s %s: got %d, want 403", tc.method, tc.path, w.Code)
		}
	}

	// reads stay allowed
	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/api/v1/suppliers", nil))
	if w.Code == http.StatusForbidden {
		t.Fatalf("GET /suppliers must stay readable for readonly role")
	}
}
