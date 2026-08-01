package order_test

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"github.com/trademind-ai/trademind/backend/internal/modules/admin"
	"github.com/trademind-ai/trademind/backend/internal/modules/order"
	"github.com/trademind-ai/trademind/backend/internal/pkg/ctxkey"
	"github.com/trademind-ai/trademind/backend/internal/pkg/model"
)

// readonly admins must be rejected at the route level for every order write
// endpoint, not only for the few handlers that call denyWrite internally.
func TestOrderWriteRoutesRejectReadonly(t *testing.T) {
	db := openImportTestDB(t)
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
	order.Register(r.Group("/api/v1"), &order.Handler{Svc: &order.Service{DB: db}})

	oid := uuid.New().String()
	cases := []struct{ method, path string }{
		{http.MethodPost, "/api/v1/orders"},
		{http.MethodPut, "/api/v1/orders/" + oid},
		{http.MethodDelete, "/api/v1/orders/" + oid},
		{http.MethodPost, "/api/v1/orders/import"},
		{http.MethodPost, "/api/v1/orders/" + oid + "/items"},
		{http.MethodPost, "/api/v1/orders/" + oid + "/shipments"},
		{http.MethodPost, "/api/v1/orders/shipments/batch"},
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
	r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/api/v1/orders", nil))
	if w.Code == http.StatusForbidden {
		t.Fatalf("GET /orders must stay readable for readonly role")
	}
}
