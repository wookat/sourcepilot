package migrationimport

import (
	"errors"
	"fmt"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"github.com/trademind-ai/trademind/backend/internal/modules/sourcing"
	"github.com/trademind-ai/trademind/backend/internal/pkg/adminperm"
)

// commitSources imports source-archive rows through the sourcing module:
// supplier find-or-create + product source binding via BindSource, and the
// SKU mapping (with the reference price) via SaveSKUMappings. An existing
// mapping for the same source + SKU is reported as a duplicate and skipped.
func (s *Service) commitSources(c *gin.Context, job *ImportJob, errorRows *[]ImportJobRow, body WizardBody, rows []SourceInput, adminID *uuid.UUID) {
	if s.Sourcing == nil {
		s.markRowsAll(job, errorRows, body, rows, "货源模块不可用")
		return
	}
	tid, tenantErr := adminperm.TenantIDFromGin(c)
	for _, in := range rows {
		if tenantErr != nil {
			s.markRows(job, errorRows, body, []int{in.RowNumber}, RowStatusFailed, FSupplierName, tenantErr.Error())
			continue
		}
		sku, err := s.findTenantSKU(c, tid, in.SKUCode)
		if err != nil {
			s.markRows(job, errorRows, body, []int{in.RowNumber}, RowStatusFailed, FSKUCode, err.Error())
			continue
		}
		src, err := s.Sourcing.BindSource(c, sku.ProductID, sourcing.BindSourceBody{
			SupplierName: in.SupplierName,
			SourceURL:    in.SourceURL,
		}, adminID)
		if errors.Is(err, sourcing.ErrConflict) {
			src, err = s.findExistingSource(c, tid, sku.ProductID, in)
		}
		if err != nil {
			s.markRows(job, errorRows, body, []int{in.RowNumber}, RowStatusFailed, FSupplierName, err.Error())
			continue
		}
		var mapped int64
		if err := s.DB.WithContext(c.Request.Context()).Model(&sourcing.ProductSourceSKU{}).
			Where("product_source_id = ? AND local_sku_id = ?", src.ID, sku.ID).
			Count(&mapped).Error; err != nil {
			s.markRows(job, errorRows, body, []int{in.RowNumber}, RowStatusFailed, "", err.Error())
			continue
		}
		if mapped > 0 {
			s.markRows(job, errorRows, body, []int{in.RowNumber}, RowStatusDuplicate, FSKUCode,
				fmt.Sprintf("SKU「%s」与该供应商的货源映射已存在，跳过", in.SKUCode))
			continue
		}
		if _, err := s.Sourcing.SaveSKUMappings(c, src.ID, []sourcing.SKUMappingBody{{
			LocalSKUID:    sku.ID.String(),
			ExternalSKUID: in.ExternalSKUID,
			CurrentPrice:  in.RefPrice,
		}}, adminID); err != nil {
			s.markRows(job, errorRows, body, []int{in.RowNumber}, RowStatusFailed, FRefPrice, err.Error())
			continue
		}
		job.SuccessRows++
	}
}

// findExistingSource loads the already-bound product source matching the
// row's supplier so a conflicting bind can still attach the SKU mapping.
func (s *Service) findExistingSource(c *gin.Context, tenantID int64, productID uuid.UUID, in SourceInput) (*sourcing.ProductSource, error) {
	var src sourcing.ProductSource
	err := s.DB.WithContext(c.Request.Context()).
		Joins("JOIN suppliers ON suppliers.id = product_sources.supplier_id AND suppliers.deleted_at IS NULL").
		Where("product_sources.tenant_id = ? AND product_sources.product_id = ? AND suppliers.name = ?",
			tenantID, productID, in.SupplierName).
		Order("product_sources.created_at ASC").
		First(&src).Error
	if err != nil {
		return nil, fmt.Errorf("货源已绑定但无法定位既有记录: %w", err)
	}
	return &src, nil
}

func (s *Service) markRowsAll(job *ImportJob, errorRows *[]ImportJobRow, body WizardBody, rows []SourceInput, msg string) {
	for _, in := range rows {
		s.markRows(job, errorRows, body, []int{in.RowNumber}, RowStatusFailed, "", msg)
	}
}
