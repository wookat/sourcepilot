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
	"github.com/trademind-ai/trademind/backend/internal/modules/operationlog"
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
	if err := db.AutoMigrate(&admin.AdminUser{}, &platformtenant.Tenant{}, &operationlog.OperationLog{}); err != nil {
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
	platformtenant.Register(r.Group("/api/v1"), &platformtenant.Handler{Svc: &platformtenant.Service{DB: db, OpLog: &operationlog.Service{DB: db}}})
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

func createTenantForTest(t *testing.T, r *gin.Engine, name string) int64 {
	t.Helper()
	pw := strings.Join([]string{"secret", "123"}, "")
	w := doJSON(r, http.MethodPost, "/api/v1/platform/tenants",
		fmt.Sprintf(`{"name":"%s","adminEmail":"%s@example.com","adminPassword":"%s"}`, name, name, pw))
	if w.Code != http.StatusOK {
		t.Fatalf("create %s: got %d body=%s", name, w.Code, w.Body.String())
	}
	var payload struct {
		Data struct {
			Tenant struct {
				ID int64 `json:"id"`
			} `json:"tenant"`
		} `json:"data"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &payload); err != nil {
		t.Fatal(err)
	}
	return payload.Data.Tenant.ID
}

// Platform admin can rename a tenant; duplicate names are rejected and the
// platform tenant (id 0) cannot be renamed.
func TestPlatformAdminRenameTenant(t *testing.T) {
	db := openTenantTestDB(t)
	actor := seedActor(t, db, 0, "admin")
	r := newTenantRouter(db, actor, 0)
	id := createTenantForTest(t, r, "e2e-rename-a")
	_ = createTenantForTest(t, r, "e2e-rename-b")

	w := doJSON(r, http.MethodPut, fmt.Sprintf("/api/v1/platform/tenants/%d", id), `{"name":"e2e-rename-a2"}`)
	if w.Code != http.StatusOK || !strings.Contains(w.Body.String(), "e2e-rename-a2") {
		t.Fatalf("rename: got %d body=%s", w.Code, w.Body.String())
	}
	var tRow platformtenant.Tenant
	if err := db.First(&tRow, "id = ?", id).Error; err != nil || tRow.Name != "e2e-rename-a2" {
		t.Fatalf("rename not persisted: err=%v name=%s", err, tRow.Name)
	}

	if w := doJSON(r, http.MethodPut, fmt.Sprintf("/api/v1/platform/tenants/%d", id), `{"name":"e2e-rename-b"}`); w.Code != http.StatusBadRequest {
		t.Fatalf("duplicate rename: got %d, want 400", w.Code)
	}
	if w := doJSON(r, http.MethodPut, "/api/v1/platform/tenants/0", `{"name":"x"}`); w.Code != http.StatusBadRequest {
		t.Fatalf("rename platform tenant: got %d, want 400", w.Code)
	}
	if w := doJSON(r, http.MethodPut, "/api/v1/platform/tenants/999999", `{"name":"x"}`); w.Code != http.StatusNotFound {
		t.Fatalf("rename missing tenant: got %d, want 404", w.Code)
	}

	var cnt int64
	if err := db.Model(&operationlog.OperationLog{}).
		Where("action = ? AND resource = ? AND resource_id = ?", "tenant.rename", "tenant", fmt.Sprintf("%d", id)).
		Count(&cnt).Error; err != nil || cnt != 1 {
		t.Fatalf("audit log for tenant.rename: err=%v cnt=%d, want 1", err, cnt)
	}
}

// Platform admin can disable and re-enable a tenant; the platform tenant
// (id 0) cannot be disabled.
func TestPlatformAdminDisableEnableTenant(t *testing.T) {
	db := openTenantTestDB(t)
	actor := seedActor(t, db, 0, "admin")
	r := newTenantRouter(db, actor, 0)
	id := createTenantForTest(t, r, "e2e-disable")

	w := doJSON(r, http.MethodPost, fmt.Sprintf("/api/v1/platform/tenants/%d/disable", id), "")
	if w.Code != http.StatusOK || !strings.Contains(w.Body.String(), `"status":"disabled"`) {
		t.Fatalf("disable: got %d body=%s", w.Code, w.Body.String())
	}
	var tRow platformtenant.Tenant
	if err := db.First(&tRow, "id = ?", id).Error; err != nil || tRow.Status != platformtenant.StatusDisabled {
		t.Fatalf("disable not persisted: err=%v status=%s", err, tRow.Status)
	}

	w = doJSON(r, http.MethodPost, fmt.Sprintf("/api/v1/platform/tenants/%d/enable", id), "")
	if w.Code != http.StatusOK || !strings.Contains(w.Body.String(), `"status":"active"`) {
		t.Fatalf("enable: got %d body=%s", w.Code, w.Body.String())
	}
	if err := db.First(&tRow, "id = ?", id).Error; err != nil || tRow.Status != platformtenant.StatusActive {
		t.Fatalf("enable not persisted: err=%v status=%s", err, tRow.Status)
	}

	if w := doJSON(r, http.MethodPost, "/api/v1/platform/tenants/0/disable", ""); w.Code != http.StatusBadRequest {
		t.Fatalf("disable platform tenant: got %d, want 400", w.Code)
	}
	if w := doJSON(r, http.MethodPost, "/api/v1/platform/tenants/999999/disable", ""); w.Code != http.StatusNotFound {
		t.Fatalf("disable missing tenant: got %d, want 404", w.Code)
	}

	for _, action := range []string{"tenant.disable", "tenant.enable"} {
		var cnt int64
		if err := db.Model(&operationlog.OperationLog{}).
			Where("action = ? AND resource = ? AND resource_id = ?", action, "tenant", fmt.Sprintf("%d", id)).
			Count(&cnt).Error; err != nil || cnt != 1 {
			t.Fatalf("audit log for %s: err=%v cnt=%d, want 1", action, err, cnt)
		}
	}
}

// Non-platform personas are rejected with 403 on all governance routes and
// leave no side effects.
func TestNonPlatformAdminGovernForbidden(t *testing.T) {
	db := openTenantTestDB(t)
	platformActor := seedActor(t, db, 0, "admin")
	pr := newTenantRouter(db, platformActor, 0)
	id := createTenantForTest(t, pr, "e2e-govern")

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
			if w := doJSON(r, http.MethodPut, fmt.Sprintf("/api/v1/platform/tenants/%d", id), `{"name":"e2e-hacked"}`); w.Code != http.StatusForbidden {
				t.Fatalf("rename: got %d, want 403", w.Code)
			}
			if w := doJSON(r, http.MethodPost, fmt.Sprintf("/api/v1/platform/tenants/%d/disable", id), ""); w.Code != http.StatusForbidden {
				t.Fatalf("disable: got %d, want 403", w.Code)
			}
			if w := doJSON(r, http.MethodPost, fmt.Sprintf("/api/v1/platform/tenants/%d/enable", id), ""); w.Code != http.StatusForbidden {
				t.Fatalf("enable: got %d, want 403", w.Code)
			}
			var tRow platformtenant.Tenant
			if err := db.First(&tRow, "id = ?", id).Error; err != nil {
				t.Fatal(err)
			}
			if tRow.Name != "e2e-govern" || tRow.Status != platformtenant.StatusActive {
				t.Fatalf("forbidden govern must have no side effect: name=%s status=%s", tRow.Name, tRow.Status)
			}
		})
	}
}
