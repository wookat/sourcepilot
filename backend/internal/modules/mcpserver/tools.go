package mcpserver

import (
	"context"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/trademind-ai/trademind/backend/internal/modules/readonlyquery"
)

// The tool input/output shapes are shared with the Open API entry so both
// surfaces stay in sync; the aliases keep the historical mcpserver names.
type (
	// OrdersQueryIn filters orders_query.
	OrdersQueryIn = readonlyquery.OrdersQueryIn
	// OrderSummary is one masked order row.
	OrderSummary = readonlyquery.OrderSummary
	// OrdersQueryOut is the orders_query payload.
	OrdersQueryOut = readonlyquery.OrdersQueryOut
	// InventoryQueryIn filters inventory_query.
	InventoryQueryIn = readonlyquery.InventoryQueryIn
	// InventoryItem is one SKU stock row.
	InventoryItem = readonlyquery.InventoryItem
	// InventoryQueryOut is the inventory_query payload.
	InventoryQueryOut = readonlyquery.InventoryQueryOut
	// ReportSummaryIn bounds report_summary.
	ReportSummaryIn = readonlyquery.ReportSummaryIn
	// CurrencySales is paid sales aggregated per currency (no FX conversion).
	CurrencySales = readonlyquery.CurrencySales
	// ReportSummaryOut is the report_summary payload.
	ReportSummaryOut = readonlyquery.ReportSummaryOut
	// ExceptionsPendingIn filters exceptions_pending.
	ExceptionsPendingIn = readonlyquery.ExceptionsPendingIn
	// ExceptionItem is one masked pending exception row.
	ExceptionItem = readonlyquery.ExceptionItem
	// ExceptionsPendingOut is the exceptions_pending payload.
	ExceptionsPendingOut = readonlyquery.ExceptionsPendingOut
)

// queries builds the shared read-only query service for one request.
func (d *Deps) queries() *readonlyquery.Service {
	return &readonlyquery.Service{DB: d.DB, Exceptions: d.Exceptions}
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

func ordersQueryTool(d *Deps, tenantID int64) mcp.ToolHandlerFor[OrdersQueryIn, OrdersQueryOut] {
	return func(ctx context.Context, _ *mcp.CallToolRequest, in OrdersQueryIn) (*mcp.CallToolResult, OrdersQueryOut, error) {
		out, err := d.queries().OrdersQuery(ctx, tenantID, in)
		return nil, out, err
	}
}

func inventoryQueryTool(d *Deps, tenantID int64) mcp.ToolHandlerFor[InventoryQueryIn, InventoryQueryOut] {
	return func(ctx context.Context, _ *mcp.CallToolRequest, in InventoryQueryIn) (*mcp.CallToolResult, InventoryQueryOut, error) {
		out, err := d.queries().InventoryQuery(ctx, tenantID, in)
		return nil, out, err
	}
}

func reportSummaryTool(d *Deps, tenantID int64) mcp.ToolHandlerFor[ReportSummaryIn, ReportSummaryOut] {
	return func(ctx context.Context, _ *mcp.CallToolRequest, in ReportSummaryIn) (*mcp.CallToolResult, ReportSummaryOut, error) {
		out, err := d.queries().ReportSummary(ctx, tenantID, in)
		return nil, out, err
	}
}

func exceptionsPendingTool(d *Deps, tenantID int64) mcp.ToolHandlerFor[ExceptionsPendingIn, ExceptionsPendingOut] {
	return func(ctx context.Context, _ *mcp.CallToolRequest, in ExceptionsPendingIn) (*mcp.CallToolResult, ExceptionsPendingOut, error) {
		out, err := d.queries().ExceptionsPending(ctx, tenantID, in)
		return nil, out, err
	}
}
