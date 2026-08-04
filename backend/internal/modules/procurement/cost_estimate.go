package procurement

import (
	"context"
	"errors"
	"math"
	"math/big"
	"strconv"
	"strings"

	"github.com/google/uuid"
	"github.com/trademind-ai/trademind/backend/internal/modules/order"
	"github.com/trademind-ai/trademind/backend/internal/modules/sourcing"
	"github.com/trademind-ai/trademind/backend/internal/providers/fxrate"
	"gorm.io/gorm"
)

// CostEstimateLine is one sales-order line's procurement cost reference.
type CostEstimateLine struct {
	OrderItemID  string   `json:"orderItemId"`
	LocalSKUID   string   `json:"localSkuId,omitempty"`
	SKUName      string   `json:"skuName"`
	Quantity     int      `json:"quantity"`
	SupplierName string   `json:"supplierName,omitempty"`
	UnitCostCNY  *float64 `json:"unitCostCny,omitempty"`
	LineCostCNY  *float64 `json:"lineCostCny,omitempty"`
	IssueCode    string   `json:"issueCode,omitempty"`
	IssueMessage string   `json:"issueMessage,omitempty"`
}

// OrderCostEstimateDTO is GET /procurement/cost-estimates/:id (id = sales order).
// Costs are reference procurement prices in CNY; gross profit is only
// computed when the order currency is CNY, the report currency manual rate
// table (settings group report_currency) can convert CNY→order currency, or
// settings.pricing.exchangeRate (CNY → order currency) is configured.
type OrderCostEstimateDTO struct {
	OrderID          string             `json:"orderId"`
	OrderNo          string             `json:"orderNo"`
	Currency         string             `json:"currency"`
	TotalAmount      float64            `json:"totalAmount"`
	EstimatedCostCNY float64            `json:"estimatedCostCny"`
	ExchangeRate     *float64           `json:"exchangeRate,omitempty"`
	EstimatedCost    *float64           `json:"estimatedCost,omitempty"`
	GrossProfit      *float64           `json:"grossProfit,omitempty"`
	MarginPercent    *float64           `json:"marginPercent,omitempty"`
	MissingLines     int                `json:"missingLines"`
	Lines            []CostEstimateLine `json:"lines"`
}

// CostEstimateSummary is one order's compact cost/profit summary for list views.
type CostEstimateSummary struct {
	OrderID          string   `json:"orderId"`
	Currency         string   `json:"currency"`
	TotalAmount      float64  `json:"totalAmount"`
	EstimatedCostCNY float64  `json:"estimatedCostCny"`
	ExchangeRate     *float64 `json:"exchangeRate,omitempty"`
	EstimatedCost    *float64 `json:"estimatedCost,omitempty"`
	GrossProfit      *float64 `json:"grossProfit,omitempty"`
	MarginPercent    *float64 `json:"marginPercent,omitempty"`
	MissingLines     int      `json:"missingLines"`
}

// MaxBatchEstimateOrders caps POST /procurement/cost-estimates/batch.
const MaxBatchEstimateOrders = 50

// EstimateOrderCostBatch estimates several orders at once (for list views);
// orders that no longer exist are omitted from the result map. All related
// rows are loaded with a fixed number of batched queries.
func (s *Service) EstimateOrderCostBatch(ctx context.Context, ids []uuid.UUID) (map[string]CostEstimateSummary, error) {
	out := make(map[string]CostEstimateSummary, len(ids))
	seen := make(map[uuid.UUID]struct{}, len(ids))
	uniq := make([]uuid.UUID, 0, len(ids))
	for _, id := range ids {
		if _, done := seen[id]; done {
			continue
		}
		seen[id] = struct{}{}
		uniq = append(uniq, id)
	}
	if len(uniq) == 0 {
		return out, nil
	}

	var orders []order.Order
	if err := s.DB.WithContext(ctx).Where("id IN ?", uniq).Find(&orders).Error; err != nil {
		return nil, err
	}
	if len(orders) == 0 {
		return out, nil
	}
	orderIDs := make([]uuid.UUID, 0, len(orders))
	for _, o := range orders {
		orderIDs = append(orderIDs, o.ID)
	}
	var items []order.OrderItem
	if err := s.DB.WithContext(ctx).Where("order_id IN ?", orderIDs).Find(&items).Error; err != nil {
		return nil, err
	}
	itemsByOrder := make(map[uuid.UUID][]order.OrderItem, len(orders))
	for _, it := range items {
		itemsByOrder[it.OrderID] = append(itemsByOrder[it.OrderID], it)
	}

	costs, err := s.resolveLineCostBatch(ctx, items)
	if err != nil {
		return nil, err
	}

	type rateKey struct {
		tenantID int64
		currency string
	}
	type rateVal struct {
		rate float64
		ok   bool
	}
	rates := map[rateKey]rateVal{}
	for _, o := range orders {
		k := rateKey{tenantID: o.TenantID, currency: strings.ToUpper(strings.TrimSpace(o.Currency))}
		rv, cached := rates[k]
		if !cached {
			rv.rate, rv.ok = s.resolveExchangeRate(ctx, o.TenantID, o.Currency)
			rates[k] = rv
		}
		est := buildOrderCostEstimate(o, itemsByOrder[o.ID], costs, rv.rate, rv.ok)
		out[est.OrderID] = CostEstimateSummary{
			OrderID:          est.OrderID,
			Currency:         est.Currency,
			TotalAmount:      est.TotalAmount,
			EstimatedCostCNY: est.EstimatedCostCNY,
			ExchangeRate:     est.ExchangeRate,
			EstimatedCost:    est.EstimatedCost,
			GrossProfit:      est.GrossProfit,
			MarginPercent:    est.MarginPercent,
			MissingLines:     est.MissingLines,
		}
	}
	return out, nil
}

// SettingsReader decouples procurement from the settings module implementation.
type SettingsReader interface {
	PlainByGroup(ctx context.Context, tenantID int64, groupKey string) (map[string]string, error)
}

// EstimateOrderCost estimates one sales order's procurement cost from the
// primary source SKU mapping reference price (falling back to the latest
// captured price history), mirroring purchase-order generation pricing.
func (s *Service) EstimateOrderCost(ctx context.Context, orderID uuid.UUID) (*OrderCostEstimateDTO, error) {
	var o order.Order
	if err := s.DB.WithContext(ctx).First(&o, "id = ?", orderID).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	var items []order.OrderItem
	if err := s.DB.WithContext(ctx).Where("order_id = ?", orderID).Find(&items).Error; err != nil {
		return nil, err
	}

	costs := make(map[uuid.UUID]resolvedLineCost, len(items))
	for _, it := range items {
		unit, supplierName, issueCode, issueMsg, err := s.resolveLineCost(ctx, it)
		if err != nil {
			return nil, err
		}
		costs[it.ID] = resolvedLineCost{
			unit:         unit,
			supplierName: supplierName,
			issueCode:    issueCode,
			issueMessage: issueMsg,
		}
	}
	rate, hasRate := s.resolveExchangeRate(ctx, o.TenantID, o.Currency)
	return buildOrderCostEstimate(o, items, costs, rate, hasRate), nil
}

// resolvedLineCost is one order line's reference unit cost or pricing issue.
type resolvedLineCost struct {
	unit         *float64
	supplierName string
	issueCode    string
	issueMessage string
}

// buildOrderCostEstimate assembles the estimate DTO from pre-resolved line costs.
func buildOrderCostEstimate(o order.Order, items []order.OrderItem, costs map[uuid.UUID]resolvedLineCost, rate float64, hasRate bool) *OrderCostEstimateDTO {
	out := &OrderCostEstimateDTO{
		OrderID:     o.ID.String(),
		OrderNo:     o.OrderNo,
		Currency:    o.Currency,
		TotalAmount: o.TotalAmount,
		Lines:       []CostEstimateLine{},
	}
	for _, it := range items {
		line := CostEstimateLine{
			OrderItemID: it.ID.String(),
			SKUName:     it.SKUName,
			Quantity:    it.Quantity,
		}
		if it.ProductSKUID != nil {
			line.LocalSKUID = it.ProductSKUID.String()
		}
		c := costs[it.ID]
		line.SupplierName = c.supplierName
		if c.issueCode != "" {
			line.IssueCode = c.issueCode
			line.IssueMessage = c.issueMessage
			out.MissingLines++
		} else {
			u := roundMoney2(*c.unit)
			lc := roundMoney2(u * float64(it.Quantity))
			line.UnitCostCNY = &u
			line.LineCostCNY = &lc
			out.EstimatedCostCNY += lc
		}
		out.Lines = append(out.Lines, line)
	}
	out.EstimatedCostCNY = roundMoney2(out.EstimatedCostCNY)

	if hasRate {
		out.ExchangeRate = &rate
		cost := roundMoney2(out.EstimatedCostCNY * rate)
		out.EstimatedCost = &cost
		if out.MissingLines == 0 && len(items) > 0 {
			profit := roundMoney2(o.TotalAmount - cost)
			out.GrossProfit = &profit
			if o.TotalAmount > 0 {
				margin := roundMoney2(profit / o.TotalAmount * 100)
				out.MarginPercent = &margin
			}
		}
	}
	return out
}

// resolveLineCostBatch mirrors resolveLineCost for many lines using batched
// IN queries plus one windowed latest-price query, keyed by order item id.
func (s *Service) resolveLineCostBatch(ctx context.Context, items []order.OrderItem) (map[uuid.UUID]resolvedLineCost, error) {
	out := make(map[uuid.UUID]resolvedLineCost, len(items))
	productIDSet := map[uuid.UUID]struct{}{}
	productIDs := []uuid.UUID{}
	for _, it := range items {
		if it.ProductSKUID == nil || it.ProductID == nil {
			out[it.ID] = resolvedLineCost{issueCode: "sku.unmatched", issueMessage: "订单行未匹配本地 SKU"}
			continue
		}
		if _, ok := productIDSet[*it.ProductID]; !ok {
			productIDSet[*it.ProductID] = struct{}{}
			productIDs = append(productIDs, *it.ProductID)
		}
	}
	if len(productIDs) == 0 {
		return out, nil
	}

	// Primary source per product: same ordering as First (primary key asc).
	var sources []sourcing.ProductSource
	if err := s.DB.WithContext(ctx).Preload("Supplier").
		Where("product_id IN ? AND is_primary = TRUE AND status <> ?", productIDs, sourcing.SourceStatusDisabled).
		Order("id").Find(&sources).Error; err != nil {
		return nil, err
	}
	primaryByProduct := make(map[uuid.UUID]*sourcing.ProductSource, len(sources))
	sourceIDs := []uuid.UUID{}
	for i := range sources {
		src := &sources[i]
		if _, ok := primaryByProduct[src.ProductID]; ok {
			continue
		}
		primaryByProduct[src.ProductID] = src
		sourceIDs = append(sourceIDs, src.ID)
	}

	// SKU mappings: same ordering as First; keep the first row per
	// (source, local SKU) pair.
	type mapKey struct {
		sourceID uuid.UUID
		localSKU uuid.UUID
	}
	mappingByPair := map[mapKey]*sourcing.ProductSourceSKU{}
	if len(sourceIDs) > 0 {
		localSKUSet := map[uuid.UUID]struct{}{}
		localSKUs := []uuid.UUID{}
		for _, it := range items {
			if it.ProductSKUID == nil || it.ProductID == nil {
				continue
			}
			if _, ok := localSKUSet[*it.ProductSKUID]; !ok {
				localSKUSet[*it.ProductSKUID] = struct{}{}
				localSKUs = append(localSKUs, *it.ProductSKUID)
			}
		}
		var mappings []sourcing.ProductSourceSKU
		if err := s.DB.WithContext(ctx).
			Where("product_source_id IN ? AND local_sku_id IN ?", sourceIDs, localSKUs).
			Order("id").Find(&mappings).Error; err != nil {
			return nil, err
		}
		for i := range mappings {
			m := &mappings[i]
			k := mapKey{sourceID: m.ProductSourceID, localSKU: m.LocalSKUID}
			if _, ok := mappingByPair[k]; !ok {
				mappingByPair[k] = m
			}
		}
	}

	// Latest captured price per mapping that has no current price.
	histIDs := []uuid.UUID{}
	for _, m := range mappingByPair {
		if m.CurrentPrice == nil {
			histIDs = append(histIDs, m.ID)
		}
	}
	latestPrice := map[uuid.UUID]float64{}
	if len(histIDs) > 0 {
		type histRow struct {
			SourceSKUID uuid.UUID `gorm:"column:source_sku_id"`
			Price       float64   `gorm:"column:price"`
		}
		var rows []histRow
		if err := s.DB.WithContext(ctx).Raw(`
SELECT source_sku_id, price FROM (
  SELECT source_sku_id, price,
         ROW_NUMBER() OVER (PARTITION BY source_sku_id ORDER BY captured_at DESC) AS rn
  FROM source_price_history
  WHERE source_sku_id IN ?
) t WHERE rn = 1`, histIDs).Scan(&rows).Error; err != nil {
			return nil, err
		}
		for _, r := range rows {
			latestPrice[r.SourceSKUID] = r.Price
		}
	}

	for _, it := range items {
		if it.ProductSKUID == nil || it.ProductID == nil {
			continue
		}
		primary, ok := primaryByProduct[*it.ProductID]
		if !ok {
			out[it.ID] = resolvedLineCost{issueCode: "source.missing", issueMessage: "商品没有主货源"}
			continue
		}
		supplierName := ""
		if primary.Supplier != nil {
			supplierName = primary.Supplier.Name
		}
		mapping, ok := mappingByPair[mapKey{sourceID: primary.ID, localSKU: *it.ProductSKUID}]
		if !ok {
			out[it.ID] = resolvedLineCost{supplierName: supplierName, issueCode: "mapping.missing", issueMessage: "主货源缺少该 SKU 的映射"}
			continue
		}
		expected := mapping.CurrentPrice
		if expected == nil {
			if p, ok := latestPrice[mapping.ID]; ok {
				expected = &p
			}
		}
		if expected == nil {
			out[it.ID] = resolvedLineCost{supplierName: supplierName, issueCode: "price.missing", issueMessage: "SKU 缺少参考进价"}
			continue
		}
		out[it.ID] = resolvedLineCost{unit: expected, supplierName: supplierName}
	}
	return out, nil
}

// resolveLineCost returns the reference unit cost (CNY) for one order line,
// or an issue code when the line cannot be priced yet.
func (s *Service) resolveLineCost(ctx context.Context, it order.OrderItem) (*float64, string, string, string, error) {
	if it.ProductSKUID == nil || it.ProductID == nil {
		return nil, "", "sku.unmatched", "订单行未匹配本地 SKU", nil
	}
	var primary sourcing.ProductSource
	err := s.DB.WithContext(ctx).Preload("Supplier").
		Where("product_id = ? AND is_primary = TRUE AND status <> ?", *it.ProductID, sourcing.SourceStatusDisabled).
		First(&primary).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, "", "source.missing", "商品没有主货源", nil
		}
		return nil, "", "", "", err
	}
	supplierName := ""
	if primary.Supplier != nil {
		supplierName = primary.Supplier.Name
	}
	var mapping sourcing.ProductSourceSKU
	err = s.DB.WithContext(ctx).
		Where("product_source_id = ? AND local_sku_id = ?", primary.ID, *it.ProductSKUID).
		First(&mapping).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, supplierName, "mapping.missing", "主货源缺少该 SKU 的映射", nil
		}
		return nil, "", "", "", err
	}
	expected := mapping.CurrentPrice
	if expected == nil {
		var hist sourcing.SourcePriceHistory
		if err := s.DB.WithContext(ctx).
			Where("source_sku_id = ?", mapping.ID).
			Order("captured_at DESC").
			First(&hist).Error; err == nil {
			p := hist.Price
			expected = &p
		}
	}
	if expected == nil {
		return nil, supplierName, "price.missing", "SKU 缺少参考进价", nil
	}
	return expected, supplierName, "", "", nil
}

// resolveExchangeRate returns the CNY→order-currency rate: 1 for CNY orders.
// Other currencies resolve through the report currency manual rate table
// first (same rates the sales reports use: CNY→currency =
// rate(CNY→base) / rate(currency→base)), then fall back to the legacy
// single settings.pricing.exchangeRate (false when neither is configured).
func (s *Service) resolveExchangeRate(ctx context.Context, tenantID int64, currency string) (float64, bool) {
	if strings.EqualFold(strings.TrimSpace(currency), "CNY") {
		return 1, true
	}
	if s.Settings == nil {
		return 0, false
	}
	if rate, ok := s.reportTableRate(ctx, tenantID, currency); ok {
		return rate, true
	}
	// Tenant-scoped settings first, then tenant 0 (global defaults, where the
	// pricing settings page writes).
	tenants := []int64{tenantID}
	if tenantID != 0 {
		tenants = append(tenants, 0)
	}
	for _, tid := range tenants {
		m, err := s.Settings.PlainByGroup(ctx, tid, "pricing")
		if err != nil {
			continue
		}
		// default_exchange_rate is the key the pricing settings page writes.
		for _, k := range []string{"exchangeRate", "default_exchange_rate"} {
			if v := strings.TrimSpace(m[k]); v != "" {
				if f, err := strconv.ParseFloat(v, 64); err == nil && f > 0 {
					return f, true
				}
			}
		}
	}
	return 0, false
}

// reportTableRate derives the CNY→currency rate from the report currency
// manual rate table so cost estimates share the report conversion口径. Both
// legs (CNY→base and currency→base) must be resolvable.
func (s *Service) reportTableRate(ctx context.Context, tenantID int64, currency string) (float64, bool) {
	p := &fxrate.ManualProvider{Settings: s.Settings}
	tab, err := p.Table(ctx, tenantID)
	if err != nil || tab == nil {
		return 0, false
	}
	rCur, ok := tab.Rate(currency)
	if !ok || rCur.Sign() <= 0 {
		return 0, false
	}
	rCNY, ok := tab.Rate("CNY")
	if !ok || rCNY.Sign() <= 0 {
		return 0, false
	}
	f, _ := new(big.Rat).Quo(rCNY, rCur).Float64()
	if f <= 0 {
		return 0, false
	}
	return f, true
}

func roundMoney2(v float64) float64 {
	return math.Round(v*100) / 100
}
