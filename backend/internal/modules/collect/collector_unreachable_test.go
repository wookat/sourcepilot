package collect

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/trademind-ai/trademind/backend/internal/pkg/response"
)

// Round 85: when the collector service is down, proxy endpoints must return a
// stable envelope (502 + CodeCollectorUnreachable + Chinese guidance) instead
// of a raw transport error, so the settings page can render a guidance state.
func TestAuthStatusCollectorUnreachableEnvelope(t *testing.T) {
	gin.SetMode(gin.TestMode)
	// Port 1 on localhost: connection refused immediately.
	client := NewCollectorClient("http://127.0.0.1:1", 2*time.Second)
	h := &Handler{Svc: &Service{Client: client}}

	r := gin.New()
	r.GET("/auth-status", h.Get1688AuthStatus)
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/auth-status", nil)
	r.ServeHTTP(w, req)

	if w.Code != http.StatusBadGateway {
		t.Fatalf("expected 502, got %d", w.Code)
	}
	var env struct {
		Code    int    `json:"code"`
		Message string `json:"message"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &env); err != nil {
		t.Fatalf("invalid envelope: %v", err)
	}
	if env.Code != response.CodeCollectorUnreachable {
		t.Errorf("expected code %d, got %d", response.CodeCollectorUnreachable, env.Code)
	}
	if env.Message != collectorUnreachableMessage {
		t.Errorf("expected Chinese guidance message, got %q", env.Message)
	}
}

// Collector business rejections keep their original message/code (no
// unreachable masking).
func TestAuthStatusCollectorRejectedKeepsMessage(t *testing.T) {
	gin.SetMode(gin.TestMode)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusUnprocessableEntity)
		_, _ = w.Write([]byte(`{"ok":false,"error":{"code":"BROWSER_BUSY","message":"浏览器忙"}}`))
	}))
	defer srv.Close()

	client := NewCollectorClient(srv.URL, 2*time.Second)
	h := &Handler{Svc: &Service{Client: client}}

	r := gin.New()
	r.GET("/auth-status", h.Get1688AuthStatus)
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/auth-status", nil)
	r.ServeHTTP(w, req)

	if w.Code != http.StatusBadGateway {
		t.Fatalf("expected 502, got %d", w.Code)
	}
	var env struct {
		Code    int    `json:"code"`
		Message string `json:"message"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &env); err != nil {
		t.Fatalf("invalid envelope: %v", err)
	}
	if env.Code != response.CodeInternalError {
		t.Errorf("expected code %d, got %d", response.CodeInternalError, env.Code)
	}
	if env.Message == collectorUnreachableMessage {
		t.Error("rejection must not be masked as unreachable")
	}
}
