package auth

import (
	"context"
	"testing"

	"github.com/trademind-ai/trademind/backend/internal/modules/admin"
	"github.com/trademind-ai/trademind/backend/internal/pkg/model"
)

// Self-registration must provision a dedicated tenant per account: a new user
// must never land in the platform tenant (tenant 0), and two registrations
// must not share a tenant.
func TestCreateRegistrationUserProvisionsIsolatedTenant(t *testing.T) {
	db := newTenantStateTestDB(t)

	u1 := admin.AdminUser{
		Base:         model.Base{},
		Username:     admin.NewInternalUsername(),
		Email:        "tenant-b@example.com",
		DisplayName:  "tenant-b@example.com",
		PasswordHash: "x",
		Role:         "admin",
		Status:       "active",
	}
	if err := createRegistrationUser(context.Background(), db, &u1); err != nil {
		t.Fatalf("createRegistrationUser: %v", err)
	}
	if u1.TenantID == 0 {
		t.Fatalf("registered user must not join platform tenant 0")
	}

	var stored admin.AdminUser
	if err := db.Where("email = ?", u1.Email).First(&stored).Error; err != nil {
		t.Fatalf("load user: %v", err)
	}
	if stored.TenantID != u1.TenantID {
		t.Fatalf("stored tenant id %d != %d", stored.TenantID, u1.TenantID)
	}

	var count int64
	if err := db.Table("tenants").Where("id = ? AND status = 'active'", u1.TenantID).Count(&count).Error; err != nil {
		t.Fatalf("count tenants: %v", err)
	}
	if count != 1 {
		t.Fatalf("expected 1 active tenant row for id %d, got %d", u1.TenantID, count)
	}

	u2 := admin.AdminUser{
		Base:         model.Base{},
		Username:     admin.NewInternalUsername(),
		Email:        "tenant-c@example.com",
		DisplayName:  "tenant-c@example.com",
		PasswordHash: "x",
		Role:         "admin",
		Status:       "active",
	}
	if err := createRegistrationUser(context.Background(), db, &u2); err != nil {
		t.Fatalf("createRegistrationUser second: %v", err)
	}
	if u2.TenantID == u1.TenantID {
		t.Fatalf("two registrations must not share tenant %d", u1.TenantID)
	}
}
