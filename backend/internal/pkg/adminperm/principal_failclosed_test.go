package adminperm

import (
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/trademind-ai/trademind/backend/internal/pkg/ctxkey"
	"github.com/trademind-ai/trademind/backend/internal/testing/faildb"
	"gorm.io/gorm"
)

func failClosedContext(t *testing.T) (*gin.Context, *gorm.DB) {
	t.Helper()
	gin.SetMode(gin.TestMode)
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Request = httptest.NewRequest("GET", "/", nil)
	c.Set(ctxkey.AdminID, uuid.New().String())
	c.Set(ctxkey.TenantID, int64(1))
	db, err := faildb.Closed()
	if err != nil {
		t.Fatalf("faildb: %v", err)
	}
	return c, db
}

// R189: an unresolvable principal (database error, not "row missing") must
// still yield a principal that authorizes nothing. Returning nil made every
// caller that only reads the principal widen the store scope to "all stores".
func TestLoadPrincipalFailsClosedOnDBError(t *testing.T) {
	c, db := failClosedContext(t)
	p, err := LoadPrincipal(c, db)
	if err == nil {
		t.Fatal("expected a database error")
	}
	if p == nil {
		t.Fatal("expected a deny-all principal, got nil (callers widen the scope)")
	}
	if p.IsAdmin() {
		t.Fatal("unresolved principal must not be admin")
	}
	if ids := p.AllowedStoreIDs(); ids == nil || len(ids) != 0 {
		t.Fatalf("expected an empty (non-nil) allowed store set, got %v", ids)
	}
	if ids := p.OperableStoreIDs(); ids == nil || len(ids) != 0 {
		t.Fatalf("expected an empty (non-nil) operable store set, got %v", ids)
	}
	if p.Can(PermSettingsManage) || p.Can(PermProductWrite) {
		t.Fatal("unresolved principal must hold no permission")
	}
	if p.CanViewStore(uuid.New()) || p.CanOperateStore(uuid.New()) {
		t.Fatal("unresolved principal must not view or operate any store")
	}
}

// The deny-all principal must not be cached: a transient database error may not
// pin the caller to "no access" for the rest of the request lifecycle.
func TestLoadPrincipalDBErrorNotCached(t *testing.T) {
	c, db := failClosedContext(t)
	if _, err := LoadPrincipal(c, db); err == nil {
		t.Fatal("expected a database error")
	}
	if _, ok := c.Get(ctxPrincipalKey); ok {
		t.Fatal("deny-all principal must not be cached on the request context")
	}
}
