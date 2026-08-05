package inventory

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/google/uuid"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

var (
	ErrWarehouseNotFound       = errors.New("warehouse not found")
	ErrWarehouseCodeConflict   = errors.New("warehouse code already exists")
	ErrDefaultWarehouseLocked  = errors.New("default warehouse cannot be deleted or disabled")
	ErrWarehouseHasStock       = errors.New("warehouse still holds stock")
	ErrWarehouseDisabled       = errors.New("warehouse is disabled")
	ErrInsufficientWarehouse   = errors.New("insufficient stock in source warehouse")
	ErrTransferSameWarehouse   = errors.New("source and target warehouse must differ")
	ErrTransferInvalidQuantity = errors.New("transfer quantity must be positive")
	ErrDefaultMustBeEnabled    = errors.New("disabled warehouse cannot become the default")
)

// EnsureDefaultWarehouse returns the tenant's default warehouse, creating it
// idempotently when missing (legacy tenants migrated at startup are covered too).
func (s *Service) EnsureDefaultWarehouse(ctx context.Context, tenantID int64) (*Warehouse, error) {
	if s == nil || s.DB == nil {
		return nil, fmt.Errorf("inventory: no db")
	}
	var w Warehouse
	err := s.DB.WithContext(ctx).
		Where("tenant_id = ? AND is_default = ?", tenantID, true).
		First(&w).Error
	if err == nil {
		return &w, nil
	}
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, err
	}
	w = Warehouse{
		TenantID:  tenantID,
		Code:      DefaultWarehouseCode,
		Name:      DefaultWarehouseName,
		IsDefault: true,
		Enabled:   true,
		Priority:  0,
	}
	if err := s.DB.WithContext(ctx).Create(&w).Error; err != nil {
		// Concurrent creation: re-read.
		var again Warehouse
		if err2 := s.DB.WithContext(ctx).
			Where("tenant_id = ? AND is_default = ?", tenantID, true).
			First(&again).Error; err2 == nil {
			return &again, nil
		}
		return nil, err
	}
	return &w, nil
}

// ListWarehouses returns the tenant's warehouses (default first, then priority asc).
func (s *Service) ListWarehouses(ctx context.Context, tenantID int64) ([]Warehouse, error) {
	if _, err := s.EnsureDefaultWarehouse(ctx, tenantID); err != nil {
		return nil, err
	}
	var rows []Warehouse
	err := s.DB.WithContext(ctx).
		Where("tenant_id = ?", tenantID).
		Order("is_default DESC, priority ASC, created_at ASC").
		Find(&rows).Error
	return rows, err
}

// GetWarehouse loads one warehouse with tenant scope (cross-tenant → not found).
func (s *Service) GetWarehouse(ctx context.Context, tenantID int64, id uuid.UUID) (*Warehouse, error) {
	var w Warehouse
	err := s.DB.WithContext(ctx).
		Where("id = ? AND tenant_id = ?", id, tenantID).
		First(&w).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, ErrWarehouseNotFound
	}
	if err != nil {
		return nil, err
	}
	return &w, nil
}

// CreateWarehouseBody POST /inventory/warehouses
type CreateWarehouseBody struct {
	Code     string `json:"code"`
	Name     string `json:"name"`
	Priority int    `json:"priority"`
	Remark   string `json:"remark"`
}

// CreateWarehouse adds a non-default warehouse for the tenant.
func (s *Service) CreateWarehouse(ctx context.Context, tenantID int64, body CreateWarehouseBody) (*Warehouse, error) {
	if _, err := s.EnsureDefaultWarehouse(ctx, tenantID); err != nil {
		return nil, err
	}
	code := strings.TrimSpace(body.Code)
	name := strings.TrimSpace(body.Name)
	if name == "" {
		return nil, fmt.Errorf("warehouse name required")
	}
	if code == "" {
		code = strings.ToLower(uuid.NewString()[:8])
	}
	if strings.EqualFold(code, DefaultWarehouseCode) {
		return nil, ErrWarehouseCodeConflict
	}
	var n int64
	if err := s.DB.WithContext(ctx).Model(&Warehouse{}).
		Where("tenant_id = ? AND code = ?", tenantID, code).
		Count(&n).Error; err != nil {
		return nil, err
	}
	if n > 0 {
		return nil, ErrWarehouseCodeConflict
	}
	w := Warehouse{
		TenantID: tenantID,
		Code:     clampStr(code, 64),
		Name:     clampStr(name, 128),
		Enabled:  true,
		Priority: body.Priority,
		Remark:   clampStr(body.Remark, 255),
	}
	if err := s.DB.WithContext(ctx).Create(&w).Error; err != nil {
		return nil, err
	}
	return &w, nil
}

// UpdateWarehouseBody PUT /inventory/warehouses/:id
type UpdateWarehouseBody struct {
	Name     *string `json:"name"`
	Priority *int    `json:"priority"`
	Remark   *string `json:"remark"`
	Enabled  *bool   `json:"enabled"`
}

// UpdateWarehouse edits name / priority / remark / enabled. The default
// warehouse can be renamed but never disabled.
func (s *Service) UpdateWarehouse(ctx context.Context, tenantID int64, id uuid.UUID, body UpdateWarehouseBody) (*Warehouse, error) {
	w, err := s.GetWarehouse(ctx, tenantID, id)
	if err != nil {
		return nil, err
	}
	updates := map[string]any{}
	if body.Name != nil {
		name := strings.TrimSpace(*body.Name)
		if name == "" {
			return nil, fmt.Errorf("warehouse name required")
		}
		updates["name"] = clampStr(name, 128)
	}
	if body.Priority != nil {
		updates["priority"] = *body.Priority
	}
	if body.Remark != nil {
		updates["remark"] = clampStr(*body.Remark, 255)
	}
	if body.Enabled != nil {
		if w.IsDefault && !*body.Enabled {
			return nil, ErrDefaultWarehouseLocked
		}
		updates["enabled"] = *body.Enabled
	}
	if len(updates) == 0 {
		return w, nil
	}
	if err := s.DB.WithContext(ctx).Model(&Warehouse{}).
		Where("id = ? AND tenant_id = ?", id, tenantID).
		Updates(updates).Error; err != nil {
		return nil, err
	}
	return s.GetWarehouse(ctx, tenantID, id)
}

// DeleteWarehouse soft-deletes a non-default warehouse; it must hold zero stock
// (transfer stock back to another warehouse first).
func (s *Service) DeleteWarehouse(ctx context.Context, tenantID int64, id uuid.UUID) error {
	w, err := s.GetWarehouse(ctx, tenantID, id)
	if err != nil {
		return err
	}
	if w.IsDefault {
		return ErrDefaultWarehouseLocked
	}
	var qty struct {
		Total *int64 `gorm:"column:total"`
	}
	if err := s.DB.WithContext(ctx).Model(&WarehouseStock{}).
		Select("SUM(stock) AS total").
		Where("warehouse_id = ?", id).
		Scan(&qty).Error; err != nil {
		return err
	}
	if qty.Total != nil && *qty.Total != 0 {
		return ErrWarehouseHasStock
	}
	return s.DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("warehouse_id = ?", id).Delete(&WarehouseStock{}).Error; err != nil {
			return err
		}
		return tx.Where("id = ? AND tenant_id = ?", id, tenantID).Delete(&Warehouse{}).Error
	})
}

// SetDefaultWarehouse atomically moves the default flag to the target
// warehouse. Per-SKU quantities are preserved: the old default's derived
// stock is materialized into persisted rows, and the new default's rows are
// removed (its quantity becomes the derived remainder). Compatible with the
// partial unique index on (tenant_id) WHERE is_default (old flag cleared
// before the new one is set inside the same transaction).
func (s *Service) SetDefaultWarehouse(ctx context.Context, tenantID int64, id uuid.UUID) (*Warehouse, error) {
	target, err := s.GetWarehouse(ctx, tenantID, id)
	if err != nil {
		return nil, err
	}
	if target.IsDefault {
		return target, nil
	}
	if !target.Enabled {
		return nil, ErrDefaultMustBeEnabled
	}
	err = s.DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var old Warehouse
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
			Where("tenant_id = ? AND is_default = ?", tenantID, true).
			First(&old).Error; err != nil {
			return err
		}
		var tgt Warehouse
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
			Where("id = ? AND tenant_id = ?", id, tenantID).
			First(&tgt).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return ErrWarehouseNotFound
			}
			return err
		}
		if tgt.IsDefault {
			return nil
		}
		if !tgt.Enabled {
			return ErrDefaultMustBeEnabled
		}
		// Materialize the old default's derived per-SKU stock as persisted rows.
		type derivedRow struct {
			SKUID     uuid.UUID `gorm:"column:sku_id"`
			ProductID uuid.UUID `gorm:"column:product_id"`
			Derived   int       `gorm:"column:derived"`
		}
		var derived []derivedRow
		if err := tx.Raw(`
SELECT sk.id AS sku_id, sk.product_id AS product_id,
       COALESCE(sk.stock,0) - COALESCE((
         SELECT SUM(ws.stock) FROM warehouse_stocks ws
         JOIN warehouses w ON w.id = ws.warehouse_id AND w.deleted_at IS NULL AND w.is_default = FALSE
         WHERE ws.product_sku_id = sk.id
       ), 0) AS derived
FROM product_skus sk
JOIN products p ON p.id = sk.product_id AND p.deleted_at IS NULL
WHERE p.tenant_id = ?`, tenantID).Scan(&derived).Error; err != nil {
			return err
		}
		if err := tx.Where("warehouse_id = ?", old.ID).Delete(&WarehouseStock{}).Error; err != nil {
			return err
		}
		rows := make([]WarehouseStock, 0, len(derived))
		for _, d := range derived {
			if d.Derived == 0 {
				continue
			}
			rows = append(rows, WarehouseStock{
				TenantID:     tenantID,
				WarehouseID:  old.ID,
				ProductID:    d.ProductID,
				ProductSKUID: d.SKUID,
				Stock:        d.Derived,
			})
		}
		if len(rows) > 0 {
			if err := tx.CreateInBatches(rows, 200).Error; err != nil {
				return err
			}
		}
		// The new default becomes derived: drop its persisted rows.
		if err := tx.Where("warehouse_id = ?", tgt.ID).Delete(&WarehouseStock{}).Error; err != nil {
			return err
		}
		if err := tx.Model(&Warehouse{}).
			Where("id = ? AND tenant_id = ?", old.ID, tenantID).
			Update("is_default", false).Error; err != nil {
			return err
		}
		return tx.Model(&Warehouse{}).
			Where("id = ? AND tenant_id = ?", tgt.ID, tenantID).
			Update("is_default", true).Error
	})
	if err != nil {
		return nil, err
	}
	return s.GetWarehouse(ctx, tenantID, id)
}

// skuTotalStockScoped returns the SKU's total stock with tenant scope
// (cross-tenant / unknown SKU → ErrWarehouseNotFound-style 404 semantics).
func (s *Service) skuTotalStockScoped(ctx context.Context, tenantID int64, skuID uuid.UUID) (int, error) {
	var row struct {
		Stock *int `gorm:"column:stock"`
	}
	res := s.DB.WithContext(ctx).Raw(`
SELECT sk.stock FROM product_skus sk
JOIN products p ON p.id = sk.product_id AND p.deleted_at IS NULL
WHERE sk.id = ? AND p.tenant_id = ?`, skuID, tenantID).Scan(&row)
	if res.Error != nil {
		return 0, res.Error
	}
	if res.RowsAffected == 0 {
		return 0, ErrWarehouseNotFound
	}
	if row.Stock == nil {
		return 0, nil
	}
	return *row.Stock, nil
}

// nonDefaultStockSum returns SUM(stock) of non-default warehouse rows for one SKU.
func nonDefaultStockSum(tx *gorm.DB, skuID uuid.UUID) (int, error) {
	var out struct {
		Total *int64 `gorm:"column:total"`
	}
	err := tx.Model(&WarehouseStock{}).
		Select("SUM(warehouse_stocks.stock) AS total").
		Joins("JOIN warehouses w ON w.id = warehouse_stocks.warehouse_id AND w.deleted_at IS NULL").
		Where("warehouse_stocks.product_sku_id = ? AND w.is_default = ?", skuID, false).
		Scan(&out).Error
	if err != nil {
		return 0, err
	}
	if out.Total == nil {
		return 0, nil
	}
	return int(*out.Total), nil
}

// WarehouseStocksForSKU returns the SKU's full per-warehouse breakdown, with
// the default warehouse quantity derived from the SKU total.
func (s *Service) WarehouseStocksForSKU(ctx context.Context, tenantID int64, skuID uuid.UUID, totalStock int) ([]WarehouseStockEntry, error) {
	whs, err := s.ListWarehouses(ctx, tenantID)
	if err != nil {
		return nil, err
	}
	byWh := map[uuid.UUID]int{}
	var rows []WarehouseStock
	if err := s.DB.WithContext(ctx).
		Where("tenant_id = ? AND product_sku_id = ?", tenantID, skuID).
		Find(&rows).Error; err != nil {
		return nil, err
	}
	othersSum := 0
	for _, r := range rows {
		byWh[r.WarehouseID] = r.Stock
	}
	out := make([]WarehouseStockEntry, 0, len(whs))
	for _, w := range whs {
		if !w.IsDefault {
			othersSum += byWh[w.ID]
		}
	}
	for _, w := range whs {
		st := byWh[w.ID]
		if w.IsDefault {
			st = totalStock - othersSum
		}
		out = append(out, WarehouseStockEntry{
			WarehouseID:   w.ID,
			WarehouseName: w.Name,
			IsDefault:     w.IsDefault,
			Enabled:       w.Enabled,
			Stock:         st,
		})
	}
	return out, nil
}

// warehouseStockBreakdownBatch resolves per-warehouse stock for many SKUs at
// once (default warehouse derived). totals maps skuID → total stock.
func (s *Service) warehouseStockBreakdownBatch(ctx context.Context, tenantID int64, totals map[uuid.UUID]int) (map[uuid.UUID][]WarehouseStockEntry, error) {
	out := map[uuid.UUID][]WarehouseStockEntry{}
	if len(totals) == 0 {
		return out, nil
	}
	whs, err := s.ListWarehouses(ctx, tenantID)
	if err != nil {
		return nil, err
	}
	skuIDs := make([]uuid.UUID, 0, len(totals))
	for id := range totals {
		skuIDs = append(skuIDs, id)
	}
	var rows []WarehouseStock
	if err := s.DB.WithContext(ctx).
		Where("tenant_id = ? AND product_sku_id IN ?", tenantID, skuIDs).
		Find(&rows).Error; err != nil {
		return nil, err
	}
	bySku := map[uuid.UUID]map[uuid.UUID]int{}
	for _, r := range rows {
		m, ok := bySku[r.ProductSKUID]
		if !ok {
			m = map[uuid.UUID]int{}
			bySku[r.ProductSKUID] = m
		}
		m[r.WarehouseID] = r.Stock
	}
	for skuID, total := range totals {
		m := bySku[skuID]
		othersSum := 0
		for _, w := range whs {
			if !w.IsDefault {
				othersSum += m[w.ID]
			}
		}
		entries := make([]WarehouseStockEntry, 0, len(whs))
		for _, w := range whs {
			st := m[w.ID]
			if w.IsDefault {
				st = total - othersSum
			}
			entries = append(entries, WarehouseStockEntry{
				WarehouseID:   w.ID,
				WarehouseName: w.Name,
				IsDefault:     w.IsDefault,
				Enabled:       w.Enabled,
				Stock:         st,
			})
		}
		out[skuID] = entries
	}
	return out, nil
}

// lockWarehouseStockRow loads (FOR UPDATE) or creates the non-default
// warehouse stock row for one SKU inside a transaction.
func lockWarehouseStockRow(tx *gorm.DB, tenantID int64, wh *Warehouse, productID, skuID uuid.UUID) (*WarehouseStock, error) {
	var row WarehouseStock
	err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
		Where("warehouse_id = ? AND product_sku_id = ?", wh.ID, skuID).
		First(&row).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		row = WarehouseStock{
			TenantID:     tenantID,
			WarehouseID:  wh.ID,
			ProductID:    productID,
			ProductSKUID: skuID,
			Stock:        0,
		}
		if err := tx.Create(&row).Error; err != nil {
			return nil, err
		}
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
			Where("id = ?", row.ID).First(&row).Error; err != nil {
			return nil, err
		}
		return &row, nil
	}
	if err != nil {
		return nil, err
	}
	return &row, nil
}

// addWarehouseStockTx applies a delta to one warehouse's SKU stock inside tx.
// The default warehouse is derived, so only non-default warehouses persist rows.
// Returns before/after quantities of that warehouse line.
func (s *Service) addWarehouseStockTx(tx *gorm.DB, tenantID int64, wh *Warehouse, productID, skuID uuid.UUID, delta int, totalBefore int) (before int, after int, err error) {
	if wh == nil {
		return 0, 0, ErrWarehouseNotFound
	}
	if wh.IsDefault {
		others, err := nonDefaultStockSum(tx, skuID)
		if err != nil {
			return 0, 0, err
		}
		before = totalBefore - others
		return before, before + delta, nil
	}
	row, err := lockWarehouseStockRow(tx, tenantID, wh, productID, skuID)
	if err != nil {
		return 0, 0, err
	}
	before = row.Stock
	after = before + delta
	if err := tx.Model(&WarehouseStock{}).Where("id = ?", row.ID).
		Update("stock", after).Error; err != nil {
		return 0, 0, err
	}
	return before, after, nil
}

// WarehouseSummaryEntry aggregates one warehouse for GET /inventory/warehouses/summary.
type WarehouseSummaryEntry struct {
	WarehouseID   uuid.UUID `json:"warehouseId"`
	WarehouseName string    `json:"warehouseName"`
	Code          string    `json:"code"`
	IsDefault     bool      `json:"isDefault"`
	Enabled       bool      `json:"enabled"`
	Priority      int       `json:"priority"`
	TotalStock    int64     `json:"totalStock"`
	SKUCount      int64     `json:"skuCount"`
}

// WarehouseSummary returns per-warehouse totals for the tenant. The default
// warehouse totals are derived: tenant total minus non-default sums.
func (s *Service) WarehouseSummary(ctx context.Context, tenantID int64) ([]WarehouseSummaryEntry, error) {
	whs, err := s.ListWarehouses(ctx, tenantID)
	if err != nil {
		return nil, err
	}
	var totalRow struct {
		Total    *int64 `gorm:"column:total"`
		SKUCount int64  `gorm:"column:sku_count"`
	}
	if err := s.DB.WithContext(ctx).Raw(`
SELECT SUM(COALESCE(sk.stock,0)) AS total, COUNT(sk.id) AS sku_count
FROM product_skus sk
JOIN products p ON p.id = sk.product_id AND p.deleted_at IS NULL
WHERE p.tenant_id = ?`, tenantID).Scan(&totalRow).Error; err != nil {
		return nil, err
	}
	type whAgg struct {
		WarehouseID uuid.UUID `gorm:"column:warehouse_id"`
		Total       *int64    `gorm:"column:total"`
		SKUCount    int64     `gorm:"column:sku_count"`
	}
	var aggs []whAgg
	if err := s.DB.WithContext(ctx).Model(&WarehouseStock{}).
		Select("warehouse_id, SUM(stock) AS total, COUNT(*) AS sku_count").
		Where("tenant_id = ?", tenantID).
		Group("warehouse_id").
		Scan(&aggs).Error; err != nil {
		return nil, err
	}
	byWh := map[uuid.UUID]whAgg{}
	var othersTotal int64
	whByID := map[uuid.UUID]Warehouse{}
	for _, w := range whs {
		whByID[w.ID] = w
	}
	for _, a := range aggs {
		byWh[a.WarehouseID] = a
		if w, ok := whByID[a.WarehouseID]; ok && !w.IsDefault && a.Total != nil {
			othersTotal += *a.Total
		}
	}
	tenantTotal := int64(0)
	if totalRow.Total != nil {
		tenantTotal = *totalRow.Total
	}
	out := make([]WarehouseSummaryEntry, 0, len(whs))
	for _, w := range whs {
		e := WarehouseSummaryEntry{
			WarehouseID:   w.ID,
			WarehouseName: w.Name,
			Code:          w.Code,
			IsDefault:     w.IsDefault,
			Enabled:       w.Enabled,
			Priority:      w.Priority,
		}
		if w.IsDefault {
			e.TotalStock = tenantTotal - othersTotal
			e.SKUCount = totalRow.SKUCount
		} else if a, ok := byWh[w.ID]; ok {
			if a.Total != nil {
				e.TotalStock = *a.Total
			}
			e.SKUCount = a.SKUCount
		}
		out = append(out, e)
	}
	return out, nil
}

// WarehouseMigrationPreview reports what the legacy-stock migration means for
// the tenant before/after startup migration: which SKUs belong to the default
// warehouse and whether any invariant is broken (zero-loss precheck).
type WarehouseMigrationPreview struct {
	DefaultWarehouseExists bool   `json:"defaultWarehouseExists"`
	DefaultWarehouseID     string `json:"defaultWarehouseId,omitempty"`
	DefaultWarehouseName   string `json:"defaultWarehouseName,omitempty"`
	SKUCount               int64  `json:"skuCount"`
	TotalStock             int64  `json:"totalStock"`
	NonDefaultStock        int64  `json:"nonDefaultStock"`
	DefaultDerivedStock    int64  `json:"defaultDerivedStock"`
	OrphanWarehouseRows    int64  `json:"orphanWarehouseRows"`
	NegativeDerivedSKUs    int64  `json:"negativeDerivedSkus"`
	Consistent             bool   `json:"consistent"`
}

// PreviewWarehouseMigration runs the tenant-scoped migration precheck.
func (s *Service) PreviewWarehouseMigration(ctx context.Context, tenantID int64) (*WarehouseMigrationPreview, error) {
	out := &WarehouseMigrationPreview{}
	var def Warehouse
	err := s.DB.WithContext(ctx).
		Where("tenant_id = ? AND is_default = ?", tenantID, true).
		First(&def).Error
	if err == nil {
		out.DefaultWarehouseExists = true
		out.DefaultWarehouseID = def.ID.String()
		out.DefaultWarehouseName = def.Name
	} else if !errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, err
	}
	var totals struct {
		Total    *int64 `gorm:"column:total"`
		SKUCount int64  `gorm:"column:sku_count"`
	}
	if err := s.DB.WithContext(ctx).Raw(`
SELECT SUM(COALESCE(sk.stock,0)) AS total, COUNT(sk.id) AS sku_count
FROM product_skus sk
JOIN products p ON p.id = sk.product_id AND p.deleted_at IS NULL
WHERE p.tenant_id = ?`, tenantID).Scan(&totals).Error; err != nil {
		return nil, err
	}
	out.SKUCount = totals.SKUCount
	if totals.Total != nil {
		out.TotalStock = *totals.Total
	}
	var nonDef struct {
		Total *int64 `gorm:"column:total"`
	}
	if err := s.DB.WithContext(ctx).Raw(`
SELECT SUM(ws.stock) AS total
FROM warehouse_stocks ws
JOIN warehouses w ON w.id = ws.warehouse_id AND w.deleted_at IS NULL
WHERE ws.tenant_id = ? AND w.is_default = FALSE`, tenantID).Scan(&nonDef).Error; err != nil {
		return nil, err
	}
	if nonDef.Total != nil {
		out.NonDefaultStock = *nonDef.Total
	}
	out.DefaultDerivedStock = out.TotalStock - out.NonDefaultStock
	if err := s.DB.WithContext(ctx).Raw(`
SELECT COUNT(*) FROM warehouse_stocks ws
WHERE ws.tenant_id = ?
  AND NOT EXISTS (SELECT 1 FROM warehouses w WHERE w.id = ws.warehouse_id AND w.deleted_at IS NULL)`, tenantID).
		Scan(&out.OrphanWarehouseRows).Error; err != nil {
		return nil, err
	}
	if err := s.DB.WithContext(ctx).Raw(`
SELECT COUNT(*) FROM (
  SELECT ws.product_sku_id, SUM(ws.stock) AS others, MAX(COALESCE(sk.stock,0)) AS total
  FROM warehouse_stocks ws
  JOIN warehouses w ON w.id = ws.warehouse_id AND w.deleted_at IS NULL AND w.is_default = FALSE
  JOIN product_skus sk ON sk.id = ws.product_sku_id
  WHERE ws.tenant_id = ?
  GROUP BY ws.product_sku_id
  HAVING SUM(ws.stock) > MAX(COALESCE(sk.stock,0))
) x`, tenantID).Scan(&out.NegativeDerivedSKUs).Error; err != nil {
		return nil, err
	}
	out.Consistent = out.OrphanWarehouseRows == 0 && out.NegativeDerivedSKUs == 0
	return out, nil
}
