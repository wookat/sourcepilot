package imagetask

import (
	"errors"
	"fmt"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"gorm.io/gorm"

	"github.com/trademind-ai/trademind/backend/internal/modules/product"
	"github.com/trademind-ai/trademind/backend/internal/pkg/adminperm"
	"github.com/trademind-ai/trademind/backend/internal/pkg/repository"
)

// EnsureTaskVisible verifies the task's product belongs to the request's tenant;
// cross-tenant access returns gorm.ErrRecordNotFound so handlers answer 404
// without leaking task existence.
func (s *Service) EnsureTaskVisible(c *gin.Context, taskID uuid.UUID) error {
	return s.ensureTaskVisible(c, taskID)
}

// EnsureProductVisible verifies a product referenced by a request body or path
// belongs to the request's tenant, so task creation and apply/score writes
// cannot target another tenant's catalog.
func (s *Service) EnsureProductVisible(c *gin.Context, productID uuid.UUID) error {
	if s == nil || s.DB == nil {
		return fmt.Errorf("imagetask: no db")
	}
	tid, err := adminperm.TenantIDFromGin(c)
	if err != nil {
		return err
	}
	var p product.Product
	return repository.FindByID(c.Request.Context(), s.DB.Select("id"), &p, tid, productID)
}

// EnsureTaskItemVisible verifies the item's parent task is visible to the
// request's tenant.
func (s *Service) EnsureTaskItemVisible(c *gin.Context, itemID uuid.UUID) error {
	if s == nil || s.DB == nil {
		return fmt.Errorf("imagetask: no db")
	}
	var item ImageTaskItem
	if err := s.DB.WithContext(c.Request.Context()).Select("id", "task_id").
		First(&item, "id = ?", itemID).Error; err != nil {
		return err
	}
	return s.ensureTaskVisible(c, item.TaskID)
}

// notFound reports whether err means "no such row for this tenant".
func notFound(err error) bool {
	return errors.Is(err, gorm.ErrRecordNotFound)
}
