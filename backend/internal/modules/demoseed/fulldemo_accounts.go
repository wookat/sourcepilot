package demoseed

import (
	"errors"
	"fmt"
	"strings"

	"github.com/trademind-ai/trademind/backend/internal/modules/admin"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"
)

// demoAccountSpec is one out-of-the-box RBAC demo account. Emails and
// passwords must stay in sync with scripts/seed-demo-permissions.ps1 and
// docs/DEMO_SEEDING_GUIDE.md.
type demoAccountSpec struct {
	email       string
	password    string
	displayName string
	role        string
}

// DemoAccounts are the three demo RBAC accounts guaranteed by seed.
var DemoAccounts = []demoAccountSpec{
	{email: "demo_admin@trademind.local", password: "DemoAdmin123!", displayName: "Demo Admin", role: "admin"},
	{email: "demo_operator@trademind.local", password: "DemoOperator123!", displayName: "Demo Operator", role: "operator"},
	{email: "demo_readonly@trademind.local", password: "DemoReadonly123!", displayName: "Demo Readonly", role: "readonly"},
}

// ensureDemoAccounts creates the demo RBAC accounts if missing and resets a
// drifted password hash back to the documented value (bumping TokenVersion so
// stale sessions die). It never touches non-demo accounts and only runs in
// non-production (enforced by the seeder guard).
func (s *FullDemoSeeder) ensureDemoAccounts(tx *gorm.DB, count func(string, int64)) error {
	for _, spec := range DemoAccounts {
		count("demo_accounts_ensured", 1)
		var u admin.AdminUser
		err := tx.First(&u, "tenant_id = ? AND lower(email) = ?", s.TenantID, strings.ToLower(spec.email)).Error
		switch {
		case errors.Is(err, gorm.ErrRecordNotFound):
			hash, herr := bcrypt.GenerateFromPassword([]byte(spec.password), bcrypt.DefaultCost)
			if herr != nil {
				return fmt.Errorf("demoseed: hash demo password: %w", herr)
			}
			u = admin.AdminUser{
				TenantID:     s.TenantID,
				Username:     admin.NewInternalUsername(),
				Email:        spec.email,
				PasswordHash: string(hash),
				DisplayName:  spec.displayName,
				Role:         spec.role,
				Status:       "active",
			}
			if cerr := tx.Create(&u).Error; cerr != nil {
				return fmt.Errorf("demoseed: create demo account %s: %w", spec.email, cerr)
			}
		case err != nil:
			return fmt.Errorf("demoseed: lookup demo account %s: %w", spec.email, err)
		default:
			if admin.CheckPassword(u.PasswordHash, spec.password) == nil {
				continue
			}
			hash, herr := bcrypt.GenerateFromPassword([]byte(spec.password), bcrypt.DefaultCost)
			if herr != nil {
				return fmt.Errorf("demoseed: hash demo password: %w", herr)
			}
			updates := map[string]any{
				"password_hash": string(hash),
				"token_version": gorm.Expr("token_version + 1"),
			}
			if uerr := tx.Model(&admin.AdminUser{}).Where("id = ?", u.ID).Updates(updates).Error; uerr != nil {
				return fmt.Errorf("demoseed: reset demo account password %s: %w", spec.email, uerr)
			}
		}
	}
	return nil
}
