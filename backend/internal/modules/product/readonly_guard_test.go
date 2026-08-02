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

// readonly admins must get 403 on every product write endpoint, not only
// POST /products; visibility scoping alone must not stand in for the guard.
func TestProductWriteRoutesRejectReadonly(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
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
	product.Register(r.Group("/api/v1"), &product.Handler{Svc: &product.Service{DB: db}})

	pid := uuid.New().String()
	sid := uuid.New().String()
	imgID := uuid.New().String()
	cases := []struct{ method, path string }{
		{http.MethodPost, "/api/v1/products"},
		{http.MethodPut, "/api/v1/products/" + pid},
		{http.MethodDelete, "/api/v1/products/" + pid},
		{http.MethodPut, "/api/v1/products/" + pid + "/platform-configs/douyin_shop"},
		{http.MethodPost, "/api/v1/products/" + pid + "/platform-configs/douyin_shop/build-mapping"},
		{http.MethodPut, "/api/v1/products/" + pid + "/platform-configs/douyin_shop/mapping"},
		{http.MethodPost, "/api/v1/products/" + pid + "/platform-configs/douyin_shop/images/upload"},
		{http.MethodPost, "/api/v1/products/" + pid + "/platform-configs/douyin_shop/images/main_0/retry"},
		{http.MethodPost, "/api/v1/products/" + pid + "/skus"},
		{http.MethodPut, "/api/v1/products/" + pid + "/skus/" + sid},
		{http.MethodPut, "/api/v1/products/" + pid + "/skus/" + sid + "/stock-settings"},
		{http.MethodDelete, "/api/v1/products/" + pid + "/skus/" + sid},
		{http.MethodPost, "/api/v1/products/" + pid + "/images"},
		{http.MethodPut, "/api/v1/products/" + pid + "/images/" + imgID},
		{http.MethodDelete, "/api/v1/products/" + pid + "/images/" + imgID},
		{http.MethodPost, "/api/v1/products/" + pid + "/images/reorder"},
		{http.MethodPost, "/api/v1/products/" + pid + "/sync-images"},
		{http.MethodPost, "/api/v1/products/" + pid + "/ai/optimize-title"},
		{http.MethodPost, "/api/v1/products/" + pid + "/ai/generate-description"},
		{http.MethodPost, "/api/v1/products/" + pid + "/apply-ai-title"},
		{http.MethodPost, "/api/v1/products/" + pid + "/apply-ai-description"},
		{http.MethodPost, "/api/v1/products/" + pid + "/undo-ai-title"},
		{http.MethodPost, "/api/v1/products/" + pid + "/undo-ai-description"},
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
	r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/api/v1/products", nil))
	if w.Code == http.StatusForbidden {
		t.Fatalf("GET /products must stay readable for readonly role")
	}
}
