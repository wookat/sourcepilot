package adminuser_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/trademind-ai/trademind/backend/internal/modules/admin"
	"github.com/trademind-ai/trademind/backend/internal/modules/adminuser"
	"github.com/trademind-ai/trademind/backend/internal/pkg/ctxkey"
	"github.com/trademind-ai/trademind/backend/internal/pkg/model"
	"gorm.io/gorm"
)

func seedTenantUser(t *testing.T, db *gorm.DB, tenantID int64, role string) uuid.UUID {
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
	adminuser.Register(r.Group("/api/v1"), &adminuser.Handler{Svc: &adminuser.Service{DB: db}})
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

// tenant-1 admin lists only tenant-1 users; tenant-2 users are invisible.
func TestListUsersTenantIsolation(t *testing.T) {
	db := openUserTestDB(t)
	actor := seedTenantUser(t, db, 1, "admin")
	same := seedTenantUser(t, db, 1, "operator")
	other := seedTenantUser(t, db, 2, "operator")

	r := newTenantRouter(db, actor, 1)
	w := doJSON(r, http.MethodGet, "/api/v1/admin/users?pageSize=100", "")
	if w.Code != http.StatusOK {
		t.Fatalf("list: got %d body=%s, want 200", w.Code, w.Body.String())
	}
	body := w.Body.String()
	var payload struct {
		Data struct {
			Items []struct {
				ID string `json:"id"`
			} `json:"list"`
		} `json:"data"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &payload); err != nil {
		t.Fatalf("decode list: %v body=%s", err, body)
	}
	seen := map[string]bool{}
	for _, it := range payload.Data.Items {
		seen[it.ID] = true
	}
	if !seen[actor.String()] || !seen[same.String()] {
		t.Fatalf("tenant-1 users must be listed, body=%s", body)
	}
	if seen[other.String()] {
		t.Fatalf("tenant-2 user must not be visible to tenant-1 admin, body=%s", body)
	}
}

// cross-tenant get/update/store-permissions/delete respond 404 and leave the
// target untouched, without leaking existence.
func TestCrossTenantUserOpsNotFound(t *testing.T) {
	db := openUserTestDB(t)
	actor := seedTenantUser(t, db, 1, "admin")
	other := seedTenantUser(t, db, 2, "operator")
	r := newTenantRouter(db, actor, 1)

	cases := []struct {
		method, path, body string
	}{
		{http.MethodGet, "/api/v1/admin/users/" + other.String(), ""},
		{http.MethodPatch, "/api/v1/admin/users/" + other.String(), `{"displayName":"x"}`},
		{http.MethodPut, "/api/v1/admin/users/" + other.String() + "/store-permissions", `{"items":[]}`},
		{http.MethodDelete, "/api/v1/admin/users/" + other.String(), ""},
	}
	for _, tc := range cases {
		if w := doJSON(r, tc.method, tc.path, tc.body); w.Code != http.StatusNotFound {
			t.Fatalf("%s %s: got %d body=%s, want 404", tc.method, tc.path, w.Code, w.Body.String())
		}
	}
	var visible int64
	if err := db.Model(&admin.AdminUser{}).Where("id = ?", other).Count(&visible).Error; err != nil {
		t.Fatal(err)
	}
	if visible != 1 {
		t.Fatalf("tenant-2 user must remain after cross-tenant attempts")
	}
}

// same-tenant operations still work for admin after tenant scoping.
func TestSameTenantUserOpsStillWork(t *testing.T) {
	db := openUserTestDB(t)
	actor := seedTenantUser(t, db, 1, "admin")
	target := seedTenantUser(t, db, 1, "operator")
	r := newTenantRouter(db, actor, 1)

	if w := doJSON(r, http.MethodGet, "/api/v1/admin/users/"+target.String(), ""); w.Code != http.StatusOK {
		t.Fatalf("same-tenant get: got %d body=%s, want 200", w.Code, w.Body.String())
	}
	if w := doJSON(r, http.MethodPatch, "/api/v1/admin/users/"+target.String(), `{"displayName":"renamed"}`); w.Code != http.StatusOK {
		t.Fatalf("same-tenant update: got %d body=%s, want 200", w.Code, w.Body.String())
	}
	if w := doJSON(r, http.MethodDelete, "/api/v1/admin/users/"+target.String(), ""); w.Code != http.StatusOK {
		t.Fatalf("same-tenant delete: got %d body=%s, want 200", w.Code, w.Body.String())
	}
}

// operator and readonly cannot access user management at all (403), including
// cross-tenant targets, so tenant scoping never even applies to them.
func TestTenantScopedRoutesRoleMatrix(t *testing.T) {
	db := openUserTestDB(t)
	other := seedTenantUser(t, db, 2, "operator")
	for _, role := range []string{"operator", "readonly"} {
		actor := seedTenantUser(t, db, 1, role)
		r := newTenantRouter(db, actor, 1)
		if w := doJSON(r, http.MethodGet, "/api/v1/admin/users", ""); w.Code != http.StatusForbidden {
			t.Fatalf("%s list: got %d body=%s, want 403", role, w.Code, w.Body.String())
		}
		if w := doJSON(r, http.MethodDelete, "/api/v1/admin/users/"+other.String(), ""); w.Code != http.StatusForbidden {
			t.Fatalf("%s delete: got %d body=%s, want 403", role, w.Code, w.Body.String())
		}
	}
}
