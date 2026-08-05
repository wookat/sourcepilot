package inventory

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/google/uuid"
	"github.com/trademind-ai/trademind/backend/internal/modules/product"
)

// Warehouse plan strategies (自动分仓策略).
const (
	// WarehousePlanStrategyDefault assigns the tenant's default warehouse and
	// requires it to cover every line.
	WarehousePlanStrategyDefault = "default_warehouse"
	// WarehousePlanStrategyStockFirst picks the first warehouse (by deduction
	// priority) whose availability covers every line.
	WarehousePlanStrategyStockFirst = "stock_first"
)

// ValidWarehousePlanStrategies lists accepted strategy codes.
func ValidWarehousePlanStrategies() []string {
	return []string{WarehousePlanStrategyDefault, WarehousePlanStrategyStockFirst}
}

// ErrPlanInsufficientStock marks a plan failure caused by stock shortage (kept
// visible as a failed automation log, not silently skipped).
var ErrPlanInsufficientStock = errors.New("库存不足，无法分配发货仓")

// WarehouseDemand is one SKU quantity an order needs from the chosen warehouse.
type WarehouseDemand struct {
	ProductSKUID uuid.UUID
	SKUCode      string
	Quantity     int
}

// WarehousePlan is the chosen warehouse for one order.
type WarehousePlan struct {
	WarehouseID   uuid.UUID
	WarehouseName string
	IsDefault     bool
}

// PlanOrderWarehouse picks one warehouse able to fulfil every demand line
// under the given strategy. It is read-only (planning; deduction happens at
// the existing order deduct flows, which then pin to the assigned warehouse).
func (s *Service) PlanOrderWarehouse(ctx context.Context, tenantID int64, strategy string, demands []WarehouseDemand) (*WarehousePlan, error) {
	if s == nil || s.DB == nil {
		return nil, fmt.Errorf("inventory: no db")
	}
	if len(demands) == 0 {
		return nil, fmt.Errorf("inventory: no demand lines to plan")
	}
	strategy = strings.TrimSpace(strategy)
	if strategy == "" {
		strategy = WarehousePlanStrategyDefault
	}

	// avail[warehouseID][skuID]
	availBySKU := map[uuid.UUID][]warehouseAvail{}
	tx := s.DB.WithContext(ctx)
	for _, d := range demands {
		if _, ok := availBySKU[d.ProductSKUID]; ok {
			continue
		}
		// Tenant-scoped SKU load: never plan against another tenant's SKU rows.
		var sku product.ProductSKU
		if err := tx.
			Joins("JOIN products p ON p.id = product_skus.product_id AND p.deleted_at IS NULL").
			Where("product_skus.id = ? AND p.tenant_id = ?", d.ProductSKUID, tenantID).
			First(&sku).Error; err != nil {
			return nil, fmt.Errorf("inventory: load sku %s: %w", d.ProductSKUID, err)
		}
		avails, err := s.warehouseAvailsTx(ctx, tx, tenantID, d.ProductSKUID, derefStock(sku.Stock))
		if err != nil {
			return nil, err
		}
		availBySKU[d.ProductSKUID] = avails
	}

	availIn := func(whID uuid.UUID, skuID uuid.UUID) int {
		for _, a := range availBySKU[skuID] {
			if a.Warehouse.ID == whID {
				return a.Avail
			}
		}
		return 0
	}
	// shortages lists demand lines a warehouse cannot cover (for失败留痕文案).
	shortages := func(wh Warehouse) []string {
		var out []string
		seen := map[uuid.UUID]int{}
		for _, d := range demands {
			seen[d.ProductSKUID] += d.Quantity
		}
		for _, d := range demands {
			need, ok := seen[d.ProductSKUID]
			if !ok {
				continue
			}
			delete(seen, d.ProductSKUID)
			avail := availIn(wh.ID, d.ProductSKUID)
			if avail < need {
				label := strings.TrimSpace(d.SKUCode)
				if label == "" {
					label = d.ProductSKUID.String()
				}
				out = append(out, fmt.Sprintf("%s 需 %d 件仅 %d 件", label, need, avail))
			}
		}
		return out
	}

	// Candidate warehouses in deduction order (any SKU's avail list carries all warehouses).
	var candidates []Warehouse
	for _, a := range availBySKU[demands[0].ProductSKUID] {
		candidates = append(candidates, a.Warehouse)
	}

	switch strategy {
	case WarehousePlanStrategyDefault:
		for _, wh := range candidates {
			if !wh.IsDefault {
				continue
			}
			if sh := shortages(wh); len(sh) > 0 {
				return nil, fmt.Errorf("%w：%s（%s）", ErrPlanInsufficientStock, wh.Name, strings.Join(sh, "；"))
			}
			return &WarehousePlan{WarehouseID: wh.ID, WarehouseName: wh.Name, IsDefault: true}, nil
		}
		return nil, fmt.Errorf("inventory: default warehouse missing for tenant %d", tenantID)
	case WarehousePlanStrategyStockFirst:
		var firstShort []string
		var firstName string
		for _, wh := range candidates {
			if !wh.Enabled {
				continue
			}
			sh := shortages(wh)
			if len(sh) == 0 {
				return &WarehousePlan{WarehouseID: wh.ID, WarehouseName: wh.Name, IsDefault: wh.IsDefault}, nil
			}
			if firstName == "" {
				firstName, firstShort = wh.Name, sh
			}
		}
		if firstName == "" {
			return nil, fmt.Errorf("inventory: no enabled warehouse for tenant %d", tenantID)
		}
		return nil, fmt.Errorf("%w：所有仓库均无法整单覆盖（如 %s：%s）", ErrPlanInsufficientStock, firstName, strings.Join(firstShort, "；"))
	default:
		return nil, fmt.Errorf("inventory: unknown warehouse plan strategy %q", strategy)
	}
}
