package procurement

import (
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

func purchaseInboundEventKey(poID, itemID uuid.UUID) string {
	return fmt.Sprintf("purchase_inbound:%s:%s", poID.String(), itemID.String())
}

// inboundStockTx adds each purchase line quantity to local SKU stock and
// writes an append-only inventory change log per line. Idempotent via the
// change-log business event key: lines already inbound are skipped.
func inboundStockTx(tx *gorm.DB, po *PurchaseOrder, operator *uuid.UUID) ([]InboundLine, error) {
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
		chg := inventory.InventoryChangeLog{
			TenantID:         po.TenantID,
			ProductID:        sku.ProductID,
			ProductSKUID:     sku.ID,
			ChangeType:       inventory.ChangePurchaseInbound,
			BeforeStock:      before,
			AfterStock:       after,
			Delta:            it.Quantity,
			Reason:           "purchase_inbound",
			Remark:           fmt.Sprintf("采购单签收入库 po=%s", po.ID.String()),
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
