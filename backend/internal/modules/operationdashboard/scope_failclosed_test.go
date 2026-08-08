package operationdashboard

import (
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/trademind-ai/trademind/backend/internal/pkg/ctxkey"
	"github.com/trademind-ai/trademind/backend/internal/testing/faildb"
)

// R189: an unresolvable principal must not be promoted to the admin scope,
// which would expose every store of the tenant on the dashboard.
func TestScopeFromContextFailsClosedWhenPrincipalUnavailable(t *testing.T) {
	gin.SetMode(gin.TestMode)
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Request = httptest.NewRequest("GET", "/operation-dashboard/summary", nil)
	c.Set(ctxkey.AdminID, uuid.New().String())
	c.Set(ctxkey.TenantID, int64(5))

	db, err := faildb.Closed()
	if err != nil {
		t.Fatalf("faildb: %v", err)
	}
	sc := scopeFromContext(c, db)
	if sc.IsAdmin {
		t.Fatal("unresolved principal must not get the admin scope")
	}
	if sc.AllowedShopIDs == nil || len(sc.AllowedShopIDs) != 0 {
		t.Fatalf("expected an empty (non-nil) store scope, got %v", sc.AllowedShopIDs)
	}
	if sc.TenantID == nil || *sc.TenantID != 5 {
		t.Fatalf("expected tenant 5, got %v", sc.TenantID)
	}
}
