package demoseed

import (
	"context"
	"testing"

	"github.com/trademind-ai/trademind/backend/internal/modules/admin"
	"github.com/trademind-ai/trademind/backend/internal/modules/order"
	"github.com/trademind-ai/trademind/backend/internal/modules/platformtenant"
	"github.com/trademind-ai/trademind/backend/internal/modules/shop"
	"golang.org/x/crypto/bcrypt"
)

// Round 128: second demo tenant (isolated business tenant + admin account +
// small shop/order/rule dataset) seeds, is tenant-isolated, reruns
// idempotently, and cleanup / verify leave zero residue.
func TestFullDemoSeedSecondTenant(t *testing.T) {
	db := openFullDemoTestDB(t)
	s := &FullDemoSeeder{DB: db, TenantID: 1, AppEnv: "development"}

	if _, err := s.Seed(context.Background()); err != nil {
		t.Fatal(err)
	}

	var tenant platformtenant.Tenant
	if err := db.First(&tenant, "name = ?", DemoTenant2Name).Error; err != nil {
		t.Fatalf("expected second demo tenant: %v", err)
	}
	if tenant.ID == s.TenantID || tenant.ID <= 0 {
		t.Fatalf("second tenant id must be a distinct positive tenant, got %d", tenant.ID)
	}

	var adminUser admin.AdminUser
	if err := db.First(&adminUser, "tenant_id = ? AND email = ?", tenant.ID, DemoTenant2AdminEmail).Error; err != nil {
		t.Fatalf("expected second tenant admin account: %v", err)
	}
	if adminUser.Role != "admin" || adminUser.Status != "active" {
		t.Fatalf("unexpected second tenant admin: %+v", adminUser)
	}
	if err := bcrypt.CompareHashAndPassword([]byte(adminUser.PasswordHash), []byte(DemoTenant2AdminPassword)); err != nil {
		t.Fatalf("second tenant admin password mismatch: %v", err)
	}

	// 正向样本：第二租户下有自己的店铺 / 订单 / 规则。
	var t2Shops, t2Orders, t2Rules int64
	db.Model(&shop.Shop{}).Where("tenant_id = ?", tenant.ID).Count(&t2Shops)
	db.Model(&order.Order{}).Where("tenant_id = ?", tenant.ID).Count(&t2Orders)
	db.Model(&order.OrderAutomationRule{}).Where("tenant_id = ?", tenant.ID).Count(&t2Rules)
	if t2Shops == 0 || t2Orders == 0 || t2Rules == 0 {
		t.Fatalf("second tenant business data missing: shops=%d orders=%d rules=%d", t2Shops, t2Orders, t2Rules)
	}

	// 负向样本：第二租户订单不落在第一租户，双向不串。
	var crossOrders int64
	db.Model(&order.Order{}).Where("tenant_id = ? AND order_no LIKE ?", s.TenantID, "DEMO-T2-%").Count(&crossOrders)
	if crossOrders != 0 {
		t.Fatalf("second tenant orders leaked into tenant %d: %d", s.TenantID, crossOrders)
	}
	var t1InT2 int64
	db.Model(&order.Order{}).Where("tenant_id = ? AND order_no NOT LIKE ?", tenant.ID, "DEMO-T2-%").Count(&t1InT2)
	if t1InT2 != 0 {
		t.Fatalf("tenant-1 orders leaked into second tenant: %d", t1InT2)
	}

	// 幂等重跑：tenant 行不重复、admin 账号唯一。
	if _, err := s.Seed(context.Background()); err != nil {
		t.Fatal(err)
	}
	var tenantRows, adminRows int64
	db.Model(&platformtenant.Tenant{}).Where("name = ?", DemoTenant2Name).Count(&tenantRows)
	db.Model(&admin.AdminUser{}).Where("email = ?", DemoTenant2AdminEmail).Count(&adminRows)
	if tenantRows != 1 || adminRows != 1 {
		t.Fatalf("reseed not idempotent: tenants=%d admins=%d", tenantRows, adminRows)
	}

	if _, err := s.Cleanup(context.Background()); err != nil {
		t.Fatal(err)
	}
	verify, err := s.VerifyClean(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	for _, table := range []string{"tenants", "admin_users", "shops", "orders", "order_automation_rules", "shipping_rules"} {
		if verify.Counts[table] != 0 {
			t.Fatalf("expected zero %s residue after cleanup, got %d", table, verify.Counts[table])
		}
	}
	var residualTenants, residualAdmins int64
	db.Unscoped().Model(&platformtenant.Tenant{}).Where("name = ?", DemoTenant2Name).Count(&residualTenants)
	db.Unscoped().Model(&admin.AdminUser{}).Where("email = ?", DemoTenant2AdminEmail).Count(&residualAdmins)
	if residualTenants != 0 || residualAdmins != 0 {
		t.Fatalf("second tenant residue after cleanup: tenants=%d admins=%d", residualTenants, residualAdmins)
	}
}

// A custom -prefix (e.g. QA-) clean/verify must not touch the DEMO- second
// tenant, and cleaning with the default prefix afterwards removes it.
func TestSecondTenantCustomPrefixCompat(t *testing.T) {
	db := openFullDemoTestDB(t)
	s := &FullDemoSeeder{DB: db, TenantID: 1, AppEnv: "development"}
	if _, err := s.Seed(context.Background()); err != nil {
		t.Fatal(err)
	}

	qa := &FullDemoSeeder{DB: db, TenantID: 1, AppEnv: "development", Prefix: "QA-"}
	if _, err := qa.Cleanup(context.Background()); err != nil {
		t.Fatal(err)
	}
	var tenants int64
	db.Model(&platformtenant.Tenant{}).Where("name = ?", DemoTenant2Name).Count(&tenants)
	if tenants != 1 {
		t.Fatalf("QA- cleanup must not remove DEMO- second tenant, got %d rows", tenants)
	}
	verify, err := qa.VerifyClean(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if verify.Counts["tenants"] != 0 {
		t.Fatalf("QA- verify must not count DEMO- tenants, got %d", verify.Counts["tenants"])
	}

	if _, err := s.Cleanup(context.Background()); err != nil {
		t.Fatal(err)
	}
	db.Unscoped().Model(&platformtenant.Tenant{}).Where("name = ?", DemoTenant2Name).Count(&tenants)
	if tenants != 0 {
		t.Fatalf("DEMO- cleanup left second tenant residue: %d", tenants)
	}
}
