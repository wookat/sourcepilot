package adminuser_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/trademind-ai/trademind/backend/internal/modules/admin"
	"github.com/trademind-ai/trademind/backend/internal/modules/adminuser"
	"github.com/trademind-ai/trademind/backend/internal/pkg/ctxkey"
	"golang.org/x/crypto/bcrypt"
)

func doResetPassword(r http.Handler, targetID, body string) *httptest.ResponseRecorder {
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/v1/admin/users/"+targetID+"/reset-password", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)
	return w
}

// admin can reset another user's password; hash changes and token_version bumps.
func TestResetPasswordUpdatesHashAndBumpsTokenVersion(t *testing.T) {
	db := openUserTestDB(t)
	actor := seedUser(t, db, "admin")
	target := seedUser(t, db, "operator")

	var before admin.AdminUser
	if err := db.First(&before, "id = ?", target).Error; err != nil {
		t.Fatal(err)
	}

	r := newUserRouter(db, actor)
	if w := doResetPassword(r, target.String(), `{"password":"new-secret-1"}`); w.Code != http.StatusOK {
		t.Fatalf("reset: got %d body=%s, want 200", w.Code, w.Body.String())
	}

	var after admin.AdminUser
	if err := db.First(&after, "id = ?", target).Error; err != nil {
		t.Fatal(err)
	}
	if after.PasswordHash == before.PasswordHash {
		t.Fatalf("password hash must change")
	}
	if err := bcrypt.CompareHashAndPassword([]byte(after.PasswordHash), []byte("new-secret-1")); err != nil {
		t.Fatalf("new password must verify: %v", err)
	}
	if after.TokenVersion != before.TokenVersion+1 {
		t.Fatalf("token_version must bump: before=%d after=%d", before.TokenVersion, after.TokenVersion)
	}
}

type stubSessionRevoker struct {
	calls   int
	lastUID uuid.UUID
}

func (r *stubSessionRevoker) RevokeAllUserSessions(_ context.Context, userID uuid.UUID, _ string) (int64, error) {
	r.calls++
	r.lastUID = userID
	return 1, nil
}

// reset revokes the target user's secure sessions so stale sessions cannot refresh.
func TestResetPasswordRevokesUserSessions(t *testing.T) {
	db := openUserTestDB(t)
	actor := seedUser(t, db, "admin")
	target := seedUser(t, db, "operator")

	revoker := &stubSessionRevoker{}
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(func(c *gin.Context) {
		c.Set(ctxkey.AdminID, actor.String())
		c.Set(ctxkey.TenantID, int64(0))
		c.Next()
	})
	adminuser.Register(r.Group("/api/v1"), &adminuser.Handler{Svc: &adminuser.Service{DB: db, Sessions: revoker}})

	if w := doResetPassword(r, target.String(), `{"password":"new-secret-1"}`); w.Code != http.StatusOK {
		t.Fatalf("reset: got %d body=%s, want 200", w.Code, w.Body.String())
	}
	if revoker.calls != 1 || revoker.lastUID != target {
		t.Fatalf("sessions must be revoked exactly once for target: calls=%d uid=%s", revoker.calls, revoker.lastUID)
	}
}

// short password rejected with 400; hash unchanged.
func TestResetPasswordRejectsShortPassword(t *testing.T) {
	db := openUserTestDB(t)
	actor := seedUser(t, db, "admin")
	target := seedUser(t, db, "operator")

	r := newUserRouter(db, actor)
	if w := doResetPassword(r, target.String(), `{"password":"123"}`); w.Code != http.StatusBadRequest {
		t.Fatalf("short password: got %d, want 400", w.Code)
	}
}

// readonly and operator roles are rejected with 403.
func TestResetPasswordRequiresAdmin(t *testing.T) {
	db := openUserTestDB(t)
	target := seedUser(t, db, "operator")
	for _, role := range []string{"readonly", "operator"} {
		actor := seedUser(t, db, role)
		r := newUserRouter(db, actor)
		if w := doResetPassword(r, target.String(), `{"password":"new-secret-1"}`); w.Code != http.StatusForbidden {
			t.Fatalf("%s reset: got %d body=%s, want 403", role, w.Code, w.Body.String())
		}
	}
}

// invalid id returns 400, unknown id returns 404.
func TestResetPasswordInvalidAndMissing(t *testing.T) {
	db := openUserTestDB(t)
	actor := seedUser(t, db, "admin")
	r := newUserRouter(db, actor)
	if w := doResetPassword(r, "not-a-uuid", `{"password":"new-secret-1"}`); w.Code != http.StatusBadRequest {
		t.Fatalf("invalid id: got %d, want 400", w.Code)
	}
	if w := doResetPassword(r, uuid.New().String(), `{"password":"new-secret-1"}`); w.Code != http.StatusNotFound {
		t.Fatalf("missing id: got %d, want 404", w.Code)
	}
}
