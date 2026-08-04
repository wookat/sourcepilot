package adminperm

import (
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

// PlatformTenantID is the tenant that owns platform-wide governance data:
// tenant provisioning, database backups/restores, releases and DR drills, and
// the shared AI prompt catalog.
const PlatformTenantID int64 = 0

// IsPlatformTenant reports whether the request runs as the platform tenant.
func IsPlatformTenant(c *gin.Context) bool {
	tid, err := TenantIDFromGin(c)
	return err == nil && tid == PlatformTenantID
}

// RequirePlatformAdmin allows only admins of the platform tenant. Business
// tenants must never reach platform-wide operations: a tenant admin holding an
// ops permission would otherwise be able to dump or restore the whole database
// (all tenants) or rewrite the shared prompt catalog.
func RequirePlatformAdmin(c *gin.Context, db *gorm.DB) bool {
	if !IsPlatformTenant(c) {
		DenyPermission(c)
		return false
	}
	p, err := LoadPrincipal(c, db)
	if err != nil || p == nil || !p.IsAdmin() {
		DenyPermission(c)
		return false
	}
	return true
}

// RequirePlatformAdminMW is RequirePlatformAdmin as route middleware.
func RequirePlatformAdminMW(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		if !RequirePlatformAdmin(c, db) {
			c.Abort()
			return
		}
		c.Next()
	}
}
