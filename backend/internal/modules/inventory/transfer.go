package inventory

import (
	"context"
	"fmt"
	"strings"

	"github.com/google/uuid"
	"github.com/trademind-ai/trademind/backend/internal/modules/operationlog"
	"github.com/trademind-ai/trademind/backend/internal/modules/product"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// TransferStockBody POST /inventory/transfers
type TransferStockBody struct {
	ProductSKUID    string `json:"productSkuId"`
	FromWarehouseID string `json:"fromWarehouseId"`
	ToWarehouseID   string `json:"toWarehouseId"`
	Quantity        int    `json:"quantity"`
	Remark          string `json:"remark"`
}

// TransferResult is the outcome of one warehouse-to-warehouse stock transfer.
type TransferResult struct {
	TransferID        uuid.UUID `json:"transferId"`
	ProductSKUID      uuid.UUID `json:"productSkuId"`
	FromWarehouseID   uuid.UUID `json:"fromWarehouseId"`
	FromWarehouseName string    `json:"fromWarehouseName"`
	ToWarehouseID     uuid.UUID `json:"toWarehouseId"`
	ToWarehouseName   string    `json:"toWarehouseName"`
	Quantity          int       `json:"quantity"`
	FromBefore        int       `json:"fromBefore"`
	FromAfter         int       `json:"fromAfter"`
	ToBefore          int       `json:"toBefore"`
	ToAfter           int       `json:"toAfter"`
	OutLogID          uuid.UUID `json:"outLogId"`
	InLogID           uuid.UUID `json:"inLogId"`
}

// TransferStock moves SKU stock between two warehouses of the same tenant in
// one transaction: both warehouse lines change together and two ledger rows
// (transfer_out / transfer_in) are written atomically. The SKU total stays
// unchanged, so platform sync is not triggered.
func (s *Service) TransferStock(ctx context.Context, tenantID int64, body TransferStockBody, operator *uuid.UUID) (*TransferResult, error) {
	if s == nil || s.DB == nil {
		return nil, fmt.Errorf("inventory: no db")
	}
	skuID, err := uuid.Parse(strings.TrimSpace(body.ProductSKUID))
	if err != nil {
		return nil, fmt.Errorf("invalid productSkuId")
	}
	fromID, err := uuid.Parse(strings.TrimSpace(body.FromWarehouseID))
	if err != nil {
		return nil, fmt.Errorf("invalid fromWarehouseId")
	}
	toID, err := uuid.Parse(strings.TrimSpace(body.ToWarehouseID))
	if err != nil {
		return nil, fmt.Errorf("invalid toWarehouseId")
	}
	if fromID == toID {
		return nil, ErrTransferSameWarehouse
	}
	if body.Quantity <= 0 {
		return nil, ErrTransferInvalidQuantity
	}
	from, err := s.GetWarehouse(ctx, tenantID, fromID)
	if err != nil {
		return nil, err
	}
	to, err := s.GetWarehouse(ctx, tenantID, toID)
	if err != nil {
		return nil, err
	}
	if !to.Enabled {
		return nil, ErrWarehouseDisabled
	}

	res := &TransferResult{
		TransferID:        uuid.New(),
		ProductSKUID:      skuID,
		FromWarehouseID:   from.ID,
		FromWarehouseName: from.Name,
		ToWarehouseID:     to.ID,
		ToWarehouseName:   to.Name,
		Quantity:          body.Quantity,
	}
	remark := clampStr(strings.TrimSpace(body.Remark), 400)

	txErr := s.DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		// Tenant-scoped SKU row lock serializes all warehouse mutations per SKU.
		var sku product.ProductSKU
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE", Table: clause.Table{Name: "product_skus"}}).
			Joins("JOIN products p ON p.id = product_skus.product_id AND p.deleted_at IS NULL").
			Where("product_skus.id = ? AND p.tenant_id = ?", skuID, tenantID).
			First(&sku).Error; err != nil {
			return err
		}
		total := derefStock(sku.Stock)

		fromBefore, fromAfter, err := s.addWarehouseStockTx(tx, tenantID, from, sku.ProductID, sku.ID, -body.Quantity, total)
		if err != nil {
			return err
		}
		if fromAfter < 0 {
			return ErrInsufficientWarehouse
		}
		toBefore, toAfter, err := s.addWarehouseStockTx(tx, tenantID, to, sku.ProductID, sku.ID, body.Quantity, total)
		if err != nil {
			return err
		}
		res.FromBefore, res.FromAfter = fromBefore, fromAfter
		res.ToBefore, res.ToAfter = toBefore, toAfter

		rm := clampStr(fmt.Sprintf("仓库调拨 %s→%s transfer=%s %s", from.Name, to.Name, res.TransferID.String(), remark), 520)
		outLog := InventoryChangeLog{
			TenantID:         tenantID,
			ProductID:        sku.ProductID,
			ProductSKUID:     sku.ID,
			WarehouseID:      ptrUUID(from.ID),
			ChangeType:       ChangeTransferOut,
			BeforeStock:      fromBefore,
			AfterStock:       fromAfter,
			Delta:            -body.Quantity,
			Reason:           "warehouse_transfer",
			Remark:           rm,
			CreatedBy:        operator,
			BusinessEventKey: fmt.Sprintf("warehouse_transfer:%s:out", res.TransferID.String()),
		}
		if err := tx.Create(&outLog).Error; err != nil {
			return err
		}
		inLog := InventoryChangeLog{
			TenantID:         tenantID,
			ProductID:        sku.ProductID,
			ProductSKUID:     sku.ID,
			WarehouseID:      ptrUUID(to.ID),
			ChangeType:       ChangeTransferIn,
			BeforeStock:      toBefore,
			AfterStock:       toAfter,
			Delta:            body.Quantity,
			Reason:           "warehouse_transfer",
			Remark:           rm,
			CreatedBy:        operator,
			BusinessEventKey: fmt.Sprintf("warehouse_transfer:%s:in", res.TransferID.String()),
		}
		if err := tx.Create(&inLog).Error; err != nil {
			return err
		}
		res.OutLogID = outLog.ID
		res.InLogID = inLog.ID
		return nil
	})
	if txErr != nil {
		return nil, txErr
	}

	if s.OpLog != nil {
		_ = s.OpLog.WriteBackground(ctx, operationlog.WriteOpts{
			AdminUserID: operator,
			Action:      "inventory.warehouse.transfer",
			Resource:    "product_sku",
			ResourceID:  skuID.String(),
			Status:      "success",
			Message: fmt.Sprintf("from=%s to=%s qty=%d transfer=%s",
				from.Name, to.Name, body.Quantity, res.TransferID.String()),
		})
	}
	return res, nil
}
