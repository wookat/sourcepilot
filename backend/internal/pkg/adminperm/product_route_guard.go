package adminperm

import (
	"errors"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/trademind-ai/trademind/backend/internal/pkg/response"
	"gorm.io/gorm"
)

// productRoutePrefix marks routes whose ":id" path parameter is a product id.
const productRoutePrefix = "/products/:id"

// ProductRouteTenantGuard rejects product-scoped routes whose product belongs
// to another tenant. Product sub-resources (readiness, operation progress,
// platform configs, images, skus, ai tasks) are spread across several modules
// and used to rely on each handler remembering to scope by tenant, which leaked
// product data across tenants. Enforcing it once on the authenticated group
// keeps the boundary fail-closed for current and future sub-routes.
func ProductRouteTenantGuard(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		if db == nil {
			c.Next()
			return
		}
		full := c.FullPath()
		if !strings.Contains(full, productRoutePrefix) {
			c.Next()
			return
		}
		pid, err := uuid.Parse(strings.TrimSpace(c.Param("id")))
		if err != nil || pid == uuid.Nil {
			c.Next()
			return
		}
		if err := EnsureProductInTenant(c, db, pid); err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				response.Fail(c, 404, response.CodeNotFound, "not found")
			} else {
				response.HandleError(c, err)
			}
			c.Abort()
			return
		}
		c.Next()
	}
}
