package auth

import (
	"context"
	"errors"
	"strings"

	"gorm.io/gorm"
)

// tenantDisabled reports whether a tenant has been disabled by a platform
// administrator. Tenant 0 (the platform tenant) can never be disabled, and
// tenants without a row in the tenants table (legacy) count as active.
// Errors fail open so a transient DB issue never locks everyone out.
func tenantDisabled(ctx context.Context, db *gorm.DB, tenantID int64) bool {
	if db == nil || tenantID <= 0 {
		return false
	}
	var statuses []string
	if err := db.WithContext(ctx).Table("tenants").
		Where("id = ? AND deleted_at IS NULL", tenantID).
		Limit(1).Pluck("status", &statuses).Error; err != nil {
		return false
	}
	return len(statuses) == 1 && strings.EqualFold(strings.TrimSpace(statuses[0]), "disabled")
}

// EnsureTenantActive returns ErrTenantDisabled when the tenant has been
// disabled by a platform administrator.
func EnsureTenantActive(ctx context.Context, db *gorm.DB, tenantID int64) error {
	if tenantDisabled(ctx, db, tenantID) {
		return errors.New(ErrTenantDisabled)
	}
	return nil
}
