package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/trademind-ai/trademind/backend/internal/config"
	"github.com/trademind-ai/trademind/backend/internal/modules/auth"
	"github.com/trademind-ai/trademind/backend/internal/pkg/security"
)

// Services and providers that only receive a context.Context (settings
// resolution, AI gateway) must be able to read the trusted tenant from the
// request context, not just from gin keys.
func TestBearerAuthAttachesTenantContextToRequestContext(t *testing.T) {
	t.Parallel()
	cfg := &config.Config{
		AppEnv:    config.EnvDevelopment,
		JWTSecret: "test-secret-request-context",
		Auth:      config.AuthConfig{SessionMode: config.AuthSessionModeLegacy, AccessTokenTTLMinutes: 15},
	}
	token, _, err := auth.LegacyMintToken(cfg, uuid.New(), "admin@example.com", 3, 1)
	if err != nil {
		t.Fatalf("mint legacy token: %v", err)
	}

	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(BearerAuth(cfg))
	var got int64 = -1
	r.GET("/ping", func(c *gin.Context) {
		if tc := security.FromContext(c.Request.Context()); tc != nil {
			got = tc.TenantID
		}
		c.String(200, "ok")
	})
	req := httptest.NewRequest(http.MethodGet, "/ping", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d body=%s", w.Code, w.Body.String())
	}
	if got != 3 {
		t.Fatalf("expected tenant 3 on request context, got %d", got)
	}
}
