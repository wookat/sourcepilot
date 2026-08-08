package mcptoken_test

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/trademind-ai/trademind/backend/internal/modules/admin"
	"github.com/trademind-ai/trademind/backend/internal/modules/mcptoken"
	"github.com/trademind-ai/trademind/backend/internal/pkg/ctxkey"
	"gorm.io/gorm"
)

// R179 W1 D4: write:ops token governance is admin-only — operator and
// readonly admins cannot create write tokens, and write tokens cannot be
// created without expiry.

func seedAdmin(t *testing.T, db *gorm.DB, role string) uuid.UUID {
	t.Helper()
	if err := db.AutoMigrate(&admin.AdminUser{}); err != nil {
		t.Fatal(err)
	}
	row := admin.AdminUser{Username: role + "-" + uuid.NewString()[:8], Role: role, Status: "active", TenantID: 1}
	if err := db.Create(&row).Error; err != nil {
		t.Fatal(err)
	}
	return row.ID
}

func postCreate(t *testing.T, db *gorm.DB, adminID uuid.UUID, body string) *httptest.ResponseRecorder {
	t.Helper()
	gin.SetMode(gin.TestMode)
	h := &mcptoken.Handler{Svc: &mcptoken.Service{DB: db}}
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodPost, "/api/v1/mcp/tokens", strings.NewReader(body))
	c.Set(ctxkey.TenantID, int64(1))
	c.Set(ctxkey.AdminID, adminID.String())
	h.Create(c)
	return w
}

func TestWriteTokenCreateAdminOnly(t *testing.T) {
	db := openTestDB(t)
	writeBody := `{"name":"w","scopes":["readonly","write:ops"]}`

	for _, role := range []string{"operator", "readonly"} {
		w := postCreate(t, db, seedAdmin(t, db, role), writeBody)
		if w.Code != http.StatusForbidden {
			t.Fatalf("role %s: status = %d, want 403 (body %s)", role, w.Code, w.Body.String())
		}
	}
	// The same non-admin roles can still create plain readonly tokens.
	w := postCreate(t, db, seedAdmin(t, db, "operator"), `{"name":"ro"}`)
	if w.Code != http.StatusOK {
		t.Fatalf("operator readonly create: status = %d (body %s)", w.Code, w.Body.String())
	}

	adminID := seedAdmin(t, db, "admin")
	w = postCreate(t, db, adminID, writeBody)
	if w.Code != http.StatusOK {
		t.Fatalf("admin write create: status = %d (body %s)", w.Code, w.Body.String())
	}
	if !strings.Contains(w.Body.String(), `"scope":"readonly,write:ops"`) {
		t.Fatalf("scope missing in response: %s", w.Body.String())
	}
	if !strings.Contains(w.Body.String(), `"expiresAt"`) {
		t.Fatalf("write token created without expiry: %s", w.Body.String())
	}
	// Expiry above 90 days is rejected even for admin.
	w = postCreate(t, db, adminID, `{"name":"w2","scopes":["write:ops"],"expiresInDays":120}`)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("120d write expiry: status = %d (body %s)", w.Code, w.Body.String())
	}
}
