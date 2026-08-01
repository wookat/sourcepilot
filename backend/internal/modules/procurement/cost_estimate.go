package procurement

import (
	"context"
	"errors"
	"math"
	"strconv"
	"strings"

	"github.com/google/uuid"
	"github.com/trademind-ai/trademind/backend/internal/modules/order"
	"github.com/trademind-ai/trademind/backend/internal/modules/sourcing"
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
// computed when the order currency is CNY or settings.pricing.exchangeRate
// (CNY → order currency) is configured.
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
		unit, supplierName, issueCode, issueMsg, err := s.resolveLineCost(ctx, it)
		if err != nil {
			return nil, err
		}
		line.SupplierName = supplierName
		if issueCode != "" {
			line.IssueCode = issueCode
			line.IssueMessage = issueMsg
			out.MissingLines++
		} else {
			u := roundMoney2(*unit)
			lc := roundMoney2(u * float64(it.Quantity))
			line.UnitCostCNY = &u
			line.LineCostCNY = &lc
			out.EstimatedCostCNY += lc
		}
		out.Lines = append(out.Lines, line)
	}
	out.EstimatedCostCNY = roundMoney2(out.EstimatedCostCNY)

	rate, hasRate := s.resolveExchangeRate(ctx, o.TenantID, o.Currency)
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

// resolveExchangeRate returns the CNY→order-currency rate: 1 for CNY orders,
// settings.pricing.exchangeRate otherwise (false when not configured).
func (s *Service) resolveExchangeRate(ctx context.Context, tenantID int64, currency string) (float64, bool) {
	if strings.EqualFold(strings.TrimSpace(currency), "CNY") {
		return 1, true
	}
	if s.Settings == nil {
		return 0, false
	}
	m, err := s.Settings.PlainByGroup(ctx, tenantID, "pricing")
	if err != nil {
		return 0, false
	}
	// default_exchange_rate is the key the pricing settings page writes.
	for _, k := range []string{"exchangeRate", "default_exchange_rate"} {
		if v := strings.TrimSpace(m[k]); v != "" {
			if f, err := strconv.ParseFloat(v, 64); err == nil && f > 0 {
				return f, true
			}
		}
	}
	return 0, false
}

func roundMoney2(v float64) float64 {
	return math.Round(v*100) / 100
}
