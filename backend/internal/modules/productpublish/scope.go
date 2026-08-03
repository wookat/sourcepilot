package productpublish

import (
	"context"
	"fmt"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"github.com/trademind-ai/trademind/backend/internal/modules/shop"
	"github.com/trademind-ai/trademind/backend/internal/pkg/adminperm"
)

// tenantID reads the request tenant installed by the auth middleware.
func (s *Service) tenantID(c *gin.Context) (int64, error) {
	return adminperm.TenantIDFromGin(c)
}

// shopBelongsToTenant reports whether the shop exists inside the tenant. It is
// used to keep publish targets from pointing at another tenant's shop.
func (s *Service) shopBelongsToTenant(ctx context.Context, tenantID int64, shopID uuid.UUID) bool {
	if s == nil || s.DB == nil {
		return false
	}
	var n int64
	if err := s.DB.WithContext(ctx).Model(&shop.Shop{}).
		Where("id = ? AND tenant_id = ?", shopID, tenantID).
		Count(&n).Error; err != nil {
		return false
	}
	return n > 0
}

// errProductNotFound keeps cross-tenant products indistinguishable from missing
// ones for publish flows.
func errProductNotFound(productID uuid.UUID) error {
	return fmt.Errorf("product %s not found or not eligible", productID.String())
}
