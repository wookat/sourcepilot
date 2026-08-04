package reports

import (
	"context"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/trademind-ai/trademind/backend/internal/modules/inventory"
	"github.com/trademind-ai/trademind/backend/internal/pkg/adminperm"
)

const (
	slowDaysDefault   = 30
	slowDaysMax       = 180
	turnoverWindow    = 30 // outbound observation window (days) for turnover
	inventoryMaxLists = 100
)

// InventorySKURow is one SKU line in the slow-moving / low-stock lists.
// UnitCostCNY is the reference purchase price (primary source SKU mapping
// price, falling back to the collected cost price); nil when neither exists.
type InventorySKURow struct {
	ProductID      string     `json:"productId"`
	SKUID          string     `json:"skuId"`
	Title          string     `json:"title"`
	SKUName        string     `json:"skuName,omitempty"`
	SKUCode        string     `json:"skuCode,omitempty"`
	Stock          int        `json:"stock"`
	WarningStock   int        `json:"warningStock"`
	SafetyStock    int        `json:"safetyStock"`
	UnitCostCNY    *float64   `json:"unitCostCny,omitempty"`
	StockValueCNY  *float64   `json:"stockValueCny,omitempty"`
	LastOutboundAt *time.Time `json:"lastOutboundAt,omitempty"`
}

// InventorySummary aggregates the tenant's local SKU stock. StockValueCNY
// only sums SKUs with a reference purchase price (UnvaluedSKUCount lists the
// rest). TurnoverDays = current total stock / average daily outbound over
// the last 30 days (nil when there was no outbound).
type InventorySummary struct {
	SKUCount         int64    `json:"skuCount"`
	TotalStock       int64    `json:"totalStock"`
	StockValueCNY    float64  `json:"stockValueCny"`
	ValuedSKUCount   int64    `json:"valuedSkuCount"`
	UnvaluedSKUCount int64    `json:"unvaluedSkuCount"`
	LowStockCount    int64    `json:"lowStockCount"`
	OutOfStockCount  int64    `json:"outOfStockCount"`
	SlowMovingCount  int64    `json:"slowMovingCount"`
	AvgDailyOutbound *float64 `json:"avgDailyOutbound,omitempty"`
	TurnoverDays     *float64 `json:"turnoverDays,omitempty"`
}

// InventoryReportDTO is GET /reports/inventory.
type InventoryReportDTO struct {
	GeneratedAt string            `json:"generatedAt"`
	SlowDays    int               `json:"slowDays"`
	Currency    string            `json:"currency"`
	Summary     InventorySummary  `json:"summary"`
	SlowMoving  []InventorySKURow `json:"slowMoving"`
	LowStock    []InventorySKURow `json:"lowStock"`
}

type invSKUScan struct {
	SKUID        uuid.UUID `gorm:"column:sku_id"`
	ProductID    uuid.UUID `gorm:"column:product_id"`
	Title        string    `gorm:"column:title"`
	SKUName      string    `gorm:"column:sku_name"`
	SKUCode      string    `gorm:"column:sku_code"`
	Stock        *int      `gorm:"column:stock"`
	WarningStock int       `gorm:"column:warning_stock"`
	SafetyStock  int       `gorm:"column:safety_stock"`
	CostPrice    *float64  `gorm:"column:cost_price"`
}

// InventoryReport aggregates local SKU stock for the current tenant: stock
// value at reference purchase price, turnover days, slow-moving SKUs (no
// outbound in slowDays) and the low-stock list driven by the existing
// per-SKU warning thresholds.
func (s *Service) InventoryReport(c *gin.Context, slowDays int) (*InventoryReportDTO, error) {
	if slowDays <= 0 {
		slowDays = slowDaysDefault
	}
	if slowDays > slowDaysMax {
		slowDays = slowDaysMax
	}
	ctx := c.Request.Context()

	tenantID, err := adminperm.TenantIDFromGin(c)
	if err != nil {
		tenantID = 0
	}

	// SKUs joined to products (tenant scope). Reference purchase prices are
	// resolved separately from the primary source SKU mappings.
	var rows []invSKUScan
	if err := s.DB.WithContext(ctx).Raw(`
SELECT sk.id AS sku_id, sk.product_id, p.title, sk.sku_name, sk.sku_code,
       sk.stock, sk.warning_stock, sk.safety_stock, sk.cost_price
FROM product_skus sk
JOIN products p ON p.id = sk.product_id AND p.deleted_at IS NULL
WHERE p.tenant_id = ?`, tenantID).Scan(&rows).Error; err != nil {
		return nil, err
	}

	sourcePrices, err := s.primarySourcePrices(ctx, tenantID)
	if err != nil {
		return nil, err
	}

	skuIDs := make([]uuid.UUID, 0, len(rows))
	for _, r := range rows {
		skuIDs = append(skuIDs, r.SKUID)
	}
	lastOut, outboundQty, err := s.outboundStats(c, tenantID, skuIDs)
	if err != nil {
		return nil, err
	}

	now := time.Now()
	slowBefore := now.AddDate(0, 0, -slowDays)
	out := &InventoryReportDTO{
		GeneratedAt: now.UTC().Format(time.RFC3339),
		SlowDays:    slowDays,
		Currency:    "CNY",
		SlowMoving:  []InventorySKURow{},
		LowStock:    []InventorySKURow{},
	}

	var stockValue float64
	for _, r := range rows {
		stock := 0
		if r.Stock != nil {
			stock = *r.Stock
		}
		out.Summary.SKUCount++
		out.Summary.TotalStock += int64(stock)

		unitCost := sourcePrices[r.SKUID]
		if unitCost == nil {
			unitCost = r.CostPrice
		}
		var valueCNY *float64
		if unitCost != nil {
			out.Summary.ValuedSKUCount++
			v := round2(*unitCost * float64(stock))
			valueCNY = &v
			stockValue += v
		} else {
			out.Summary.UnvaluedSKUCount++
		}

		mk := func() InventorySKURow {
			row := InventorySKURow{
				ProductID:     r.ProductID.String(),
				SKUID:         r.SKUID.String(),
				Title:         r.Title,
				SKUName:       r.SKUName,
				SKUCode:       r.SKUCode,
				Stock:         stock,
				WarningStock:  r.WarningStock,
				SafetyStock:   r.SafetyStock,
				UnitCostCNY:   unitCost,
				StockValueCNY: valueCNY,
			}
			if t, ok := lastOut[r.SKUID]; ok {
				lt := t
				row.LastOutboundAt = &lt
			}
			return row
		}

		if stock <= 0 {
			out.Summary.OutOfStockCount++
		} else if stock <= r.WarningStock {
			out.Summary.LowStockCount++
		}
		if stock <= r.WarningStock {
			if len(out.LowStock) < inventoryMaxLists {
				out.LowStock = append(out.LowStock, mk())
			}
		}

		if stock > 0 {
			t, ok := lastOut[r.SKUID]
			if !ok || t.Before(slowBefore) {
				out.Summary.SlowMovingCount++
				if len(out.SlowMoving) < inventoryMaxLists {
					out.SlowMoving = append(out.SlowMoving, mk())
				}
			}
		}
	}
	out.Summary.StockValueCNY = round2(stockValue)

	if outboundQty > 0 {
		avg := round2(float64(outboundQty) / float64(turnoverWindow))
		out.Summary.AvgDailyOutbound = &avg
		if avg > 0 {
			td := round2(float64(out.Summary.TotalStock) / avg)
			out.Summary.TurnoverDays = &td
		}
	}
	return out, nil
}

// primarySourcePrices resolves the reference purchase price (CNY) per local
// SKU from the primary product source's SKU mappings, keeping the first
// mapping per SKU (lowest source id then mapping id, same ordering as cost
// estimates).
func (s *Service) primarySourcePrices(ctx context.Context, tenantID int64) (map[uuid.UUID]*float64, error) {
	type priceRow struct {
		LocalSKUID   uuid.UUID `gorm:"column:local_sku_id"`
		CurrentPrice float64   `gorm:"column:current_price"`
	}
	var rows []priceRow
	if err := s.DB.WithContext(ctx).Raw(`
SELECT pss.local_sku_id, pss.current_price
FROM product_sources ps
JOIN product_source_skus pss ON pss.product_source_id = ps.id
WHERE ps.tenant_id = ? AND ps.is_primary = ? AND ps.status <> 'disabled'
  AND pss.current_price IS NOT NULL
ORDER BY ps.id, pss.id`, tenantID, true).Scan(&rows).Error; err != nil {
		return nil, err
	}
	out := make(map[uuid.UUID]*float64, len(rows))
	for i := range rows {
		r := rows[i]
		if _, ok := out[r.LocalSKUID]; ok {
			continue
		}
		p := r.CurrentPrice
		out[r.LocalSKUID] = &p
	}
	return out, nil
}

// outboundStats returns each SKU's most recent outbound time (any horizon)
// and the total outbound quantity over the turnover window. Outbound =
// negative-delta stock changes from order deductions or manual adjustments.
func (s *Service) outboundStats(c *gin.Context, tenantID int64, skuIDs []uuid.UUID) (map[uuid.UUID]time.Time, int64, error) {
	lastOut := map[uuid.UUID]time.Time{}
	if len(skuIDs) == 0 {
		return lastOut, 0, nil
	}
	ctx := c.Request.Context()

	type lastRow struct {
		ProductSKUID uuid.UUID `gorm:"column:product_sku_id"`
		At           string    `gorm:"column:at"`
	}
	const chunk = 500
	for i := 0; i < len(skuIDs); i += chunk {
		j := i + chunk
		if j > len(skuIDs) {
			j = len(skuIDs)
		}
		var rows []lastRow
		if err := s.DB.WithContext(ctx).Model(&inventory.InventoryChangeLog{}).
			Select("product_sku_id, MAX(created_at) AS at").
			Where("tenant_id = ? AND product_sku_id IN ? AND delta < 0", tenantID, skuIDs[i:j]).
			Group("product_sku_id").Scan(&rows).Error; err != nil {
			return nil, 0, err
		}
		for _, r := range rows {
			if t, ok := parseDBTime(r.At); ok {
				lastOut[r.ProductSKUID] = t
			}
		}
	}

	since := time.Now().AddDate(0, 0, -turnoverWindow)
	var total struct {
		Qty *int64 `gorm:"column:qty"`
	}
	if err := s.DB.WithContext(ctx).Model(&inventory.InventoryChangeLog{}).
		Select("SUM(-delta) AS qty").
		Where("tenant_id = ? AND delta < 0 AND created_at >= ?", tenantID, since).
		Scan(&total).Error; err != nil {
		return nil, 0, err
	}
	qty := int64(0)
	if total.Qty != nil {
		qty = *total.Qty
	}
	return lastOut, qty, nil
}
