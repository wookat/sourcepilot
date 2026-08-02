package collect

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
	"github.com/trademind-ai/trademind/backend/internal/pkg/ctxkey"
	"gorm.io/gorm"
)

func newCollectTestGinContext(t *testing.T) *gin.Context {
	t.Helper()
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodPost, "/api/v1/collect/tasks", nil)
	return c
}

func TestCreateTaskAsyncRejectsMissingTenantContext(t *testing.T) {
	c := newCollectTestGinContext(t)
	svc := &Service{DB: &gorm.DB{}, QueueEnabled: true}

	_, err := svc.CreateTaskAsync(c, CreateTaskBody{Source: "1688", URL: "https://example.com/item"}, nil)
	require.Error(t, err)
	require.Contains(t, err.Error(), "TENANT_CONTEXT_MISSING")
}

func TestCreateTaskAsyncPassesTenantGateWithTenantContext(t *testing.T) {
	c := newCollectTestGinContext(t)
	c.Set(ctxkey.TenantID, int64(1))
	svc := &Service{DB: &gorm.DB{}, QueueEnabled: false}

	_, err := svc.CreateTaskAsync(c, CreateTaskBody{Source: "1688", URL: "https://example.com/item"}, nil)
	require.ErrorIs(t, err, ErrCollectQueueDisabled)
}

func TestCreateBatchAsyncRejectsMissingTenantContext(t *testing.T) {
	c := newCollectTestGinContext(t)
	svc := &Service{DB: &gorm.DB{}, QueueEnabled: true}

	_, err := svc.CreateBatchAsync(c, CreateBatchBody{Source: "1688", URLs: []string{"https://example.com/item"}}, nil)
	require.Error(t, err)
	require.Contains(t, err.Error(), "TENANT_CONTEXT_MISSING")
}
