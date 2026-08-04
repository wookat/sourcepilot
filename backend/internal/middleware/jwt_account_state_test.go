package middleware_test

import (
	"net/http"
	"testing"

	"github.com/google/uuid"
	"github.com/trademind-ai/trademind/backend/internal/config"
	"github.com/trademind-ai/trademind/backend/internal/modules/admin"
)

// Legacy JWTs (no secure session) must be revoked at request time when the
// account behind them is disabled, deleted or has a bumped token_version.
func TestLegacyTokenRejectedForDisabledAccount(t *testing.T) {
	db := openMiddlewareTestDB(t)
	cfg := &config.Config{AppEnv: config.EnvDevelopment, JWTSecret: "test-secret-0123456789"}
	uid := seedUser(t, db, 7, "disabled")
	w := doAuthedRequest(cfg, db, mintToken(t, cfg, uid, 7))
	if w.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401 for disabled account, got %d body=%s", w.Code, w.Body.String())
	}
}

func TestLegacyTokenRejectedForMissingAccount(t *testing.T) {
	db := openMiddlewareTestDB(t)
	cfg := &config.Config{AppEnv: config.EnvDevelopment, JWTSecret: "test-secret-0123456789"}
	w := doAuthedRequest(cfg, db, mintToken(t, cfg, uuid.New(), 7))
	if w.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401 for missing account, got %d body=%s", w.Code, w.Body.String())
	}
}

func TestLegacyTokenRejectedAfterTokenVersionBump(t *testing.T) {
	db := openMiddlewareTestDB(t)
	cfg := &config.Config{AppEnv: config.EnvDevelopment, JWTSecret: "test-secret-0123456789"}
	uid := seedUser(t, db, 7, "active")
	tok := mintToken(t, cfg, uid, 7)
	if w := doAuthedRequest(cfg, db, tok); w.Code != http.StatusOK {
		t.Fatalf("expected 200 before bump, got %d body=%s", w.Code, w.Body.String())
	}
	if err := db.Model(&admin.AdminUser{}).Where("id = ?", uid).
		Update("token_version", 2).Error; err != nil {
		t.Fatal(err)
	}
	if w := doAuthedRequest(cfg, db, tok); w.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401 after token_version bump, got %d body=%s", w.Code, w.Body.String())
	}
}
