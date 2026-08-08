package inventory

import (
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/trademind-ai/trademind/backend/internal/pkg/ctxkey"
	"github.com/trademind-ai/trademind/backend/internal/testing/faildb"
)

// R189: the sync-batch write endpoints treat a nil operable set as "every
// store". An unresolvable principal must therefore resolve to the empty set, or
// a degraded permission read would hand out tenant-wide write scope.
func TestOperableShopIDsFailsClosedWhenPrincipalUnavailable(t *testing.T) {
	gin.SetMode(gin.TestMode)
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Request = httptest.NewRequest("POST", "/inventory-sync/batches", nil)
	c.Set(ctxkey.AdminID, uuid.New().String())
	c.Set(ctxkey.TenantID, int64(3))

	db, err := faildb.Closed()
	if err != nil {
		t.Fatalf("faildb: %v", err)
	}
	h := &Handler{Svc: &Service{DB: db}}
	ids := h.operableShopIDs(c)
	if ids == nil {
		t.Fatal("expected an empty (non-nil) operable set; nil means every store")
	}
	if len(ids) != 0 {
		t.Fatalf("expected no operable store, got %v", ids)
	}
	if shopInOperable(ids, uuid.New()) {
		t.Fatal("empty operable set must not authorize any store")
	}
}
