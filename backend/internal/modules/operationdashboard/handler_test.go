package operationdashboard

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

func handlerTestCtx(t *testing.T, rawQuery string) (*gin.Context, *httptest.ResponseRecorder) {
	t.Helper()
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodGet, "/api/v1/dashboard/screen?"+rawQuery, nil)
	return c, w
}

// 非法 shopId 必须显式 400,不得静默降级为无店铺过滤。
func TestBindQueryRejectsInvalidShopID(t *testing.T) {
	h := &Handler{Svc: &Service{DB: newDryRunDB(t)}}
	c, _ := handlerTestCtx(t, "shopId=not-a-uuid")
	if _, err := h.bindQuery(c); !errors.Is(err, errShopID) {
		t.Fatalf("expected errShopID, got %v", err)
	}

	c, _ = handlerTestCtx(t, "shopId="+uuid.New().String())
	if _, err := h.bindQuery(c); err != nil {
		t.Fatalf("valid shopId rejected: %v", err)
	}

	c, _ = handlerTestCtx(t, "")
	if _, err := h.bindQuery(c); err != nil {
		t.Fatalf("empty shopId rejected: %v", err)
	}
}

// 大屏端点对非法 shopId 返回 HTTP 400(既有错误码口径)。
func TestScreenInvalidShopIDReturns400(t *testing.T) {
	h := &Handler{Svc: &Service{DB: newDryRunDB(t)}}
	c, w := handlerTestCtx(t, "shopId=bogus")
	h.Screen(c)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", w.Code)
	}
}
