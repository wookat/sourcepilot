package procurement

import (
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/trademind-ai/trademind/backend/internal/modules/inventory"
	"github.com/trademind-ai/trademind/backend/internal/modules/product"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// InboundLine is the per-line result of a purchase-order inbound.
type InboundLine struct {
	ItemID       uuid.UUID `json:"itemId"`
	ProductSKUID uuid.UUID `json:"productSkuId"`
	SKUCode      string    `json:"skuCode,omitempty"`
	Quantity     int       `json:"quantity"`
	BeforeStock  int       `json:"beforeStock"`
	AfterStock   int       `json:"afterStock"`
	Skipped      bool      `json:"skipped,omitempty"`
	Reason       string    `json:"reason,omitempty"`
}

var (
	// ErrInboundWarehouseNotFound is returned when the requested inbound
	// warehouse does not belong to the purchase order's tenant.
	ErrInboundWarehouseNotFound = errors.New("inbound warehouse not found")
	// ErrInboundWarehouseDisabled is returned when the requested inbound
	// warehouse is disabled.
	ErrInboundWarehouseDisabled = errors.New("inbound warehouse disabled")
)

// resolveInboundWarehouse maps the optional requested warehouse to a tenant
// warehouse row; nil request means the default warehouse (nil result keeps the
// pre-multi-warehouse behaviour when no default warehouse row exists yet).
func resolveInboundWarehouse(tx *gorm.DB, tenantID int64, warehouseID *uuid.UUID) (*inventory.Warehouse, error) {
	if warehouseID != nil && *warehouseID != uuid.Nil {
		var w inventory.Warehouse
		err := tx.Where("id = ? AND tenant_id = ?", *warehouseID, tenantID).First(&w).Error
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrInboundWarehouseNotFound
		}
		if err != nil {
			return nil, err
		}
		if !w.Enabled {
			return nil, ErrInboundWarehouseDisabled
		}
		return &w, nil
	}
	var def inventory.Warehouse
	err := tx.Where("tenant_id = ? AND is_default = ?", tenantID, true).First(&def).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &def, nil
}

func purchaseInboundEventKey(poID, itemID uuid.UUID) string {
	return fmt.Sprintf("purchase_inbound:%s:%s", poID.String(), itemID.String())
}

// inboundStockTx adds each purchase line quantity to local SKU stock and
// writes an append-only inventory change log per line. Idempotent via the
// change-log business event key: lines already inbound are skipped.
func inboundStockTx(tx *gorm.DB, po *PurchaseOrder, operator *uuid.UUID, warehouseID *uuid.UUID) ([]InboundLine, error) {
	wh, err := resolveInboundWarehouse(tx, po.TenantID, warehouseID)
	if err != nil {
		return nil, err
	}
	var items []PurchaseOrderItem
	if err := tx.Where("purchase_order_id = ?", po.ID).Order("id ASC").Find(&items).Error; err != nil {
		return nil, err
	}
	out := make([]InboundLine, 0, len(items))
	for _, it := range items {
		line := InboundLine{ItemID: it.ID, ProductSKUID: it.LocalSKUID, Quantity: it.Quantity}
		if it.LocalSKUID == uuid.Nil || it.Quantity <= 0 {
			line.Skipped = true
			line.Reason = "missing_sku_or_qty"
			out = append(out, line)
			continue
		}
		key := purchaseInboundEventKey(po.ID, it.ID)
		var hit int64
		if err := tx.Model(&inventory.InventoryChangeLog{}).Where("business_event_key = ?", key).Count(&hit).Error; err != nil {
			return nil, err
		}
		if hit > 0 {
			line.Skipped = true
			line.Reason = "already_inbound"
			out = append(out, line)
			continue
		}
		var sku product.ProductSKU
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).First(&sku, "id = ?", it.LocalSKUID).Error; err != nil {
			line.Skipped = true
			line.Reason = "sku_not_found"
			out = append(out, line)
			continue
		}
		before := 0
		if sku.Stock != nil {
			before = *sku.Stock
		}
		after := before + it.Quantity
		if err := tx.Model(&product.ProductSKU{}).Where("id = ?", sku.ID).Update("stock", after).Error; err != nil {
			return nil, err
		}
		if wh != nil && !wh.IsDefault {
			if err := addWarehouseStockRowTx(tx, po.TenantID, wh.ID, sku.ProductID, sku.ID, it.Quantity); err != nil {
				return nil, err
			}
		}
		var whID *uuid.UUID
		var whName string
		if wh != nil {
			id := wh.ID
			whID = &id
			whName = " warehouse=" + wh.Name
		}
		chg := inventory.InventoryChangeLog{
			TenantID:         po.TenantID,
			ProductID:        sku.ProductID,
			ProductSKUID:     sku.ID,
			WarehouseID:      whID,
			ChangeType:       inventory.ChangePurchaseInbound,
			BeforeStock:      before,
			AfterStock:       after,
			Delta:            it.Quantity,
			Reason:           "purchase_inbound",
			Remark:           fmt.Sprintf("采购单签收入库 po=%s%s", po.ID.String(), whName),
			CreatedBy:        operator,
			BusinessEventKey: key,
		}
		if err := tx.Create(&chg).Error; err != nil {
			return nil, err
		}
		line.SKUCode = sku.SKUCode
		line.BeforeStock = before
		line.AfterStock = after
		out = append(out, line)
	}
	return out, nil
}

// addWarehouseStockRowTx upserts (with a row lock) a non-default warehouse
// stock line, adding delta to it.
func addWarehouseStockRowTx(tx *gorm.DB, tenantID int64, warehouseID uuid.UUID, productID, skuID uuid.UUID, delta int) error {
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
