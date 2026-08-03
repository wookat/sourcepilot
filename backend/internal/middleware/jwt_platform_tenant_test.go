package middleware_test

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"github.com/google/uuid"
	"github.com/trademind-ai/trademind/backend/internal/config"
	"github.com/trademind-ai/trademind/backend/internal/middleware"
	"github.com/trademind-ai/trademind/backend/internal/modules/admin"
	"github.com/trademind-ai/trademind/backend/internal/modules/auth"
	"github.com/trademind-ai/trademind/backend/internal/pkg/model"
	"gorm.io/gorm"
)

func openMiddlewareTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	dsn := fmt.Sprintf("file:middleware_%s?mode=memory&cache=shared&_pragma=busy_timeout(5000)", uuid.New().String())
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Skipf("sqlite unavailable: %v", err)
	}
	sqlDB, _ := db.DB()
	sqlDB.SetMaxOpenConns(1)
	if err := db.AutoMigrate(&admin.AdminUser{}); err != nil {
		t.Fatal(err)
	}
	return db
}

func seedUser(t *testing.T, db *gorm.DB, tenantID int64, status string) uuid.UUID {
	t.Helper()
	uid := uuid.New()
	if err := db.Create(&admin.AdminUser{
		Base:         model.Base{ID: uid},
		TenantID:     tenantID,
		Username:     "u-" + uid.String()[:12],
		Email:        "u-" + uid.String()[:12] + "@example.com",
		PasswordHash: "x",
		Role:         "admin",
		Status:       status,
	}).Error; err != nil {
		t.Fatal(err)
	}
	return uid
}

func mintToken(t *testing.T, cfg *config.Config, uid uuid.UUID, tenantID int64) string {
	t.Helper()
	ks, err := auth.BuildKeySet(cfg)
	if err != nil {
		t.Fatal(err)
	}
	tok, _, err := auth.MintAccessToken(cfg, ks, auth.MintAccessInput{
		UserID:       uid,
		Username:     "u",
		TenantID:     tenantID,
		SessionID:    uuid.Nil,
		TokenVersion: 1,
	})
	if err != nil {
		t.Fatal(err)
	}
	return tok
}

func doAuthedRequest(cfg *config.Config, db *gorm.DB, token string) *httptest.ResponseRecorder {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(middleware.BearerAuthWithDB(cfg, db, nil))
	r.GET("/ping", func(c *gin.Context) { c.JSON(200, gin.H{"code": 0}) })
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/ping", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	r.ServeHTTP(w, req)
	return w
}

// Production must accept a tenant-0 token for an active platform tenant admin
// (bootstrap platform admin, #181) instead of rejecting it as tenant fallback.
func TestProductionAllowsActivePlatformTenantAdmin(t *testing.T) {
	db := openMiddlewareTestDB(t)
	cfg := &config.Config{AppEnv: config.EnvProduction, JWTSecret: "test-secret-0123456789"}
	uid := seedUser(t, db, 0, "active")
	w := doAuthedRequest(cfg, db, mintToken(t, cfg, uid, 0))
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200 for platform tenant admin, got %d body=%s", w.Code, w.Body.String())
	}
}

// A tenant-0 token whose user row is missing or disabled keeps the production
// tenant-fallback rejection.
func TestProductionRejectsTenantZeroWithoutPlatformUser(t *testing.T) {
	db := openMiddlewareTestDB(t)
	cfg := &config.Config{AppEnv: config.EnvProduction, JWTSecret: "test-secret-0123456789"}

	unknown := uuid.New()
	w := doAuthedRequest(cfg, db, mintToken(t, cfg, unknown, 0))
	if w.Code != http.StatusForbidden {
		t.Fatalf("expected 403 for unknown user, got %d body=%s", w.Code, w.Body.String())
	}

	disabled := seedUser(t, db, 0, "disabled")
	w = doAuthedRequest(cfg, db, mintToken(t, cfg, disabled, 0))
	if w.Code != http.StatusForbidden {
		t.Fatalf("expected 403 for disabled user, got %d body=%s", w.Code, w.Body.String())
	}

	var body struct {
		Message string `json:"message"`
	}
	_ = json.Unmarshal(w.Body.Bytes(), &body)
	if body.Message == "" {
		t.Fatalf("expected envelope message, got %s", w.Body.String())
	}
}

// Business tenant tokens (>0) keep working unchanged in production.
func TestProductionBusinessTenantTokenUnchanged(t *testing.T) {
	db := openMiddlewareTestDB(t)
	cfg := &config.Config{AppEnv: config.EnvProduction, JWTSecret: "test-secret-0123456789"}
	uid := seedUser(t, db, 3, "active")
	w := doAuthedRequest(cfg, db, mintToken(t, cfg, uid, 3))
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200 for business tenant token, got %d body=%s", w.Code, w.Body.String())
	}
}

// A soft-deleted platform admin row must not pass the tenant-0 DB check even
// while an already-issued token is still cryptographically valid.
func TestProductionRejectsSoftDeletedPlatformAdmin(t *testing.T) {
	db := openMiddlewareTestDB(t)
	cfg := &config.Config{AppEnv: config.EnvProduction, JWTSecret: "test-secret-0123456789"}
	uid := seedUser(t, db, 0, "active")
	if err := db.Delete(&admin.AdminUser{Base: model.Base{ID: uid}}).Error; err != nil {
		t.Fatal(err)
	}
	w := doAuthedRequest(cfg, db, mintToken(t, cfg, uid, 0))
	if w.Code != http.StatusForbidden {
		t.Fatalf("expected 403 for soft-deleted platform admin, got %d body=%s", w.Code, w.Body.String())
	}
}

func mintBoundToken(t *testing.T, cfg *config.Config, uid uuid.UUID, tenantID int64) string {
	t.Helper()
	ks, err := auth.BuildKeySet(cfg)
	if err != nil {
		t.Fatal(err)
	}
	tok, _, err := auth.MintAccessToken(cfg, ks, auth.MintAccessInput{
		UserID:       uid,
		Username:     "u",
		TenantID:     tenantID,
		SessionID:    uuid.New(),
		TokenVersion: 1,
	})
	if err != nil {
		t.Fatal(err)
	}
	return tok
}

// Under the real production default (secure_session), a session-bound token of
// an active platform admin passes; a session-bound tenant-0 token without a
// matching platform admin row keeps the fallback rejection.
func TestSecureSessionProductionPlatformTenantAdmin(t *testing.T) {
	db := openMiddlewareTestDB(t)
	cfg := &config.Config{
		AppEnv:    config.EnvProduction,
		JWTSecret: "test-secret-0123456789",
		Auth:      config.AuthConfig{SessionMode: config.AuthSessionModeSecure},
	}
	uid := seedUser(t, db, 0, "active")
	w := doAuthedRequest(cfg, db, mintBoundToken(t, cfg, uid, 0))
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200 for platform admin under secure_session, got %d body=%s", w.Code, w.Body.String())
	}
	w = doAuthedRequest(cfg, db, mintBoundToken(t, cfg, uuid.New(), 0))
	if w.Code != http.StatusForbidden {
		t.Fatalf("expected 403 for unknown tenant-0 user under secure_session, got %d body=%s", w.Code, w.Body.String())
	}
}
