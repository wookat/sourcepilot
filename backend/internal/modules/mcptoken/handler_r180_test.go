package mcptoken_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/trademind-ai/trademind/backend/internal/modules/mcptoken"
	"github.com/trademind-ai/trademind/backend/internal/pkg/ctxkey"
	"gorm.io/gorm"
)

// R180 W2: write:ops tokens are admin-governed end to end — operator /
// readonly accounts can neither list nor revoke them.

func seedWriteAndReadTokens(t *testing.T, db *gorm.DB) (writeID, readID uuid.UUID) {
	t.Helper()
	svc := &mcptoken.Service{DB: db}
	w, err := svc.CreateScoped(context.Background(), 1, "writer", "",
		[]string{mcptoken.ScopeReadonly, mcptoken.ScopeWriteOps}, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	r, err := svc.Create(context.Background(), 1, "reader", "", nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	return w.Token.ID, r.Token.ID
}

func doList(t *testing.T, db *gorm.DB, adminID uuid.UUID) *httptest.ResponseRecorder {
	t.Helper()
	gin.SetMode(gin.TestMode)
	h := &mcptoken.Handler{Svc: &mcptoken.Service{DB: db}}
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodGet, "/api/v1/mcp/tokens", nil)
	c.Set(ctxkey.TenantID, int64(1))
	c.Set(ctxkey.AdminID, adminID.String())
	h.List(c)
	return w
}

func doRevoke(t *testing.T, db *gorm.DB, adminID, tokenID uuid.UUID) *httptest.ResponseRecorder {
	t.Helper()
	gin.SetMode(gin.TestMode)
	h := &mcptoken.Handler{Svc: &mcptoken.Service{DB: db}}
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodPost, "/api/v1/mcp/tokens/"+tokenID.String()+"/revoke", strings.NewReader("{}"))
	c.Params = gin.Params{{Key: "id", Value: tokenID.String()}}
	c.Set(ctxkey.TenantID, int64(1))
	c.Set(ctxkey.AdminID, adminID.String())
	h.Revoke(c)
	return w
}

func listedNames(t *testing.T, w *httptest.ResponseRecorder) []string {
	t.Helper()
	var body struct {
		Data struct {
			Items []struct {
				Name string `json:"name"`
			} `json:"items"`
		} `json:"data"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode list: %v (%s)", err, w.Body.String())
	}
	names := make([]string, 0, len(body.Data.Items))
	for _, it := range body.Data.Items {
		names = append(names, it.Name)
	}
	return names
}

func TestWriteTokenHiddenFromNonAdminList(t *testing.T) {
	db := openTestDB(t)
	seedWriteAndReadTokens(t, db)

	opNames := listedNames(t, doList(t, db, seedAdmin(t, db, "operator")))
	for _, n := range opNames {
		if n == "writer" {
			t.Fatalf("operator sees write token: %v", opNames)
		}
	}
	if len(opNames) != 1 || opNames[0] != "reader" {
		t.Fatalf("operator list = %v, want [reader]", opNames)
	}

	adNames := listedNames(t, doList(t, db, seedAdmin(t, db, "admin")))
	if len(adNames) != 2 {
		t.Fatalf("admin list = %v, want both tokens", adNames)
	}
}

func TestWriteTokenRevokeAdminOnly(t *testing.T) {
	db := openTestDB(t)
	writeID, readID := seedWriteAndReadTokens(t, db)

	// Operator revoking a write token gets 404 (invisible), token stays live.
	w := doRevoke(t, db, seedAdmin(t, db, "operator"), writeID)
	if w.Code != http.StatusNotFound {
		t.Fatalf("operator revoke write token: status = %d (body %s)", w.Code, w.Body.String())
	}
	var tok mcptoken.Token
	if err := db.First(&tok, "id = ?", writeID).Error; err != nil {
		t.Fatal(err)
	}
	if tok.RevokedAt != nil {
		t.Fatal("write token revoked by operator")
	}

	// Operator can still revoke readonly tokens.
	if w := doRevoke(t, db, seedAdmin(t, db, "operator"), readID); w.Code != http.StatusOK {
		t.Fatalf("operator revoke readonly token: status = %d (body %s)", w.Code, w.Body.String())
	}

	// Admin can revoke the write token.
	if w := doRevoke(t, db, seedAdmin(t, db, "admin"), writeID); w.Code != http.StatusOK {
		t.Fatalf("admin revoke write token: status = %d (body %s)", w.Code, w.Body.String())
	}
}
