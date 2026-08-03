package productpublish

import (
	"context"
	"errors"
	"fmt"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"gorm.io/gorm"

	"github.com/trademind-ai/trademind/backend/internal/modules/admin"
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

// ensureBatchVisibleTenant verifies the batch belongs to the given tenant via
// the tenant_id column. Rows still at tenant 0 with a creator (not yet
// backfilled) fall back to the creator admin user's tenant. Cross-tenant
// access returns gorm.ErrRecordNotFound so endpoints respond 404 without
// leaking existence.
func (s *Service) ensureBatchVisibleTenant(ctx context.Context, b *ProductPublishBatch, tenantID int64) error {
	if s == nil || s.DB == nil || b == nil {
		return fmt.Errorf("no db")
	}
	batchTenant := b.TenantID
	if batchTenant == 0 && b.CreatedBy != nil {
		var u admin.AdminUser
		err := s.DB.WithContext(ctx).Select("id", "tenant_id").First(&u, "id = ?", *b.CreatedBy).Error
		switch {
		case err == nil:
			batchTenant = u.TenantID
		case errors.Is(err, gorm.ErrRecordNotFound):
			// creator row gone: batch stays in the legacy tenant-0 bucket
		default:
			return err
		}
	}
	if batchTenant != tenantID {
		return gorm.ErrRecordNotFound
	}
	return nil
}

// ensureBatchVisible is the gin wrapper reading the request tenant.
func (s *Service) ensureBatchVisible(c *gin.Context, b *ProductPublishBatch) error {
	tid, err := s.tenantID(c)
	if err != nil {
		return err
	}
	return s.ensureBatchVisibleTenant(c.Request.Context(), b, tid)
}
