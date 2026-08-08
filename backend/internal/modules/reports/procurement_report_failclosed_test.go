package reports

import (
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/trademind-ai/trademind/backend/internal/pkg/ctxkey"
	"github.com/trademind-ai/trademind/backend/internal/testing/faildb"
)

// R189: allowedShopIDs used to answer (nil, nil) when the principal could not
// be resolved, and nil is the admin scope: the procurement report then covered
// every store of the tenant. An unresolvable principal must fail the request.
func TestAllowedShopIDsFailsClosedWhenPrincipalUnavailable(t *testing.T) {
	gin.SetMode(gin.TestMode)
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Request = httptest.NewRequest("GET", "/reports/procurement", nil)
	c.Set(ctxkey.AdminID, uuid.New().String())
	c.Set(ctxkey.TenantID, int64(11))

	db, err := faildb.Closed()
	if err != nil {
		t.Fatalf("faildb: %v", err)
	}
	ids, err := allowedShopIDs(c, &Service{DB: db})
	if err == nil {
		t.Fatal("expected the unresolved principal to fail the report")
	}
	if ids != nil {
		t.Fatalf("expected no store scope on failure, got %v", ids)
	}
}
