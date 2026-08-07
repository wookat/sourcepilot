package adminperm

import (
	"errors"

	"github.com/gin-gonic/gin"
	"github.com/trademind-ai/trademind/backend/internal/pkg/response"
	"gorm.io/gorm"
)

// DenyPermission responds with unified permission denied (403).
func DenyPermission(c *gin.Context) {
	response.Fail(c, 403, response.CodePermissionDenied, "当前账号无权限执行此操作")
}

// DenyReadonly responds when readonly attempts a write.
func DenyReadonly(c *gin.Context) {
	response.Fail(c, 403, response.CodeReadonlyForbidden, "只读账号不可执行写操作")
}

// DenyStorePermission responds when store scope blocks access.
func DenyStorePermission(c *gin.Context) {
	response.Fail(c, 403, response.CodeStorePermissionDenied, "店铺无操作权限")
}

// FailStoreWriteScope maps store write scope violations onto the unified
// envelope: missing / cross-tenant / invisible stores → 404 (no existence
// leak), view-only stores → 403/40303. It reports whether err was consumed.
func FailStoreWriteScope(c *gin.Context, err error) bool {
	if errors.Is(err, gorm.ErrRecordNotFound) {
		response.Fail(c, 404, response.CodeNotFound, "记录不存在或已被删除")
		return true
	}
	if errors.Is(err, ErrStoreNotOperable) {
		response.Fail(c, 403, response.CodeStorePermissionDenied, "店铺无操作权限")
		return true
	}
	return false
}

// DenySettings responds when settings management is required.
func DenySettings(c *gin.Context) {
	response.Fail(c, 403, response.CodeSettingsPermissionRequired, "仅管理员可管理系统配置")
}

// DenyUserManage responds when user management is required.
func DenyUserManage(c *gin.Context) {
	response.Fail(c, 403, response.CodeUserManagePermissionRequired, "仅管理员可管理用户与权限")
}

// RequirePermission checks permission key; returns false after writing response.
func RequirePermission(c *gin.Context, db *gorm.DB, perm string) bool {
	p, err := LoadPrincipal(c, db)
	if err != nil {
		response.HandleError(c, err)
		return false
	}
	if p.Can(perm) {
		return true
	}
	switch perm {
	case PermSettingsManage:
		DenySettings(c)
	case PermUserManage:
		DenyUserManage(c)
	default:
		DenyPermission(c)
	}
	return false
}

// RequireWrite checks readonly + permission; returns false after writing response.
func RequireWrite(c *gin.Context, db *gorm.DB, perm string) bool {
	p, err := LoadPrincipal(c, db)
	if err != nil {
		response.HandleError(c, err)
		return false
	}
	if p.IsReadonly() {
		DenyReadonly(c)
		return false
	}
	if !p.Can(perm) {
		switch perm {
		case PermSettingsManage:
			DenySettings(c)
		case PermUserManage:
			DenyUserManage(c)
		default:
			DenyPermission(c)
		}
		return false
	}
	return true
}
