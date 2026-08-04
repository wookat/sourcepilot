package auth

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/trademind-ai/trademind/backend/internal/modules/admin"
	"github.com/trademind-ai/trademind/backend/internal/pkg/model"
)

// A database error with no fresh snapshot must fail closed instead of
// letting the request through (round103: fail-open → TTL cache + fail-closed).
func TestEnsureAccountActiveFailsClosedWithoutCache(t *testing.T) {
	db := newTenantStateTestDB(t)
	if err := db.Migrator().DropTable(&admin.AdminUser{}); err != nil {
		t.Fatal(err)
	}
	uid := uuid.New()
	if err := EnsureAccountActive(context.Background(), db, uid, 1); err == nil || err.Error() != ErrAuthStateUnavailable {
		t.Fatalf("want %s, got %v", ErrAuthStateUnavailable, err)
	}
}

// A fresh last-known-good snapshot bridges a transient database error so a
// blip does not lock every tenant out.
func TestEnsureAccountActiveUsesFreshCacheOnDBError(t *testing.T) {
	db := newTenantStateTestDB(t)
	uid := uuid.New()
	if err := db.Create(&admin.AdminUser{
		Base:         model.Base{ID: uid},
		TenantID:     1,
		Username:     "cache-" + uuid.NewString()[:12],
		Email:        "cache-" + uuid.NewString()[:8] + "@example.com",
		PasswordHash: "x",
		Role:         "admin",
		Status:       "active",
	}).Error; err != nil {
		t.Fatal(err)
	}
	// Warm the cache with a successful read.
	if err := EnsureAccountActive(context.Background(), db, uid, 1); err != nil {
		t.Fatalf("warm read: %v", err)
	}
	if err := db.Migrator().DropTable(&admin.AdminUser{}); err != nil {
		t.Fatal(err)
	}
	if err := EnsureAccountActive(context.Background(), db, uid, 1); err != nil {
		t.Fatalf("fresh cache should bridge db error, got %v", err)
	}
	// An expired snapshot must not bridge the error.
	accountStateCache.Store(uid, cachedAccountState{state: accountState{Status: "active", TokenVersion: 1}, at: time.Now().Add(-authStateCacheTTL - time.Second)})
	if err := EnsureAccountActive(context.Background(), db, uid, 1); err == nil || err.Error() != ErrAuthStateUnavailable {
		t.Fatalf("want %s, got %v", ErrAuthStateUnavailable, err)
	}
}

// A cached revoked/disabled snapshot must keep rejecting during the outage.
func TestEnsureAccountActiveCachedDisabledStillRejects(t *testing.T) {
	db := newTenantStateTestDB(t)
	if err := db.Migrator().DropTable(&admin.AdminUser{}); err != nil {
		t.Fatal(err)
	}
	uid := uuid.New()
	accountStateCache.Store(uid, cachedAccountState{state: accountState{Status: "disabled", TokenVersion: 1}, at: time.Now()})
	if err := EnsureAccountActive(context.Background(), db, uid, 1); err == nil || err.Error() != ErrUserDisabled {
		t.Fatalf("want %s, got %v", ErrUserDisabled, err)
	}
}

func TestEnsureTenantActiveFailClosed(t *testing.T) {
	db := newTenantStateTestDB(t)
	if err := db.Exec(`INSERT INTO tenants (id, name, status) VALUES (9001, 't-fc', 'active')`).Error; err != nil {
		t.Fatal(err)
	}
	// Warm cache.
	if err := EnsureTenantActive(context.Background(), db, 9001); err != nil {
		t.Fatalf("warm read: %v", err)
	}
	if err := db.Exec(`DROP TABLE tenants`).Error; err != nil {
		t.Fatal(err)
	}
	// Fresh cache bridges the error.
	if err := EnsureTenantActive(context.Background(), db, 9001); err != nil {
		t.Fatalf("fresh cache should bridge db error, got %v", err)
	}
	// Unknown tenant with no cache fails closed.
	if err := EnsureTenantActive(context.Background(), db, 9002); err == nil || err.Error() != ErrAuthStateUnavailable {
		t.Fatalf("want %s, got %v", ErrAuthStateUnavailable, err)
	}
	if !tenantDisabled(context.Background(), db, 9002) {
		t.Fatal("tenantDisabled must fail closed on unavailable state")
	}
}
