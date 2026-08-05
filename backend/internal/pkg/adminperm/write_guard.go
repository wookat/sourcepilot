package adminperm

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/trademind-ai/trademind/backend/internal/pkg/response"
	"gorm.io/gorm"
)

// readonlyWriteAllowlist lists mutating-method routes that readonly principals
// may still call. Only two categories qualify:
//  1. self-service session management (any authenticated account may manage
//     its own sessions);
//  2. pure read-like computations that use POST for payload size (checks,
//     previews, estimates, validations) and never persist business data.
//
// Keys are "METHOD <gin full route path>". Keep this list in sync with
// internal/securitytests/permmatrix/matrix.json (readonly="allow" write routes).
var readonlyWriteAllowlist = map[string]bool{
	"POST /api/v1/auth/logout":                 true,
	"POST /api/v1/auth/logout-all":             true,
	"POST /api/v1/auth/sessions/revoke-others": true,
	"DELETE /api/v1/auth/sessions/:id":         true,

	"POST /api/v1/pricing/calculate":                                  true,
	"POST /api/v1/products/readiness/batch":                           true,
	"POST /api/v1/inventory/stock-settings/batch-preview":             true,
	"POST /api/v1/procurement/cost-estimates/batch":                   true,
	"POST /api/v1/product-publish/batch-targets/check":                true,
	"POST /api/v1/products/:id/publish-targets/check":                 true,
	"POST /api/v1/products/ai-images/batches/check":                   true,
	"POST /api/v1/products/ai-text/batches/check":                     true,
	"POST /api/v1/products/banned-words/check-batch":                  true,
	"POST /api/v1/shipping-rules/recommend":                           true,
	"POST /api/v1/orders/shipping-recommendations":                    true,
	"POST /api/v1/products/:id/platform-configs/douyin_shop/validate": true,
}

// ReadonlyWriteAllowed reports whether a mutating route is allowlisted for
// readonly principals (exported for the permission matrix contract tests).
func ReadonlyWriteAllowed(method, fullPath string) bool {
	return readonlyWriteAllowlist[method+" "+fullPath]
}

// ReadonlyWriteGuard is a group-level middleware that rejects readonly
// principals on every mutating-method route (POST/PUT/PATCH/DELETE) unless the
// route is explicitly allowlisted. It provides a fail-closed default so new
// write endpoints are protected from readonly access even when a handler-level
// guard is forgotten. Handler/service level permission and scope checks remain
// authoritative for finer-grained rules.
func ReadonlyWriteGuard(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		switch c.Request.Method {
		case http.MethodGet, http.MethodHead, http.MethodOptions:
			c.Next()
			return
		}
		if ReadonlyWriteAllowed(c.Request.Method, c.FullPath()) {
			c.Next()
			return
		}
		p, _ := LoadPrincipal(c, db)
		if p == nil || p.IsReadonly() {
			response.Fail(c, 403, response.CodeForbidden, "当前账号为只读权限，无法执行此操作")
			c.Abort()
			return
		}
		c.Next()
	}
}
