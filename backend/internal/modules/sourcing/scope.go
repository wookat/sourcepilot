package sourcing

import (
	"errors"
	"fmt"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"gorm.io/gorm"

	"github.com/trademind-ai/trademind/backend/internal/pkg/adminperm"
)

// tenantID returns the trusted tenant id of the request.
func (s *Service) tenantID(c *gin.Context) (int64, error) {
	if s == nil || s.DB == nil {
		return 0, fmt.Errorf("sourcing: no db")
	}
	return adminperm.TenantIDFromGin(c)
}

// ensureSupplierVisible verifies the supplier belongs to the request's tenant;
// cross-tenant / unknown ids return ErrNotFound so endpoints answer 404 without
// leaking existence.
func (s *Service) ensureSupplierVisible(c *gin.Context, id uuid.UUID) error {
	tid, err := s.tenantID(c)
	if err != nil {
		return err
	}
	var sup Supplier
	err = s.DB.WithContext(c.Request.Context()).Select("id").
		Where("tenant_id = ?", tid).First(&sup, "id = ?", id).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return ErrNotFound
	}
	return err
}

// ensureSourceVisible verifies the product source belongs to the request's tenant.
func (s *Service) ensureSourceVisible(c *gin.Context, id uuid.UUID) error {
	tid, err := s.tenantID(c)
	if err != nil {
		return err
	}
	var src ProductSource
	err = s.DB.WithContext(c.Request.Context()).Select("id").
		Where("tenant_id = ?", tid).First(&src, "id = ?", id).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return ErrNotFound
	}
	return err
}

// ensureSwitchEventVisible verifies the switch event belongs to the request's tenant.
func (s *Service) ensureSwitchEventVisible(c *gin.Context, id uuid.UUID) error {
	tid, err := s.tenantID(c)
	if err != nil {
		return err
	}
	var ev SourceSwitchEvent
	err = s.DB.WithContext(c.Request.Context()).Select("id").
		Where("tenant_id = ?", tid).First(&ev, "id = ?", id).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return ErrNotFound
	}
	return err
}
