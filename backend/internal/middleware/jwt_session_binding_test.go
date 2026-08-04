package middleware

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/trademind-ai/trademind/backend/internal/config"
	"github.com/trademind-ai/trademind/backend/internal/modules/auth"
)

func sessionBindingConfig(mode string) *config.Config {
	return &config.Config{
		AppEnv:    config.EnvProduction,
		JWTSecret: "test-secret-session-binding",
		Auth:      config.AuthConfig{SessionMode: mode, AccessTokenTTLMinutes: 15},
	}
}

func serveWithBearer(t *testing.T, cfg *config.Config, token string) *httptest.ResponseRecorder {
	t.Helper()
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(BearerAuth(cfg))
	r.GET("/ping", func(c *gin.Context) { c.String(200, "ok") })
	req := httptest.NewRequest(http.MethodGet, "/ping", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	return w
}

// secure_session 模式拒绝无 session 绑定的 legacy token，返回 401 引导重新登录。
func TestBearerAuth_secureSessionRejectsLegacyToken(t *testing.T) {
	t.Parallel()
	cfg := sessionBindingConfig(config.AuthSessionModeSecure)
	token, _, err := auth.LegacyMintToken(cfg, uuid.New(), "admin@example.com", 3, 1)
	if err != nil {
		t.Fatalf("mint legacy token: %v", err)
	}
	w := serveWithBearer(t, cfg, token)
	if w.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d body=%s", w.Code, w.Body.String())
	}
	var body struct {
		Message string `json:"message"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatalf("unmarshal body: %v", err)
	}
	if body.Message != auth.ErrSessionBindingRequired {
		t.Fatalf("expected %s, got %q", auth.ErrSessionBindingRequired, body.Message)
	}
}

// secure_session 模式下带 session 绑定的 access token 正常通过。
func TestBearerAuth_secureSessionAcceptsSessionBoundToken(t *testing.T) {
	t.Parallel()
	cfg := sessionBindingConfig(config.AuthSessionModeSecure)
	ks, err := auth.BuildKeySet(cfg)
	if err != nil {
		t.Fatalf("build key set: %v", err)
	}
	token, _, err := auth.MintAccessToken(cfg, ks, auth.MintAccessInput{
		UserID:       uuid.New(),
		Username:     "admin@example.com",
		TenantID:     3,
		SessionID:    uuid.New(),
		TokenVersion: 1,
	})
	if err != nil {
		t.Fatalf("mint access token: %v", err)
	}
	w := serveWithBearer(t, cfg, token)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d body=%s", w.Code, w.Body.String())
	}
}

// legacy_local_storage 模式（开发/遗留部署）继续接受 legacy token。
func TestBearerAuth_legacyModeStillAcceptsLegacyToken(t *testing.T) {
	t.Parallel()
	cfg := sessionBindingConfig(config.AuthSessionModeLegacy)
	cfg.AppEnv = config.EnvDevelopment
	token, _, err := auth.LegacyMintToken(cfg, uuid.New(), "admin@example.com", 0, 1)
	if err != nil {
		t.Fatalf("mint legacy token: %v", err)
	}
	w := serveWithBearer(t, cfg, token)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d body=%s", w.Code, w.Body.String())
	}
}
