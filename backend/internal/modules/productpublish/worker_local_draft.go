package productpublish

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"

	"github.com/trademind-ai/trademind/backend/internal/modules/operationlog"
	"github.com/trademind-ai/trademind/backend/internal/modules/product"
	"gorm.io/datatypes"
	"gorm.io/gorm"
)

// processLocalDraftTask completes a queued local_draft_create task (e.g. a
// failed task re-enqueued via retry). Local drafts never call a platform
// API: the worker rebuilds the listing draft from the product and records a
// draft publication, matching the inline create path.
func (s *Service) processLocalDraftTask(ctx context.Context, taskID uuid.UUID, workerID string) error {
	if s == nil || s.DB == nil {
		return fmt.Errorf("productpublish: no db")
	}
	defer func() {
		if r := recover(); r != nil {
			s.handlePublishPanic(ctx, taskID, workerID, r)
		}
	}()

	lease := s.publishLeaseTTL()
	taskRow, claim, claimed, err := s.tryClaimProductPublishTask(ctx, taskID, workerID, lease)
	if err != nil {
		return err
	}
	if !claimed || taskRow == nil {
		return nil
	}
	cancelRen := s.startPublishLeaseRenewal(ctx, taskID, workerID, claim, lease)
	defer cancelRen()

	fail := func(msg string) error {
		fin := time.Now().UTC()
		_ = s.finishProductPublishTask(ctx, taskID, workerID, claim, map[string]any{
			"status":         TaskFailed,
			"publish_status": StatusPubFailed,
			"error_code":     inferPublishErrorCode(msg),
			"error_message":  localizePublishFailMessage(msg),
			"finished_at":    &fin,
		})
		if s.OpLog != nil {
			_ = s.OpLog.WriteBackground(ctx, operationlog.WriteOpts{
				AdminUserID: taskRow.CreatedBy,
				Action:      "product.publish.failed",
				Resource:    "product_publish_task",
				ResourceID:  taskID.String(),
				Status:      "failed",
				Message:     fmt.Sprintf("taskId=%s err=%s", taskID.String(), truncateMsg(msg)),
			})
		}
		return fmt.Errorf("%s", msg)
	}

	var prod product.Product
	if err := s.DB.WithContext(ctx).
		Preload("Images", func(db *gorm.DB) *gorm.DB { return db.Order("sort_order ASC, created_at ASC") }).
		Preload("SKUs", func(db *gorm.DB) *gorm.DB { return db.Order("created_at ASC") }).
		First(&prod, "id = ?", taskRow.ProductID).Error; err != nil {
		return fail(fmt.Sprintf("load product: %v", err))
	}
	draft, err := BuildPlatformDraftFromProduct(prod)
	if err != nil {
		return fail(err.Error())
	}

	fin := time.Now().UTC()
	snap := map[string]any{
		"localDraftOnly": true,
		"capability":     CapLocalDraftOnly,
		"title":          draft.Title,
		"description":    draft.Description,
		"currency":       draft.Currency,
		"imageCount":     len(draft.Images),
		"skuCount":       len(draft.SKUs),
	}
	snapRaw, _ := json.Marshal(snap)

	pubRow := ProductPublication{
		ProductID:     taskRow.ProductID,
		ShopID:        taskRow.ShopID,
		Platform:      taskRow.Platform,
		Status:        StatusDraft,
		PublishStatus: StatusDraftCreated,
		Title:         draft.Title,
		Currency:      draft.Currency,
		PublishMode:   PublishModeSaveAsPlatformDraft,
		CreatedBy:     taskRow.CreatedBy,
		LastSyncedAt:  &fin,
		RawData:       datatypes.JSON(snapRaw),
	}
	if err := s.DB.WithContext(ctx).Create(&pubRow).Error; err != nil {
		return fail(err.Error())
	}
	_ = s.DB.WithContext(ctx).Model(&ProductPublication{}).Where("id = ?", pubRow.ID).
		Updates(map[string]any{"publish_task_id": taskID}).Error

	if err := s.finishProductPublishTask(ctx, taskID, workerID, claim, map[string]any{
		"status":         TaskSuccess,
		"publish_status": StatusDraftCreated,
		"error_code":     "",
		"error_message":  "",
		"title":          draft.Title,
		"description":    draft.Description,
		"output":         datatypes.JSON(snapRaw),
		"finished_at":    &fin,
	}); err != nil {
		return err
	}

	if s.OpLog != nil {
		_ = s.OpLog.WriteBackground(ctx, operationlog.WriteOpts{
			AdminUserID: taskRow.CreatedBy,
			Action:      "product.publish.success",
			Resource:    "product_publish_task",
			ResourceID:  taskID.String(),
			Status:      "success",
			Message: fmt.Sprintf("taskId=%s publicationId=%s capability=%s",
				taskID.String(), pubRow.ID.String(), CapLocalDraftOnly),
		})
	}
	return nil
}
