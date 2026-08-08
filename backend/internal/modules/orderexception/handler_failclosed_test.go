package orderexception

import (
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/trademind-ai/trademind/backend/internal/pkg/ctxkey"
	"github.com/trademind-ai/trademind/backend/internal/testing/faildb"
)

// R189: when the principal cannot be resolved the exception list must narrow to
// "no store", never fall back to the tenant-wide (nil = unrestricted) scope.
func TestRequestScopeFailsClosedWhenPrincipalUnavailable(t *testing.T) {
	gin.SetMode(gin.TestMode)
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Request = httptest.NewRequest("GET", "/order-exceptions", nil)
	c.Set(ctxkey.AdminID, uuid.New().String())
	c.Set(ctxkey.TenantID, int64(7))

	db, err := faildb.Closed()
	if err != nil {
		t.Fatalf("faildb: %v", err)
	}
	h := &Handler{Svc: &Service{DB: db}}
	tenantID, allowed := h.requestScope(c)
	if tenantID == nil || *tenantID != 7 {
		t.Fatalf("expected tenant 7, got %v", tenantID)
	}
	if allowed == nil {
		t.Fatal("expected an empty (non-nil) store scope; nil means every store")
	}
	if len(allowed) != 0 {
		t.Fatalf("expected no allowed store, got %v", allowed)
	}
}
