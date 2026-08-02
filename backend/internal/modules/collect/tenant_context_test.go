package collect

import (
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/trademind-ai/trademind/backend/internal/pkg/security"
)

func TestTenantIDFromGin(t *testing.T) {
	gin.SetMode(gin.TestMode)

	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	if _, err := tenantIDFromGin(c); err == nil {
		t.Fatal("expected error when tenant context missing")
	}

	c2, _ := gin.CreateTestContext(httptest.NewRecorder())
	security.SetGin(c2, &security.TenantContext{TenantID: 0})
	if _, err := tenantIDFromGin(c2); err == nil {
		t.Fatal("expected error when tenant id is not positive")
	}

	c3, _ := gin.CreateTestContext(httptest.NewRecorder())
	security.SetGin(c3, &security.TenantContext{TenantID: 7})
	got, err := tenantIDFromGin(c3)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != 7 {
		t.Fatalf("expected tenant 7, got %d", got)
	}
}
