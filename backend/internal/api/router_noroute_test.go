package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/trademind-ai/trademind/backend/internal/pkg/response"
)

// 未知路由返回统一 JSON 404 envelope（中文口径），而非 Gin 裸文本。
func TestRegisterNoRoute_unknownRouteReturnsJSON404(t *testing.T) {
	t.Parallel()
	gin.SetMode(gin.TestMode)
	e := gin.New()
	RegisterNoRoute(e)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/definitely-not-a-route", nil)
	w := httptest.NewRecorder()
	e.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", w.Code)
	}
	if ct := w.Header().Get("Content-Type"); !strings.Contains(ct, "application/json") {
		t.Fatalf("expected JSON content type, got %q", ct)
	}
	var body struct {
		Code    int    `json:"code"`
		Message string `json:"message"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatalf("unmarshal body: %v (body=%s)", err, w.Body.String())
	}
	if body.Code != response.CodeNotFound {
		t.Fatalf("expected code %d, got %d", response.CodeNotFound, body.Code)
	}
	if !strings.Contains(body.Message, "接口不存在") {
		t.Fatalf("expected Chinese not-found message, got %q", body.Message)
	}
}
