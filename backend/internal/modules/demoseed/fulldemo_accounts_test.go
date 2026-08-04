package demoseed

import (
	"context"
	"testing"

	"github.com/trademind-ai/trademind/backend/internal/modules/admin"
	"golang.org/x/crypto/bcrypt"
)

// Seed must create the three documented demo RBAC accounts when they are
// missing, and reset a drifted password back to the documented value.
func TestSeedEnsuresDemoAccounts(t *testing.T) {
	db := openFullDemoTestDB(t)
	if db == nil {
		return
	}

	s := &FullDemoSeeder{DB: db, TenantID: 7, AppEnv: "development"}
	if _, err := s.Seed(context.Background()); err != nil {
		t.Fatalf("seed: %v", err)
	}

	for _, spec := range DemoAccounts {
		var u admin.AdminUser
		if err := db.First(&u, "email = ?", spec.email).Error; err != nil {
			t.Fatalf("demo account %s not created: %v", spec.email, err)
		}
		if u.Role != spec.role {
			t.Fatalf("demo account %s role = %q, want %q", spec.email, u.Role, spec.role)
		}
		if admin.CheckPassword(u.PasswordHash, spec.password) != nil {
			t.Fatalf("demo account %s password does not match documented value", spec.email)
		}
	}

	// Drift the operator password, reseed, expect it reset + token bump.
	var op admin.AdminUser
	if err := db.First(&op, "email = ?", "demo_operator@trademind.local").Error; err != nil {
		t.Fatal(err)
	}
	drifted, err := bcrypt.GenerateFromPassword([]byte("SomethingElse1!"), bcrypt.DefaultCost)
	if err != nil {
		t.Fatal(err)
	}
	if err := db.Model(&admin.AdminUser{}).Where("id = ?", op.ID).
		Update("password_hash", string(drifted)).Error; err != nil {
		t.Fatal(err)
	}

	if _, err := s.Seed(context.Background()); err != nil {
		t.Fatalf("reseed: %v", err)
	}
	var op2 admin.AdminUser
	if err := db.First(&op2, "id = ?", op.ID).Error; err != nil {
		t.Fatal(err)
	}
	if admin.CheckPassword(op2.PasswordHash, "DemoOperator123!") != nil {
		t.Fatal("drifted demo operator password was not reset by reseed")
	}
	if op2.TokenVersion != op.TokenVersion+1 {
		t.Fatalf("token_version = %d, want %d", op2.TokenVersion, op.TokenVersion+1)
	}
}
