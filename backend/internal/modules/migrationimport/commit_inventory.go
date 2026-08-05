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

// commitInventory writes opening balances row by row: total SKU stock, the
// non-default warehouse stock line, one inventory change log, and optionally
// the reference cost price. Idempotent per SKU+warehouse via the change-log
// business event key.
func (s *Service) commitInventory(c *gin.Context, job *ImportJob, errorRows *[]ImportJobRow, body WizardBody, rows []InventoryInput, adminID *uuid.UUID) {
	tid, tenantErr := adminperm.TenantIDFromGin(c)
	for _, in := range rows {
		if tenantErr != nil {
			s.markRows(job, errorRows, body, []int{in.RowNumber}, RowStatusFailed, FSKUCode, tenantErr.Error())
			continue
		}
		sku, err := s.findTenantSKU(c, tid, in.SKUCode)
		if err != nil {
			s.markRows(job, errorRows, body, []int{in.RowNumber}, RowStatusFailed, FSKUCode, err.Error())
			continue
		}
		wh, err := s.resolveOpeningWarehouse(c, tid, in.WarehouseCode)
		if err != nil {
			s.markRows(job, errorRows, body, []int{in.RowNumber}, RowStatusFailed, FWarehouseCode, err.Error())
			continue
		}
		var whID *uuid.UUID
		if wh != nil {
			id := wh.ID
			whID = &id
		}
		key := openingEventKey(tid, sku.ID, whID)
		txErr := s.DB.WithContext(c.Request.Context()).Transaction(func(tx *gorm.DB) error {
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
		})
		if errors.Is(txErr, errOpeningDuplicate) {
			s.markRows(job, errorRows, body, []int{in.RowNumber}, RowStatusDuplicate, FSKUCode,
				fmt.Sprintf("SKU「%s」该仓库已有期初导入记录，跳过", in.SKUCode))
			continue
		}
		if txErr != nil {
			s.markRows(job, errorRows, body, []int{in.RowNumber}, RowStatusFailed, "", txErr.Error())
			continue
		}
		job.SuccessRows++
	}
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
