package migrationimport

import (
	"errors"
	"fmt"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"github.com/trademind-ai/trademind/backend/internal/modules/inventory"
	"github.com/trademind-ai/trademind/backend/internal/modules/product"
	"github.com/trademind-ai/trademind/backend/internal/pkg/adminperm"
)

// openingEventKey identifies one opening-balance write per tenant + SKU +
// warehouse. A SKU+warehouse pair therefore accepts only one opening import,
// no matter how many files carry it.
func openingEventKey(tenantID int64, skuID uuid.UUID, warehouseID *uuid.UUID) string {
	whKey := "default"
	if warehouseID != nil {
		whKey = warehouseID.String()
	}
	return fmt.Sprintf("import_opening:%d:%s:%s", tenantID, skuID.String(), whKey)
}

// findTenantSKU resolves one SKU code to the tenant's SKU row (joined through
// non-deleted products). Ambiguous codes are rejected.
func (s *Service) findTenantSKU(c *gin.Context, tenantID int64, skuCode string) (*product.ProductSKU, error) {
	var skus []product.ProductSKU
	if err := s.DB.WithContext(c.Request.Context()).
		Joins("JOIN products ON products.id = product_skus.product_id AND products.deleted_at IS NULL").
		Where("products.tenant_id = ? AND product_skus.sku_code = ?", tenantID, skuCode).
		Limit(2).Find(&skus).Error; err != nil {
		return nil, err
	}
	switch len(skus) {
	case 0:
		return nil, fmt.Errorf("SKU「%s」不存在", skuCode)
	case 1:
		return &skus[0], nil
	default:
		return nil, fmt.Errorf("SKU「%s」存在多条记录，无法确定目标", skuCode)
	}
}

// resolveOpeningWarehouse maps a warehouse code to a tenant warehouse row.
// Empty code means the default warehouse (nil when multi-warehouse is not
// enabled yet, keeping the pre-multi-warehouse total-stock behaviour).
func (s *Service) resolveOpeningWarehouse(c *gin.Context, tenantID int64, code string) (*inventory.Warehouse, error) {
	if code == "" {
		var def inventory.Warehouse
		err := s.DB.WithContext(c.Request.Context()).
			Where("tenant_id = ? AND is_default = ?", tenantID, true).First(&def).Error
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		if err != nil {
			return nil, err
		}
		return &def, nil
	}
	var w inventory.Warehouse
	err := s.DB.WithContext(c.Request.Context()).
		Where("tenant_id = ? AND code = ?", tenantID, code).First(&w).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, fmt.Errorf("仓库「%s」不存在", code)
	}
	if err != nil {
		return nil, err
	}
	if !w.Enabled {
		return nil, fmt.Errorf("仓库「%s」已停用", code)
	}
	return &w, nil
}

// rowOutcome buffers one row's result inside a chunk transaction so job
// counters are only updated after the chunk actually commits.
type rowOutcome struct {
	rowNumber int
	status    string // "" means success
	field     string
	message   string
}

func (s *Service) applyOutcomes(job *ImportJob, errorRows *[]ImportJobRow, body WizardBody, outcomes []rowOutcome) {
	for _, o := range outcomes {
		if o.status == "" {
			job.SuccessRows++
			continue
		}
		s.markRows(job, errorRows, body, []int{o.rowNumber}, o.status, o.field, o.message)
	}
}

// commitInventory writes opening balances: total SKU stock, the non-default
// warehouse stock line, one inventory change log, and optionally the
// reference cost price. Idempotent per SKU+warehouse via the change-log
// business event key. SKUs are resolved in bulk and rows are written in
// chunked transactions (one commit per chunk, one savepoint per row) so
// 10k-row files stay fast while a bad row still only fails itself.
func (s *Service) commitInventory(c *gin.Context, job *ImportJob, errorRows *[]ImportJobRow, body WizardBody, rows []InventoryInput, adminID *uuid.UUID) {
	tid, tenantErr := adminperm.TenantIDFromGin(c)
	pKey := progressKey(tid, job.Kind, job.BatchKey)
	if tenantErr != nil {
		for _, in := range rows {
			s.markRows(job, errorRows, body, []int{in.RowNumber}, RowStatusFailed, FSKUCode, tenantErr.Error())
		}
		commits.advance(pKey, len(rows))
		return
	}
	codes := make([]string, 0, len(rows))
	for _, in := range rows {
		codes = append(codes, in.SKUCode)
	}
	foundSKUs, ambiguous, err := s.findTenantSKUsBulk(c, tid, codes)
	if err != nil {
		for _, in := range rows {
			s.markRows(job, errorRows, body, []int{in.RowNumber}, RowStatusFailed, FSKUCode, err.Error())
		}
		commits.advance(pKey, len(rows))
		return
	}
	// Warehouse rows repeat across the file; resolve each code once, before
	// any chunk transaction starts (s.DB queries inside an open transaction
	// deadlock on single-connection test databases).
	whCache := map[string]*inventory.Warehouse{}
	whErrs := map[string]error{}
	for _, in := range rows {
		code := in.WarehouseCode
		if _, ok := whCache[code]; ok {
			continue
		}
		if _, ok := whErrs[code]; ok {
			continue
		}
		wh, err := s.resolveOpeningWarehouse(c, tid, code)
		if err != nil {
			whErrs[code] = err
			continue
		}
		whCache[code] = wh
	}
	dbErr := chunkTx(s.DB.WithContext(c.Request.Context()), len(rows), commitChunkSize, func(tx *gorm.DB, start, end int) error {
		outcomes := make([]rowOutcome, 0, end-start)
		for _, in := range rows[start:end] {
			sku, err := resolveBulkSKU(foundSKUs, ambiguous, in.SKUCode)
			if err != nil {
				outcomes = append(outcomes, rowOutcome{in.RowNumber, RowStatusFailed, FSKUCode, err.Error()})
				continue
			}
			if whErr, bad := whErrs[in.WarehouseCode]; bad {
				outcomes = append(outcomes, rowOutcome{in.RowNumber, RowStatusFailed, FWarehouseCode, whErr.Error()})
				continue
			}
			wh := whCache[in.WarehouseCode]
			var whID *uuid.UUID
			if wh != nil {
				id := wh.ID
				whID = &id
			}
			key := openingEventKey(tid, sku.ID, whID)
			// Nested transaction = savepoint: a failing row rolls back only
			// itself, not the whole chunk.
			txErr := tx.Transaction(func(rowTx *gorm.DB) error {
				return s.writeOpeningRow(rowTx, tid, sku, wh, whID, key, in, job, adminID)
			})
			switch {
			case errors.Is(txErr, errOpeningDuplicate):
				outcomes = append(outcomes, rowOutcome{in.RowNumber, RowStatusDuplicate, FSKUCode,
					fmt.Sprintf("SKU「%s」该仓库已有期初导入记录，跳过", in.SKUCode)})
			case txErr != nil:
				outcomes = append(outcomes, rowOutcome{in.RowNumber, RowStatusFailed, "", txErr.Error()})
			default:
				outcomes = append(outcomes, rowOutcome{rowNumber: in.RowNumber})
			}
		}
		s.applyOutcomes(job, errorRows, body, outcomes)
		commits.advance(pKey, end-start)
		return nil
	})
	if dbErr != nil {
		// A chunk-level commit error is unexpected (per-row errors are
		// buffered); remaining rows were not processed.
		s.markRows(job, errorRows, body, []int{rows[0].RowNumber}, RowStatusFailed, "", dbErr.Error())
	}
}

// writeOpeningRow applies one opening-balance row inside its savepoint.
func (s *Service) writeOpeningRow(tx *gorm.DB, tid int64, sku *product.ProductSKU, wh *inventory.Warehouse, whID *uuid.UUID, key string, in InventoryInput, job *ImportJob, adminID *uuid.UUID) error {
	var hit int64
	if err := tx.Model(&inventory.InventoryChangeLog{}).
		Where("business_event_key = ?", key).Count(&hit).Error; err != nil {
		return err
	}
	if hit > 0 {
		return errOpeningDuplicate
	}
	var locked product.ProductSKU
	if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
		First(&locked, "id = ?", sku.ID).Error; err != nil {
		return err
	}
	before := 0
	if locked.Stock != nil {
		before = *locked.Stock
	}
	after := before + in.Quantity
	updates := map[string]any{"stock": after}
	if in.CostPrice != nil {
		updates["cost_price"] = *in.CostPrice
	}
	if err := tx.Model(&product.ProductSKU{}).Where("id = ?", locked.ID).
		Updates(updates).Error; err != nil {
		return err
	}
	if wh != nil && !wh.IsDefault {
		if err := addOpeningWarehouseStockTx(tx, tid, wh.ID, locked.ProductID, locked.ID, in.Quantity); err != nil {
			return err
		}
	}
	whRemark := ""
	if wh != nil {
		whRemark = " warehouse=" + wh.Code
	}
	return tx.Create(&inventory.InventoryChangeLog{
		TenantID:         tid,
		ProductID:        locked.ProductID,
		ProductSKUID:     locked.ID,
		WarehouseID:      whID,
		ChangeType:       inventory.ChangeImportOpening,
		BeforeStock:      before,
		AfterStock:       after,
		Delta:            in.Quantity,
		Reason:           "import_opening",
		Remark:           fmt.Sprintf("库存期初导入 file=%s%s", job.FileName, whRemark),
		CreatedBy:        adminID,
		BusinessEventKey: key,
	}).Error
}

var errOpeningDuplicate = errors.New("opening balance already imported")

// addOpeningWarehouseStockTx upserts (with a row lock) a non-default
// warehouse stock line, adding delta to it.
func addOpeningWarehouseStockTx(tx *gorm.DB, tenantID int64, warehouseID, productID, skuID uuid.UUID, delta int) error {
	var row inventory.WarehouseStock
	err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
		Where("warehouse_id = ? AND product_sku_id = ?", warehouseID, skuID).
		First(&row).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		row = inventory.WarehouseStock{
			TenantID:     tenantID,
			WarehouseID:  warehouseID,
			ProductID:    productID,
			ProductSKUID: skuID,
			Stock:        delta,
		}
		return tx.Create(&row).Error
	}
	if err != nil {
		return err
	}
	return tx.Model(&inventory.WarehouseStock{}).Where("id = ?", row.ID).
		Update("stock", row.Stock+delta).Error
}
