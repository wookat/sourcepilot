package selection

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

func newGuardDB(t *testing.T) *gorm.DB {
	t.Helper()
	dsn := filepath.Join(t.TempDir(), "guard.db")
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Skipf("sqlite unavailable: %v", err)
	}
	if err := db.AutoMigrate(
		&admin.AdminUser{}, &admin.UserStorePermission{},
		&SelectionTask{}, &SelectionCandidate{}, &SelectionSourceMatch{}, &SelectionEvaluation{},
	); err != nil {
		t.Fatal(err)
	}
	return db
}

func seedAdmin(t *testing.T, db *gorm.DB, role string) uuid.UUID {
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

func newSelectionRouter(db *gorm.DB, uid uuid.UUID, tenantID int64) *gin.Engine {
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

// readonly admins must be rejected at the route level for every selection
// write endpoint while reads stay allowed.
func TestSelectionWriteRoutesRejectReadonly(t *testing.T) {
	db := newGuardDB(t)
	uid := seedAdmin(t, db, "readonly")
	r := newSelectionRouter(db, uid, 0)

	id := uuid.New().String()
	cases := []struct{ method, path string }{
		{http.MethodPost, "/api/v1/selection/tasks"},
		{http.MethodPost, "/api/v1/selection/tasks/" + id + "/retry"},
		{http.MethodPost, "/api/v1/selection/candidates/" + id + "/decision"},
		{http.MethodPost, "/api/v1/selection/candidates/" + id + "/to-draft"},
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
	r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/api/v1/selection/tasks", nil))
	if w.Code != http.StatusOK {
		t.Fatalf("GET /selection/tasks: got %d, want 200 for readonly role", w.Code)
	}
}

// operator (non-readonly) admins must pass the write guard; the request must
// reach the handler instead of being rejected with 403.
func TestSelectionWriteRoutesAllowOperator(t *testing.T) {
	db := newGuardDB(t)
	uid := seedAdmin(t, db, "operator")
	r := newSelectionRouter(db, uid, 0)

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/v1/selection/tasks/"+uuid.New().String()+"/retry", strings.NewReader("{}"))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)
	if w.Code == http.StatusForbidden {
		t.Fatalf("operator retry: got 403, want handler-level response")
	}
}
