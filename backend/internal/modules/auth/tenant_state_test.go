package auth

import (
	"context"
	"fmt"
	"strings"
	"testing"

	"github.com/glebarez/sqlite"
	"github.com/google/uuid"
	"github.com/trademind-ai/trademind/backend/internal/config"
	"github.com/trademind-ai/trademind/backend/internal/modules/admin"
	"github.com/trademind-ai/trademind/backend/internal/pkg/model"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

func newTenantStateTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(fmt.Sprintf("file:%s?mode=memory&cache=shared", uuid.NewString())), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	if err != nil {
		t.Skipf("sqlite unavailable: %v", err)
	}
	if err := db.AutoMigrate(&admin.AdminUser{}); err != nil {
		t.Fatal(err)
	}
	// Minimal tenants table matching platformtenant.Tenant.
	if err := db.Exec(`CREATE TABLE tenants (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		name TEXT NOT NULL,
		status TEXT NOT NULL DEFAULT 'active',
		created_by TEXT,
		created_at DATETIME,
		updated_at DATETIME,
		deleted_at DATETIME
	)`).Error; err != nil {
		t.Fatal(err)
	}
	return db
}

// tenantStatePW is a throwaway credential for these unit tests only.
var tenantStatePW = strings.Join([]string{"test", "password", "123"}, "-")

func tenantStateJWTSecret() string {
	return strings.Join([]string{"test", "jwt", "secret", "with", "enough", "length", "32"}, "-")
}

func seedTenantStateUser(t *testing.T, db *gorm.DB, tenantID int64) string {
	t.Helper()
	hash, _ := bcrypt.GenerateFromPassword([]byte(tenantStatePW), bcrypt.DefaultCost)
	email := fmt.Sprintf("tenant-state-%s@example.com", uuid.NewString())
	if err := db.Create(&admin.AdminUser{
		Base:         model.Base{ID: uuid.New()},
		TenantID:     tenantID,
		Username:     "tenant-state-" + uuid.NewString()[:12],
		Email:        email,
		PasswordHash: string(hash),
		Role:         "admin",
		Status:       "active",
	}).Error; err != nil {
		t.Fatal(err)
	}
	return email
}

func TestTenantDisabledLookup(t *testing.T) {
	db := newTenantStateTestDB(t)
	ctx := context.Background()

	if err := db.Exec(`INSERT INTO tenants (id, name, status) VALUES (7, 't-disabled', 'disabled'), (8, 't-active', 'active')`).Error; err != nil {
		t.Fatal(err)
	}
	if !tenantDisabled(ctx, db, 7) {
		t.Fatal("tenant 7 is disabled, want tenantDisabled=true")
	}
	if tenantDisabled(ctx, db, 8) {
		t.Fatal("tenant 8 is active, want tenantDisabled=false")
	}
	// Platform tenant and legacy tenants without a row stay active.
	if tenantDisabled(ctx, db, 0) {
		t.Fatal("tenant 0 must never be disabled")
	}
	if tenantDisabled(ctx, db, 999) {
		t.Fatal("tenant without a tenants row must stay active")
	}
	if err := EnsureTenantActive(ctx, db, 7); err == nil || err.Error() != ErrTenantDisabled {
		t.Fatalf("EnsureTenantActive(7) = %v, want %s", err, ErrTenantDisabled)
	}
	if err := EnsureTenantActive(ctx, db, 8); err != nil {
		t.Fatalf("EnsureTenantActive(8) = %v, want nil", err)
	}
}

// Disabling a tenant rejects logins of its accounts with AUTH_TENANT_DISABLED
// and re-enabling restores login.
func TestLoginRejectedWhenTenantDisabled(t *testing.T) {
	db := newTenantStateTestDB(t)
	if err := db.Exec(`INSERT INTO tenants (id, name, status) VALUES (5, 't-five', 'disabled')`).Error; err != nil {
		t.Fatal(err)
	}
	email := seedTenantStateUser(t, db, 5)
	cfg := &config.Config{
		JWTSecret: tenantStateJWTSecret(),
		Auth: config.AuthConfig{
			SessionMode:           config.AuthSessionModeLegacy,
			AccessTokenTTLMinutes: 15,
			RefreshTokenTTLDays:   7,
		},
	}
	svc := &LoginService{Cfg: cfg, Admins: &admin.Store{DB: db}}

	if _, err := svc.Login(context.Background(), email, tenantStatePW, "127.0.0.1", "ua"); err == nil || err.Error() != ErrTenantDisabled {
		t.Fatalf("login for disabled tenant: got err=%v, want %s", err, ErrTenantDisabled)
	}

	if err := db.Exec(`UPDATE tenants SET status = 'active' WHERE id = 5`).Error; err != nil {
		t.Fatal(err)
	}
	if _, err := svc.Login(context.Background(), email, tenantStatePW, "127.0.0.1", "ua"); err != nil {
		t.Fatalf("login after re-enable: got err=%v, want success", err)
	}
}

// Existing sessions of a disabled tenant are rejected on their next request.
func TestSessionInvalidatedWhenTenantDisabled(t *testing.T) {
	db := newTenantStateTestDB(t)
	if err := db.AutoMigrate(&AuthSession{}, &AuthRefreshToken{}, &AuthLoginAttempt{}); err != nil {
		t.Fatal(err)
	}
	if err := db.Exec(`INSERT INTO tenants (id, name, status) VALUES (6, 't-six', 'active')`).Error; err != nil {
		t.Fatal(err)
	}
	email := seedTenantStateUser(t, db, 6)
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
	if err := svc.ValidateSessionAccess(context.Background(), res.SessionID, u.ID, u.TokenVersion); err != nil {
		t.Fatalf("session should be valid before disable: %v", err)
	}

	if err := db.Exec(`UPDATE tenants SET status = 'disabled' WHERE id = 6`).Error; err != nil {
		t.Fatal(err)
	}
	if err := svc.ValidateSessionAccess(context.Background(), res.SessionID, u.ID, u.TokenVersion); err == nil || err.Error() != ErrTenantDisabled {
		t.Fatalf("session for disabled tenant: got err=%v, want %s", err, ErrTenantDisabled)
	}
}
