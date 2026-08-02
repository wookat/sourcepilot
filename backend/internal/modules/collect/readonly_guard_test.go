package collect

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

func newCollectGuardDB(t *testing.T) *gorm.DB {
	t.Helper()
	dsn := filepath.Join(t.TempDir(), "guard.db")
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Skipf("sqlite unavailable: %v", err)
	}
	if err := db.AutoMigrate(
		&admin.AdminUser{}, &admin.UserStorePermission{},
		&CollectTask{}, &CollectBatch{}, &CollectTaskEvent{},
	); err != nil {
		t.Fatal(err)
	}
	return db
}

func seedCollectAdmin(t *testing.T, db *gorm.DB, role string) uuid.UUID {
	t.Helper()
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
	return uid
}

func newCollectGuardRouter(db *gorm.DB, uid uuid.UUID, tenantID int64) *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(func(c *gin.Context) {
		c.Set(ctxkey.AdminID, uid.String())
		c.Set(ctxkey.TenantID, tenantID)
		c.Next()
	})
	Register(r.Group("/api/v1"), &Handler{Svc: &Service{DB: db}})
	return r
}

func collectWriteRoutes(id string) []struct{ method, path string } {
	return []struct{ method, path string }{
		{http.MethodPost, "/api/v1/collect/tasks"},
		{http.MethodPost, "/api/v1/collect/tasks/" + id + "/retry"},
		{http.MethodPost, "/api/v1/collect/batches"},
		{http.MethodPost, "/api/v1/collect/batches/" + id + "/retry-failed"},
		{http.MethodPost, "/api/v1/collector/providers/1688/open-login-browser"},
		{http.MethodPost, "/api/v1/collector/providers/pinduoduo/open-login-browser"},
		{http.MethodPost, "/api/v1/collector/providers/taobao_tmall/open-login-browser"},
		{http.MethodPost, "/api/v1/collect/providers/taobao_tmall/open-login-browser"},
	}
}

// readonly admins must be rejected at the route level for every collect
// write endpoint while reads stay allowed.
func TestCollectWriteRoutesRejectReadonly(t *testing.T) {
	db := newCollectGuardDB(t)
	uid := seedCollectAdmin(t, db, "readonly")
	r := newCollectGuardRouter(db, uid, 0)

	for _, tc := range collectWriteRoutes(uuid.New().String()) {
		w := httptest.NewRecorder()
		req := httptest.NewRequest(tc.method, tc.path, strings.NewReader("{}"))
		req.Header.Set("Content-Type", "application/json")
		r.ServeHTTP(w, req)
		if w.Code != http.StatusForbidden {
			t.Fatalf("%s %s: got %d, want 403", tc.method, tc.path, w.Code)
		}
	}

	for _, path := range []string{
		"/api/v1/collect/tasks",
		"/api/v1/collect/batches",
	} {
		w := httptest.NewRecorder()
		r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, path, nil))
		if w.Code == http.StatusForbidden {
			t.Fatalf("GET %s must stay readable for readonly role", path)
		}
	}
}

// admin and operator must not be blocked by the readonly guard (they may
// still fail later, e.g. queue disabled, but never with route-level 403).
func TestCollectWriteRoutesAllowWritableRoles(t *testing.T) {
	for _, role := range []string{"admin", "operator"} {
		t.Run(role, func(t *testing.T) {
			db := newCollectGuardDB(t)
			uid := seedCollectAdmin(t, db, role)
			r := newCollectGuardRouter(db, uid, 0)

			for _, tc := range collectWriteRoutes(uuid.New().String()) {
				w := httptest.NewRecorder()
				req := httptest.NewRequest(tc.method, tc.path, strings.NewReader("{}"))
				req.Header.Set("Content-Type", "application/json")
				r.ServeHTTP(w, req)
				if w.Code == http.StatusForbidden {
					t.Fatalf("%s %s: %s got 403, guard must only block readonly", tc.method, tc.path, role)
				}
			}
		})
	}
}
