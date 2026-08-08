package mcptoken_test

import (
	"context"
	"fmt"
	"strings"
	"testing"

	"github.com/glebarez/sqlite"
	"github.com/google/uuid"
	"github.com/trademind-ai/trademind/backend/internal/modules/mcptoken"
	"gorm.io/gorm"
)

func openTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	dsn := fmt.Sprintf("file:mcptoken_%s?mode=memory&cache=shared&_pragma=busy_timeout(5000)", uuid.New().String())
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Skipf("sqlite unavailable: %v", err)
	}
	sqlDB, _ := db.DB()
	sqlDB.SetMaxOpenConns(1)
	if err := db.AutoMigrate(&mcptoken.Token{}); err != nil {
		t.Fatal(err)
	}
	// Token authentication also checks the tenant is not disabled.
	if err := db.Exec(`CREATE TABLE tenants (id integer primary key, status text, deleted_at datetime)`).Error; err != nil {
		t.Fatal(err)
	}
	return db
}

func TestCreateStoresHashOnly(t *testing.T) {
	svc := &mcptoken.Service{DB: openTestDB(t)}
	res, err := svc.Create(context.Background(), 1, "claude-desktop", "", nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(res.Plaintext, mcptoken.TokenPrefix) {
		t.Fatalf("plaintext %q missing prefix", res.Plaintext)
	}
	if len(res.Plaintext) != len(mcptoken.TokenPrefix)+64 {
		t.Fatalf("unexpected plaintext length %d", len(res.Plaintext))
	}
	if res.Token.TokenHash == res.Plaintext {
		t.Fatal("plaintext stored verbatim")
	}
	if res.Token.TokenHash != mcptoken.HashToken(res.Plaintext) {
		t.Fatal("stored hash mismatch")
	}
	var count int64
	if err := svc.DB.Model(&mcptoken.Token{}).Where("token_hash = ?", res.Plaintext).Count(&count).Error; err != nil {
		t.Fatal(err)
	}
	if count != 0 {
		t.Fatal("plaintext found in database")
	}
	masked := res.Token.Masked()
	if strings.Contains(masked, res.Plaintext[len(mcptoken.TokenPrefix)+4:len(res.Plaintext)-4]) {
		t.Fatalf("masked view %q leaks secret", masked)
	}
}

func TestCreateValidation(t *testing.T) {
	svc := &mcptoken.Service{DB: openTestDB(t)}
	if _, err := svc.Create(context.Background(), 1, "  ", "", nil, nil); err == nil {
		t.Fatal("expected error for empty name")
	}
	if _, err := svc.Create(context.Background(), -1, "x", "", nil, nil); err == nil {
		t.Fatal("expected error for negative tenant")
	}
	// Tenant 0 stays valid for legacy single-tenant data.
	if _, err := svc.Create(context.Background(), 0, "legacy", "", nil, nil); err != nil {
		t.Fatalf("tenant 0 should be allowed: %v", err)
	}
}

func TestAuthenticate(t *testing.T) {
	svc := &mcptoken.Service{DB: openTestDB(t)}
	res, err := svc.Create(context.Background(), 7, "inspector", "", nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	tok, err := svc.Authenticate(context.Background(), res.Plaintext)
	if err != nil {
		t.Fatal(err)
	}
	if tok.TenantID != 7 {
		t.Fatalf("tenant = %d, want 7", tok.TenantID)
	}

	for _, bad := range []string{
		"",
		"sp_mcp_ro_short",
		strings.Repeat("a", len(res.Plaintext)),
		res.Plaintext[:len(res.Plaintext)-1] + "0",
	} {
		if bad == res.Plaintext {
			continue
		}
		if _, err := svc.Authenticate(context.Background(), bad); err == nil {
			t.Fatalf("expected failure for %q", bad)
		}
	}
}

func TestRevokeBlocksAuth(t *testing.T) {
	svc := &mcptoken.Service{DB: openTestDB(t)}
	res, err := svc.Create(context.Background(), 1, "to-revoke", "", nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := svc.Revoke(context.Background(), 1, res.Token.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.Authenticate(context.Background(), res.Plaintext); err == nil {
		t.Fatal("revoked token still authenticates")
	}
	// Idempotent revoke.
	if _, err := svc.Revoke(context.Background(), 1, res.Token.ID); err != nil {
		t.Fatalf("second revoke: %v", err)
	}
}

func TestTenantIsolation(t *testing.T) {
	svc := &mcptoken.Service{DB: openTestDB(t)}
	a, err := svc.Create(context.Background(), 1, "tenant-a", "", nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := svc.Create(context.Background(), 2, "tenant-b", "", nil, nil); err != nil {
		t.Fatal(err)
	}
	rows, err := svc.List(context.Background(), 1)
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 1 || rows[0].Name != "tenant-a" {
		t.Fatalf("tenant 1 list leaked rows: %+v", rows)
	}
	// Cross-tenant revoke must fail closed.
	if _, err := svc.Revoke(context.Background(), 2, a.Token.ID); err == nil {
		t.Fatal("cross-tenant revoke succeeded")
	}
}

func TestCreateCapTenantZero(t *testing.T) {
	// Tenant 0 (platform bootstrap admin, outside the tenant system) follows
	// the same per-tenant cap: its token count is scoped to tenant_id = 0 and
	// must neither bypass the cap nor consume any other tenant's quota.
	svc := &mcptoken.Service{DB: openTestDB(t)}
	for i := 0; i < mcptoken.MaxActiveTokensPerTenant; i++ {
		if _, err := svc.Create(context.Background(), 0, fmt.Sprintf("t0-%d", i), "", nil, nil); err != nil {
			t.Fatalf("create %d for tenant 0: %v", i, err)
		}
	}
	if _, err := svc.Create(context.Background(), 0, "t0-over", "", nil, nil); err != mcptoken.ErrTooManyTokens {
		t.Fatalf("tenant 0 over-cap create: got %v, want ErrTooManyTokens", err)
	}
	// A regular tenant is unaffected by tenant 0's full quota.
	if _, err := svc.Create(context.Background(), 1, "tenant-1-ok", "", nil, nil); err != nil {
		t.Fatalf("tenant 1 create blocked by tenant 0 quota: %v", err)
	}
	rows, err := svc.List(context.Background(), 0)
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != mcptoken.MaxActiveTokensPerTenant {
		t.Fatalf("tenant 0 list: got %d rows, want %d", len(rows), mcptoken.MaxActiveTokensPerTenant)
	}
}
