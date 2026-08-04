package auth

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/trademind-ai/trademind/backend/internal/config"
	"github.com/trademind-ai/trademind/backend/internal/modules/admin"
)

func newSessionFailClosedFixture(t *testing.T) (*SessionService, *LoginSessionResult, *admin.AdminUser) {
	t.Helper()
	db := newTenantStateTestDB(t)
	if err := db.AutoMigrate(&AuthSession{}, &AuthRefreshToken{}, &AuthLoginAttempt{}); err != nil {
		t.Fatal(err)
	}
	email := seedTenantStateUser(t, db, 0)
	cfg := &config.Config{
		JWTSecret: tenantStateJWTSecret(),
		Auth: config.AuthConfig{
			SessionMode:           config.AuthSessionModeSecure,
			AccessTokenTTLMinutes: 15,
			RefreshTokenTTLDays:   7,
		},
	}
	svc := &SessionService{Cfg: cfg, DB: db, Admins: &admin.Store{DB: db}}
	res, err := svc.CreateSession(context.Background(), email, tenantStatePW, "127.0.0.1", "test")
	if err != nil {
		t.Fatal(err)
	}
	var u admin.AdminUser
	if err := db.First(&u, "email = ?", email).Error; err != nil {
		t.Fatal(err)
	}
	return svc, res, &u
}

// A missing session row stays a revocation, not an availability problem.
func TestValidateSessionAccessMissingSessionIsRevoked(t *testing.T) {
	svc, res, u := newSessionFailClosedFixture(t)
	if err := svc.DB.Exec(`DELETE FROM auth_sessions`).Error; err != nil {
		t.Fatal(err)
	}
	if err := svc.ValidateSessionAccess(context.Background(), res.SessionID, u.ID, u.TokenVersion); err == nil || err.Error() != ErrSessionRevoked {
		t.Fatalf("want %s, got %v", ErrSessionRevoked, err)
	}
}

// A database outage with no fresh snapshot fails closed with
// AUTH_STATE_UNAVAILABLE instead of masquerading as a revoked session
// (which would force-log-out every user on a transient blip).
func TestValidateSessionAccessDBErrorFailsClosedUnavailable(t *testing.T) {
	svc, res, u := newSessionFailClosedFixture(t)
	sessionStateCache.Delete(res.SessionID)
	if err := svc.DB.Exec(`DROP TABLE auth_sessions`).Error; err != nil {
		t.Fatal(err)
	}
	if err := svc.ValidateSessionAccess(context.Background(), res.SessionID, u.ID, u.TokenVersion); err == nil || err.Error() != ErrAuthStateUnavailable {
		t.Fatalf("want %s, got %v", ErrAuthStateUnavailable, err)
	}
}

// A fresh last-known-good snapshot bridges a transient outage; an expired
// snapshot fails closed again.
func TestValidateSessionAccessFreshSnapshotBridgesOutage(t *testing.T) {
	svc, res, u := newSessionFailClosedFixture(t)
	// Warm the snapshot with a successful validation.
	if err := svc.ValidateSessionAccess(context.Background(), res.SessionID, u.ID, u.TokenVersion); err != nil {
		t.Fatalf("warm validate: %v", err)
	}
	if err := svc.DB.Exec(`DROP TABLE auth_sessions`).Error; err != nil {
		t.Fatal(err)
	}
	if err := svc.ValidateSessionAccess(context.Background(), res.SessionID, u.ID, u.TokenVersion); err != nil {
		t.Fatalf("fresh snapshot should bridge db error, got %v", err)
	}
	// A snapshot for another user must not bridge.
	if err := svc.ValidateSessionAccess(context.Background(), res.SessionID, uuid.New(), u.TokenVersion); err == nil || err.Error() != ErrSessionRevoked {
		t.Fatalf("want %s, got %v", ErrSessionRevoked, err)
	}
	// An expired snapshot fails closed with AUTH_STATE_UNAVAILABLE.
	sessionStateCache.Store(res.SessionID, sessionAccessSnapshot{userID: u.ID, tokenVersion: u.TokenVersion, at: time.Now().Add(-authStateCacheTTL - time.Second)})
	if err := svc.ValidateSessionAccess(context.Background(), res.SessionID, u.ID, u.TokenVersion); err == nil || err.Error() != ErrAuthStateUnavailable {
		t.Fatalf("want %s, got %v", ErrAuthStateUnavailable, err)
	}
}
