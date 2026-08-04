package auth

import (
	"context"
	"errors"
	"strings"
	"sync"
	"time"

	"gorm.io/gorm"
)

type cachedTenantState struct {
	disabled bool
	at       time.Time
}

var tenantStateCache sync.Map // int64 -> cachedTenantState

// tenantState reports whether a tenant has been disabled by a platform
// administrator. Tenant 0 (the platform tenant) can never be disabled, and
// tenants without a row in the tenants table (legacy) count as active.
// Transient database errors fall back to the last snapshot read within
// authStateCacheTTL; with no fresh snapshot the lookup fails closed.
func tenantState(ctx context.Context, db *gorm.DB, tenantID int64) (disabled bool, err error) {
	disabled, _, err = tenantStateDetailed(ctx, db, tenantID)
	return disabled, err
}

// tenantStateDetailed additionally reports whether the answer was bridged
// from the last snapshot because the database was unreachable.
func tenantStateDetailed(ctx context.Context, db *gorm.DB, tenantID int64) (disabled bool, bridged bool, err error) {
	if db == nil || tenantID <= 0 {
		return false, false, nil
	}
	var statuses []string
	if dbErr := db.WithContext(ctx).Table("tenants").
		Where("id = ? AND deleted_at IS NULL", tenantID).
		Limit(1).Pluck("status", &statuses).Error; dbErr != nil {
		if cached, ok := tenantStateCache.Load(tenantID); ok {
			c := cached.(cachedTenantState)
			if time.Since(c.at) <= authStateCacheTTL {
				return c.disabled, true, nil
			}
		}
		return false, true, errors.New(ErrAuthStateUnavailable)
	}
	disabled = len(statuses) == 1 && strings.EqualFold(strings.TrimSpace(statuses[0]), "disabled")
	tenantStateCache.Store(tenantID, cachedTenantState{disabled: disabled, at: time.Now()})
	return disabled, false, nil
}

// tenantDisabled reports whether the tenant is disabled, counting an
// unavailable trusted state as disabled (fail closed).
func tenantDisabled(ctx context.Context, db *gorm.DB, tenantID int64) bool {
	disabled, err := tenantState(ctx, db, tenantID)
	return disabled || err != nil
}

// EnsureTenantActive returns ErrTenantDisabled when the tenant has been
// disabled by a platform administrator, or ErrAuthStateUnavailable when the
// trusted state cannot be established (database error with no fresh cache).
func EnsureTenantActive(ctx context.Context, db *gorm.DB, tenantID int64) error {
	_, err := EnsureTenantActiveDetailed(ctx, db, tenantID)
	return err
}

// EnsureTenantActiveDetailed reports, in addition to the validation result,
// whether the decision was bridged from the last snapshot because the
// database was unreachable.
func EnsureTenantActiveDetailed(ctx context.Context, db *gorm.DB, tenantID int64) (bridged bool, err error) {
	disabled, bridged, err := tenantStateDetailed(ctx, db, tenantID)
	if err != nil {
		return bridged, err
	}
	if disabled {
		return bridged, errors.New(ErrTenantDisabled)
	}
	return bridged, nil
}
