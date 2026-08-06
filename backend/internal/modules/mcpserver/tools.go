package mcpserver

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/trademind-ai/trademind/backend/internal/modules/order"
	"github.com/trademind-ai/trademind/backend/internal/modules/orderexception"
	"gorm.io/gorm"
)

const maxPageSize = 100

func normPage(page, pageSize int) (int, int) {
	if page < 1 {
		page = 1
	}
	if pageSize < 1 {
		pageSize = 20
	}
	if pageSize > maxPageSize {
		pageSize = maxPageSize
	}
	return page, pageSize
}

// maskName keeps the first rune of a customer name and hides the rest.
func maskName(name string) string {
	r := []rune(strings.TrimSpace(name))
	if len(r) == 0 {
		return ""
	}
	return string(r[0]) + "**"
}

func parseDate(s string) (*time.Time, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return nil, nil
	}
	t, err := time.Parse("2006-01-02", s)
	if err != nil {
		return nil, fmt.Errorf("invalid date %q (want YYYY-MM-DD)", s)
	}
	return &t, nil
}

// registerTools mounts the read-only tools bound to one tenant. No tool may
// mutate state; outputs exclude secrets, credentials, contact details and
// internal UUIDs.
func registerTools(srv *mcp.Server, d *Deps, tenantID int64) {
	mcp.AddTool(srv, &mcp.Tool{
		Name:        "orders_query",
		Description: "查询当前租户的订单列表（只读）。支持按状态、支付状态、平台、关键词与创建日期范围过滤，返回脱敏后的订单摘要。",
	}, ordersQueryTool(d, tenantID))
	mcp.AddTool(srv, &mcp.Tool{
		Name:        "inventory_query",
		Description: "查询当前租户的 SKU 库存（只读）。支持关键词与仅看低库存过滤，返回 SKU 编码、名称、库存与预警线。",
	}, inventoryQueryTool(d, tenantID))
	mcp.AddTool(srv, &mcp.Tool{
		Name:        "report_summary",
		Description: "查询当前租户的经营摘要（只读）：时间范围内订单量、已支付订单量、按币种的已支付销售额，以及当前未处理异常数与低库存 SKU 数。金额为订单原币种合计，未做汇率折算。",
	}, reportSummaryTool(d, tenantID))
	mcp.AddTool(srv, &mcp.Tool{
		Name:        "exceptions_pending",
		Description: "查询当前租户的未处理异常待办（只读）：SKU 未匹配、库存不足、同步失败等，返回异常类型、级别、关联订单号与建议动作。",
	}, exceptionsPendingTool(d, tenantID))
}

// OrdersQueryIn filters orders_query.
type OrdersQueryIn struct {
	Status        string `json:"status,omitempty" jsonschema:"订单状态过滤（如 pending/completed/cancelled）"`
	PaymentStatus string `json:"paymentStatus,omitempty" jsonschema:"支付状态过滤（unpaid/paid/refunded）"`
	Platform      string `json:"platform,omitempty" jsonschema:"平台过滤（如 douyin）"`
	Keyword       string `json:"keyword,omitempty" jsonschema:"按订单号模糊匹配"`
	StartDate     string `json:"startDate,omitempty" jsonschema:"创建时间起（YYYY-MM-DD，含当日）"`
	EndDate       string `json:"endDate,omitempty" jsonschema:"创建时间止（YYYY-MM-DD，含当日）"`
	Page          int    `json:"page,omitempty" jsonschema:"页码，默认 1"`
	PageSize      int    `json:"pageSize,omitempty" jsonschema:"每页条数，默认 20，最大 100"`
}

// OrderSummary is one masked order row.
type OrderSummary struct {
	OrderNo           string  `json:"orderNo"`
	Platform          string  `json:"platform"`
	Status            string  `json:"status"`
	PaymentStatus     string  `json:"paymentStatus"`
	FulfillmentStatus string  `json:"fulfillmentStatus"`
	Currency          string  `json:"currency"`
	TotalAmount       float64 `json:"totalAmount"`
	CustomerName      string  `json:"customerName,omitempty"`
	CreatedAt         string  `json:"createdAt"`
}

// OrdersQueryOut is the orders_query payload.
type OrdersQueryOut struct {
	Total int64          `json:"total"`
	Items []OrderSummary `json:"items"`
}

func ordersQueryTool(d *Deps, tenantID int64) mcp.ToolHandlerFor[OrdersQueryIn, OrdersQueryOut] {
	return func(ctx context.Context, _ *mcp.CallToolRequest, in OrdersQueryIn) (*mcp.CallToolResult, OrdersQueryOut, error) {
		var out OrdersQueryOut
		page, ps := normPage(in.Page, in.PageSize)
		start, err := parseDate(in.StartDate)
		if err != nil {
			return nil, out, err
		}
		end, err := parseDate(in.EndDate)
		if err != nil {
			return nil, out, err
		}
		tx := d.DB.WithContext(ctx).Model(&order.Order{}).Where("tenant_id = ?", tenantID)
		if v := strings.TrimSpace(in.Status); v != "" {
			tx = tx.Where("status = ?", v)
		}
		if v := strings.TrimSpace(in.PaymentStatus); v != "" {
			tx = tx.Where("payment_status = ?", v)
		}
		if v := strings.TrimSpace(in.Platform); v != "" {
			tx = tx.Where("platform = ?", v)
		}
		if v := strings.TrimSpace(in.Keyword); v != "" {
			tx = tx.Where("order_no ILIKE ?", "%"+v+"%")
		}
		if start != nil {
			tx = tx.Where("created_at >= ?", *start)
		}
		if end != nil {
			tx = tx.Where("created_at < ?", end.Add(24*time.Hour))
		}
		if err := tx.Count(&out.Total).Error; err != nil {
			return nil, out, fmt.Errorf("orders_query: %w", err)
		}
		var rows []order.Order
		if err := tx.Order("created_at DESC").Offset((page - 1) * ps).Limit(ps).Find(&rows).Error; err != nil {
			return nil, out, fmt.Errorf("orders_query: %w", err)
		}
		out.Items = make([]OrderSummary, 0, len(rows))
		for _, r := range rows {
			out.Items = append(out.Items, OrderSummary{
				OrderNo:           r.OrderNo,
				Platform:          r.Platform,
				Status:            r.Status,
				PaymentStatus:     r.PaymentStatus,
				FulfillmentStatus: r.FulfillmentStatus,
				Currency:          r.Currency,
				TotalAmount:       r.TotalAmount,
				CustomerName:      maskName(r.CustomerName),
				CreatedAt:         r.CreatedAt.UTC().Format(time.RFC3339),
			})
		}
		return nil, out, nil
	}
}

// InventoryQueryIn filters inventory_query.
type InventoryQueryIn struct {
	Keyword      string `json:"keyword,omitempty" jsonschema:"按 SKU 编码 / SKU 名称 / 商品标题模糊匹配"`
	LowStockOnly bool   `json:"lowStockOnly,omitempty" jsonschema:"仅返回库存不高于预警线的 SKU"`
	Page         int    `json:"page,omitempty" jsonschema:"页码，默认 1"`
	PageSize     int    `json:"pageSize,omitempty" jsonschema:"每页条数，默认 20，最大 100"`
}

// InventoryItem is one SKU stock row.
type InventoryItem struct {
	SKUCode      string `json:"skuCode"`
	SKUName      string `json:"skuName,omitempty"`
	ProductTitle string `json:"productTitle"`
	Stock        *int   `json:"stock,omitempty"`
	WarningStock int    `json:"warningStock"`
	StockStatus  string `json:"stockStatus,omitempty"`
}

// InventoryQueryOut is the inventory_query payload.
type InventoryQueryOut struct {
	Total int64           `json:"total"`
	Items []InventoryItem `json:"items"`
}

type skuRow struct {
	SKUCode      string
	SKUName      string
	ProductTitle string
	Stock        *int
	WarningStock int
	StockStatus  string
}

func inventoryQueryTool(d *Deps, tenantID int64) mcp.ToolHandlerFor[InventoryQueryIn, InventoryQueryOut] {
	return func(ctx context.Context, _ *mcp.CallToolRequest, in InventoryQueryIn) (*mcp.CallToolResult, InventoryQueryOut, error) {
		var out InventoryQueryOut
		page, ps := normPage(in.Page, in.PageSize)
		tx := d.DB.WithContext(ctx).Table("product_skus").
			Joins("JOIN products ON products.id = product_skus.product_id AND products.deleted_at IS NULL").
			Where("products.tenant_id = ?", tenantID)
		if v := strings.TrimSpace(in.Keyword); v != "" {
			like := "%" + v + "%"
			tx = tx.Where("product_skus.sku_code ILIKE ? OR product_skus.sku_name ILIKE ? OR products.title ILIKE ?", like, like, like)
		}
		if in.LowStockOnly {
			tx = tx.Where("product_skus.stock IS NOT NULL AND product_skus.stock <= product_skus.warning_stock")
		}
		if err := tx.Count(&out.Total).Error; err != nil {
			return nil, out, fmt.Errorf("inventory_query: %w", err)
		}
		var rows []skuRow
		if err := tx.Select("product_skus.sku_code AS sku_code, product_skus.sku_name AS sku_name, products.title AS product_title, product_skus.stock AS stock, product_skus.warning_stock AS warning_stock, product_skus.stock_status AS stock_status").
			Order("product_skus.created_at DESC").
			Offset((page - 1) * ps).Limit(ps).
			Scan(&rows).Error; err != nil {
			return nil, out, fmt.Errorf("inventory_query: %w", err)
		}
		out.Items = make([]InventoryItem, 0, len(rows))
		for _, r := range rows {
			out.Items = append(out.Items, InventoryItem{
				SKUCode:      r.SKUCode,
				SKUName:      r.SKUName,
				ProductTitle: r.ProductTitle,
				Stock:        r.Stock,
				WarningStock: r.WarningStock,
				StockStatus:  r.StockStatus,
			})
		}
		return nil, out, nil
	}
}

// ReportSummaryIn bounds report_summary.
type ReportSummaryIn struct {
	StartDate string `json:"startDate,omitempty" jsonschema:"统计起始日（YYYY-MM-DD，含当日；默认近 30 天）"`
	EndDate   string `json:"endDate,omitempty" jsonschema:"统计截止日（YYYY-MM-DD，含当日；默认今天）"`
}

// CurrencySales is paid sales aggregated per currency (no FX conversion).
type CurrencySales struct {
	Currency   string  `json:"currency"`
	PaidAmount float64 `json:"paidAmount"`
	OrderCount int64   `json:"orderCount"`
}

// ReportSummaryOut is the report_summary payload.
type ReportSummaryOut struct {
	StartDate          string          `json:"startDate"`
	EndDate            string          `json:"endDate"`
	OrderCount         int64           `json:"orderCount"`
	PaidOrderCount     int64           `json:"paidOrderCount"`
	SalesByCurrency    []CurrencySales `json:"salesByCurrency"`
	OpenExceptionCount int64           `json:"openExceptionCount"`
	LowStockSKUCount   int64           `json:"lowStockSkuCount"`
}

func reportSummaryTool(d *Deps, tenantID int64) mcp.ToolHandlerFor[ReportSummaryIn, ReportSummaryOut] {
	return func(ctx context.Context, _ *mcp.CallToolRequest, in ReportSummaryIn) (*mcp.CallToolResult, ReportSummaryOut, error) {
		var out ReportSummaryOut
		start, err := parseDate(in.StartDate)
		if err != nil {
			return nil, out, err
		}
		end, err := parseDate(in.EndDate)
		if err != nil {
			return nil, out, err
		}
		now := time.Now().UTC().Truncate(24 * time.Hour)
		if end == nil {
			end = &now
		}
		if start == nil {
			s := end.Add(-29 * 24 * time.Hour)
			start = &s
		}
		endExcl := end.Add(24 * time.Hour)
		out.StartDate = start.Format("2006-01-02")
		out.EndDate = end.Format("2006-01-02")

		base := func() *gorm.DB {
			return d.DB.WithContext(ctx).Model(&order.Order{}).
				Where("tenant_id = ?", tenantID).
				Where("created_at >= ? AND created_at < ?", *start, endExcl)
		}
		if err := base().Count(&out.OrderCount).Error; err != nil {
			return nil, out, fmt.Errorf("report_summary: %w", err)
		}
		if err := base().Where("payment_status = ?", order.PaymentPaid).Count(&out.PaidOrderCount).Error; err != nil {
			return nil, out, fmt.Errorf("report_summary: %w", err)
		}
		var sales []CurrencySales
		if err := base().Where("payment_status = ?", order.PaymentPaid).
			Select("currency AS currency, COALESCE(SUM(total_amount),0) AS paid_amount, COUNT(*) AS order_count").
			Group("currency").Order("currency").
			Scan(&sales).Error; err != nil {
			return nil, out, fmt.Errorf("report_summary: %w", err)
		}
		out.SalesByCurrency = sales
		if out.SalesByCurrency == nil {
			out.SalesByCurrency = []CurrencySales{}
		}

		if d.Exceptions != nil {
			res, err := d.Exceptions.ListOrderExceptions(ctx, orderexception.ListOrderExceptionsRequest{
				TenantID: &tenantID,
				Page:     1,
				PageSize: 1,
			})
			if err != nil {
				return nil, out, fmt.Errorf("report_summary: exceptions: %w", err)
			}
			out.OpenExceptionCount = res.Summary.TotalOpen
		}

		if err := d.DB.WithContext(ctx).Table("product_skus").
			Joins("JOIN products ON products.id = product_skus.product_id AND products.deleted_at IS NULL").
			Where("products.tenant_id = ?", tenantID).
			Where("product_skus.stock IS NOT NULL AND product_skus.stock <= product_skus.warning_stock").
			Count(&out.LowStockSKUCount).Error; err != nil {
			return nil, out, fmt.Errorf("report_summary: %w", err)
		}
		return nil, out, nil
	}
}

// ExceptionsPendingIn filters exceptions_pending.
type ExceptionsPendingIn struct {
	ExceptionType string `json:"exceptionType,omitempty" jsonschema:"异常类型过滤（如 sku_unmatched/insufficient_stock）"`
	Severity      string `json:"severity,omitempty" jsonschema:"级别过滤（error/warning）"`
	Page          int    `json:"page,omitempty" jsonschema:"页码，默认 1"`
	PageSize      int    `json:"pageSize,omitempty" jsonschema:"每页条数，默认 20，最大 100"`
}

// ExceptionItem is one masked pending exception row.
type ExceptionItem struct {
	ExceptionType   string `json:"exceptionType"`
	Severity        string `json:"severity"`
	Status          string `json:"status"`
	OrderNo         string `json:"orderNo,omitempty"`
	Platform        string `json:"platform,omitempty"`
	ShopName        string `json:"shopName,omitempty"`
	SKUCode         string `json:"skuCode,omitempty"`
	ProductTitle    string `json:"productTitle,omitempty"`
	Quantity        int    `json:"quantity,omitempty"`
	ErrorMessage    string `json:"errorMessage,omitempty"`
	SuggestedAction string `json:"suggestedAction,omitempty"`
	CreatedAt       string `json:"createdAt"`
}

// ExceptionsPendingOut is the exceptions_pending payload.
type ExceptionsPendingOut struct {
	Total     int64           `json:"total"`
	TotalOpen int64           `json:"totalOpen"`
	Items     []ExceptionItem `json:"items"`
}

func exceptionsPendingTool(d *Deps, tenantID int64) mcp.ToolHandlerFor[ExceptionsPendingIn, ExceptionsPendingOut] {
	return func(ctx context.Context, _ *mcp.CallToolRequest, in ExceptionsPendingIn) (*mcp.CallToolResult, ExceptionsPendingOut, error) {
		var out ExceptionsPendingOut
		if d.Exceptions == nil {
			return nil, out, fmt.Errorf("exceptions_pending: unavailable")
		}
		page, ps := normPage(in.Page, in.PageSize)
		res, err := d.Exceptions.ListOrderExceptions(ctx, orderexception.ListOrderExceptionsRequest{
			ExceptionType: strings.TrimSpace(in.ExceptionType),
			Severity:      strings.TrimSpace(in.Severity),
			Page:          page,
			PageSize:      ps,
			TenantID:      &tenantID,
		})
		if err != nil {
			return nil, out, fmt.Errorf("exceptions_pending: %w", err)
		}
		out.Total = res.Total
		out.TotalOpen = res.Summary.TotalOpen
		out.Items = make([]ExceptionItem, 0, len(res.List))
		for _, r := range res.List {
			out.Items = append(out.Items, ExceptionItem{
				ExceptionType:   r.ExceptionType,
				Severity:        r.Severity,
				Status:          r.Status,
				OrderNo:         r.OrderNo,
				Platform:        r.Platform,
				ShopName:        r.ShopName,
				SKUCode:         r.SKUCode,
				ProductTitle:    r.ProductTitle,
				Quantity:        r.Quantity,
				ErrorMessage:    r.ErrorMessage,
				SuggestedAction: r.SuggestedAction,
				CreatedAt:       r.CreatedAt.UTC().Format(time.RFC3339),
			})
		}
		return nil, out, nil
	}
}
