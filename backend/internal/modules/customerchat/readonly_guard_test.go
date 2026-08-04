package customerchat

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

// readonly admins must be rejected at the route level for every customer
// chat write endpoint while reads stay allowed.
func TestCustomerChatWriteRoutesRejectReadonly(t *testing.T) {
	dsn := filepath.Join(t.TempDir(), "guard.db")
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Skipf("sqlite unavailable: %v", err)
	}
	if err := db.AutoMigrate(&admin.AdminUser{}, &admin.UserStorePermission{}, &CustomerConversation{}, &CustomerMessage{}); err != nil {
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
		{http.MethodPost, "/api/v1/customer/conversations"},
		{http.MethodPut, "/api/v1/customer/conversations/" + id},
		{http.MethodDelete, "/api/v1/customer/conversations/" + id},
		{http.MethodPost, "/api/v1/customer/conversations/" + id + "/messages"},
		{http.MethodPost, "/api/v1/customer/conversations/" + id + "/mark-replied"},
		{http.MethodPost, "/api/v1/customer/conversations/" + id + "/ai/generate-reply"},
		{http.MethodPost, "/api/v1/customer/conversations/" + id + "/ai-suggestions"},
		{http.MethodPost, "/api/v1/customer/conversations/" + id + "/send-platform-message"},
		{http.MethodPost, "/api/v1/customer/reply-templates"},
		{http.MethodPost, "/api/v1/customer/reply-templates/reorder"},
		{http.MethodPut, "/api/v1/customer/reply-templates/" + id},
		{http.MethodDelete, "/api/v1/customer/reply-templates/" + id},
		{http.MethodPut, "/api/v1/customer/reply-suggestions/" + id},
		{http.MethodPost, "/api/v1/customer/reply-suggestions/" + id + "/accept"},
		{http.MethodPost, "/api/v1/customer/reply-suggestions/" + id + "/discard"},
		{http.MethodPost, "/api/v1/customer/ai-suggestions/" + id + "/apply"},
		{http.MethodPost, "/api/v1/customer/ai-suggestions/" + id + "/reject"},
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
	r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/api/v1/customer/conversations", nil))
	if w.Code == http.StatusForbidden {
		t.Fatalf("GET /customer/conversations must stay readable for readonly role")
	}
}
