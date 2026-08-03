package platformtenant_test

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"github.com/google/uuid"
	"github.com/trademind-ai/trademind/backend/internal/modules/admin"
	"github.com/trademind-ai/trademind/backend/internal/modules/platformtenant"
	"github.com/trademind-ai/trademind/backend/internal/pkg/ctxkey"
	"github.com/trademind-ai/trademind/backend/internal/pkg/model"
	"gorm.io/gorm"
)

func openTenantTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	dsn := fmt.Sprintf("file:platformtenant_%s?mode=memory&cache=shared&_pragma=busy_timeout(5000)", uuid.New().String())
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Skipf("sqlite unavailable: %v", err)
	}
	sqlDB, _ := db.DB()
	sqlDB.SetMaxOpenConns(1)
	if err := db.AutoMigrate(&admin.AdminUser{}, &platformtenant.Tenant{}); err != nil {
		t.Fatal(err)
	}
	return db
}

func seedActor(t *testing.T, db *gorm.DB, tenantID int64, role string) uuid.UUID {
	t.Helper()
	uid := uuid.New()
	if err := db.Create(&admin.AdminUser{
		Base:         model.Base{ID: uid},
		TenantID:     tenantID,
		Username:     "u-" + uid.String()[:12],
		Email:        "u-" + uid.String()[:12] + "@example.com",
		PasswordHash: "x",
		Role:         role,
		Status:       "active",
	}).Error; err != nil {
		t.Fatalf("seed tenant=%d %s user: %v", tenantID, role, err)
	}
	return uid
}

func newTenantRouter(db *gorm.DB, actorID uuid.UUID, tenantID int64) *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(func(c *gin.Context) {
		c.Set(ctxkey.AdminID, actorID.String())
		c.Set(ctxkey.TenantID, tenantID)
		c.Next()
	})
	platformtenant.Register(r.Group("/api/v1"), &platformtenant.Handler{Svc: &platformtenant.Service{DB: db}})
	return r
}

func doJSON(r *gin.Engine, method, path, body string) *httptest.ResponseRecorder {
	w := httptest.NewRecorder()
	var req *http.Request
	if body == "" {
		req = httptest.NewRequest(method, path, nil)
	} else {
		req = httptest.NewRequest(method, path, strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
	}
	r.ServeHTTP(w, req)
	return w
}

// Platform admin (tenant 0 admin) can list and create tenants; the created
// initial admin lands in the new tenant with role admin.
func TestPlatformAdminCanCreateTenant(t *testing.T) {
	db := openTenantTestDB(t)
	actor := seedActor(t, db, 0, "admin")
	r := newTenantRouter(db, actor, 0)

	w := doJSON(r, http.MethodPost, "/api/v1/platform/tenants",
		`{"name":"e2e-tenant-b","adminEmail":"e2e-tenant-b-admin@example.com","adminPassword":"secret123"}`)
	if w.Code != http.StatusOK {
		t.Fatalf("create: got %d body=%s, want 200", w.Code, w.Body.String())
	}
	var payload struct {
		Data struct {
			Tenant struct {
				ID   int64  `json:"id"`
				Name string `json:"name"`
			} `json:"tenant"`
			AdminID string `json:"adminId"`
		} `json:"data"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &payload); err != nil {
		t.Fatal(err)
	}
	if payload.Data.Tenant.ID == 0 || payload.Data.Tenant.Name != "e2e-tenant-b" {
		t.Fatalf("unexpected tenant payload: %+v", payload.Data.Tenant)
	}
	var u admin.AdminUser
	if err := db.First(&u, "id = ?", payload.Data.AdminID).Error; err != nil {
		t.Fatalf("created admin not found: %v", err)
	}
	if u.TenantID != payload.Data.Tenant.ID || u.Role != "admin" || u.Status != "active" {
		t.Fatalf("created admin tenant=%d role=%s status=%s, want tenant=%d admin active",
			u.TenantID, u.Role, u.Status, payload.Data.Tenant.ID)
	}

	w = doJSON(r, http.MethodGet, "/api/v1/platform/tenants", "")
	if w.Code != http.StatusOK {
		t.Fatalf("list: got %d body=%s, want 200", w.Code, w.Body.String())
	}
	if !strings.Contains(w.Body.String(), "e2e-tenant-b") {
		t.Fatalf("list should contain created tenant, body=%s", w.Body.String())
	}
}

// Non-platform personas are all rejected with 403 (tenant != 0, or non-admin role).
func TestNonPlatformAdminForbidden(t *testing.T) {
	db := openTenantTestDB(t)
	cases := []struct {
		name   string
		tenant int64
		role   string
	}{
		{"tenant1-admin", 1, "admin"},
		{"tenant0-operator", 0, "operator"},
		{"tenant0-readonly", 0, "readonly"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			actor := seedActor(t, db, tc.tenant, tc.role)
			r := newTenantRouter(db, actor, tc.tenant)
			if w := doJSON(r, http.MethodGet, "/api/v1/platform/tenants", ""); w.Code != http.StatusForbidden {
				t.Fatalf("list: got %d, want 403", w.Code)
			}
			w := doJSON(r, http.MethodPost, "/api/v1/platform/tenants",
				`{"name":"e2e-x","adminEmail":"e2e-x@example.com","adminPassword":"secret123"}`)
			if w.Code != http.StatusForbidden {
				t.Fatalf("create: got %d, want 403", w.Code)
			}
			var cnt int64
			if err := db.Model(&platformtenant.Tenant{}).Count(&cnt).Error; err != nil {
				t.Fatal(err)
			}
			if cnt != 0 {
				t.Fatalf("forbidden create must have no side effect, tenants=%d", cnt)
			}
		})
	}
}

// Duplicate tenant name or admin email is rejected with 400.
func TestCreateTenantDuplicates(t *testing.T) {
	db := openTenantTestDB(t)
	actor := seedActor(t, db, 0, "admin")
	r := newTenantRouter(db, actor, 0)

	body := `{"name":"e2e-dup","adminEmail":"e2e-dup@example.com","adminPassword":"secret123"}`
	if w := doJSON(r, http.MethodPost, "/api/v1/platform/tenants", body); w.Code != http.StatusOK {
		t.Fatalf("first create: got %d body=%s", w.Code, w.Body.String())
	}
	if w := doJSON(r, http.MethodPost, "/api/v1/platform/tenants", body); w.Code != http.StatusBadRequest {
		t.Fatalf("duplicate name: got %d, want 400", w.Code)
	}
	w := doJSON(r, http.MethodPost, "/api/v1/platform/tenants",
		`{"name":"e2e-dup-2","adminEmail":"e2e-dup@example.com","adminPassword":"secret123"}`)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("duplicate email: got %d, want 400", w.Code)
	}
}
