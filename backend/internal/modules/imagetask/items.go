package imagetask

import (
	"fmt"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"gorm.io/gorm"

	"github.com/trademind-ai/trademind/backend/internal/modules/product"
	"github.com/trademind-ai/trademind/backend/internal/pkg/adminperm"
	"github.com/trademind-ai/trademind/backend/internal/pkg/repository"
)

// ensureTaskVisible verifies the parent task's product belongs to the
// request's tenant; cross-tenant access returns gorm.ErrRecordNotFound so
// subresource endpoints respond 404 without leaking existence. Tasks without
// a product linkage (ProductID nil) have no tenant column and stay visible,
// matching the task detail endpoint.
func (s *Service) ensureTaskVisible(c *gin.Context, taskID uuid.UUID) error {
	if s == nil || s.DB == nil {
		return fmt.Errorf("imagetask: no db")
	}
	var t ImageTask
	if err := s.DB.WithContext(c.Request.Context()).Select("id", "product_id").First(&t, "id = ?", taskID).Error; err != nil {
		return err
	}
	if t.ProductID == nil {
		return nil
	}
	tid, err := adminperm.TenantIDFromGin(c)
	if err != nil {
		return err
	}
	var p product.Product
	return repository.FindByID(c.Request.Context(), s.DB.Select("id"), &p, tid, *t.ProductID)
}

// ListTaskItems returns all items for a task.
func (s *Service) ListTaskItems(c *gin.Context, taskID uuid.UUID) ([]ImageTaskItem, error) {
	if s == nil || s.DB == nil {
		return nil, fmt.Errorf("imagetask: no db")
	}
	if err := s.ensureTaskVisible(c, taskID); err != nil {
		return nil, err
	}
	var items []ImageTaskItem
	if err := s.DB.WithContext(c.Request.Context()).Where("task_id = ?", taskID).Order("created_at ASC").Find(&items).Error; err != nil {
		return nil, err
	}
	return items, nil
}

// DeleteTaskItem removes a task item row (does not delete stored files).
func (s *Service) DeleteTaskItem(c *gin.Context, taskID, itemID uuid.UUID) error {
	if s == nil || s.DB == nil {
		return fmt.Errorf("imagetask: no db")
	}
	if err := s.ensureTaskVisible(c, taskID); err != nil {
		return err
	}
	res := s.DB.WithContext(c.Request.Context()).Delete(&ImageTaskItem{}, "id = ? AND task_id = ?", itemID, taskID)
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		return gorm.ErrRecordNotFound
	}
	return nil
}
