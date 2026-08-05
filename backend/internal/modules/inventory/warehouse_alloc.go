package inventory

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// warehouseAvail pairs one warehouse with its available quantity for one SKU
// (default warehouse derived from the SKU total).
type warehouseAvail struct {
	Warehouse Warehouse
	Avail     int
}

// warehouseAvailsTx lists the tenant's warehouses in deduction order
// (priority asc, default warehouse ties first) with per-warehouse availability
// for one SKU. Must run inside a transaction that already holds the SKU row
// lock so the numbers stay stable.
func (s *Service) warehouseAvailsTx(ctx context.Context, tx *gorm.DB, tenantID int64, skuID uuid.UUID, total int) ([]warehouseAvail, error) {
	if _, err := s.EnsureDefaultWarehouse(ctx, tenantID); err != nil {
		return nil, err
	}
	var whs []Warehouse
	if err := tx.Where("tenant_id = ?", tenantID).
		Order("priority ASC, is_default DESC, created_at ASC").
		Find(&whs).Error; err != nil {
		return nil, err
	}
	var rows []WarehouseStock
	if err := tx.Where("tenant_id = ? AND product_sku_id = ?", tenantID, skuID).
		Find(&rows).Error; err != nil {
		return nil, err
	}
	byWh := map[uuid.UUID]int{}
	othersSum := 0
	whByID := map[uuid.UUID]bool{}
	for _, w := range whs {
		whByID[w.ID] = w.IsDefault
	}
	for _, r := range rows {
		byWh[r.WarehouseID] = r.Stock
		if isDef, ok := whByID[r.WarehouseID]; ok && !isDef {
			othersSum += r.Stock
		}
	}
	out := make([]warehouseAvail, 0, len(whs))
	for _, w := range whs {
		avail := byWh[w.ID]
		if w.IsDefault {
			avail = total - othersSum
		}
		out = append(out, warehouseAvail{Warehouse: w, Avail: avail})
	}
	return out, nil
}

// deductAllocation is one warehouse slice of an order-line deduction.
type deductAllocation struct {
	Warehouse Warehouse
	Quantity  int
}

// allocateDeduction decides which warehouses cover a deduction of qty.
// When warehouseID is set the whole quantity comes from that warehouse
// (insufficiency handled by the caller's total-stock check plus allowNeg).
// Otherwise stock is taken by warehouse priority, splitting across
// warehouses when no single one covers the quantity; any remainder
// (allowNeg case) lands on the default warehouse.
func (s *Service) allocateDeduction(ctx context.Context, tx *gorm.DB, tenantID int64, skuID uuid.UUID, total int, qty int, warehouseID *uuid.UUID, allowNeg bool) ([]deductAllocation, error) {
	avails, err := s.warehouseAvailsTx(ctx, tx, tenantID, skuID, total)
	if err != nil {
		return nil, err
	}
	if warehouseID != nil && *warehouseID != uuid.Nil {
		for _, a := range avails {
			if a.Warehouse.ID == *warehouseID {
				if !a.Warehouse.Enabled {
					return nil, ErrWarehouseDisabled
				}
				if a.Avail < qty && !allowNeg {
					return nil, ErrInsufficientWarehouse
				}
				return []deductAllocation{{Warehouse: a.Warehouse, Quantity: qty}}, nil
			}
		}
		return nil, ErrWarehouseNotFound
	}
	remaining := qty
	allocs := make([]deductAllocation, 0, 2)
	var defWh *Warehouse
	for i := range avails {
		if avails[i].Warehouse.IsDefault {
			defWh = &avails[i].Warehouse
		}
	}
	for _, a := range avails {
		if remaining <= 0 {
			break
		}
		if !a.Warehouse.Enabled || a.Avail <= 0 {
			continue
		}
		take := a.Avail
		if take > remaining {
			take = remaining
		}
		allocs = append(allocs, deductAllocation{Warehouse: a.Warehouse, Quantity: take})
		remaining -= take
	}
	if remaining > 0 {
		if defWh == nil {
			return nil, fmt.Errorf("inventory: default warehouse missing for tenant %d", tenantID)
		}
		merged := false
		for i := range allocs {
			if allocs[i].Warehouse.ID == defWh.ID {
				allocs[i].Quantity += remaining
				merged = true
				break
			}
		}
		if !merged {
			allocs = append(allocs, deductAllocation{Warehouse: *defWh, Quantity: remaining})
		}
	}
	return allocs, nil
}

// applyWarehouseDeductionTx decrements non-default warehouse rows for each
// allocation (the default warehouse is derived from the SKU total, which the
// caller updates). Runs inside the caller's transaction.
func (s *Service) applyWarehouseDeductionTx(tx *gorm.DB, tenantID int64, productID, skuID uuid.UUID, allocs []deductAllocation, sign int) error {
	for _, a := range allocs {
		if a.Warehouse.IsDefault {
			continue
		}
		wh := a.Warehouse
		if _, _, err := s.addWarehouseStockTx(tx, tenantID, &wh, productID, skuID, sign*a.Quantity, 0); err != nil {
			return err
		}
	}
	return nil
}
