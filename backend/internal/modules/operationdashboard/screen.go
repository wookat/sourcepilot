package operationdashboard

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/trademind-ai/trademind/backend/internal/modules/inventory"
	"github.com/trademind-ai/trademind/backend/internal/modules/order"
	"github.com/trademind-ai/trademind/backend/internal/modules/taskcenter"
	"gorm.io/gorm"
)

const (
	screenFunnelDays  = 7
	screenTrendHours  = 24
	screenAlertsLimit = 20
)

// ScreenTodayDTO carries today's headline KPIs. Sales / profit figures are
// filled by the handler from the profit report (same #276 SQL 下推口径);
// nil means the base conversion is unavailable, never faked as 0.
type ScreenTodayDTO struct {
	OrderCount            int64            `json:"orderCount"`
	PaidOrderCount        int64            `json:"paidOrderCount"`
	SalesBase             float64          `json:"salesBase"`
	BaseCurrency          string           `json:"baseCurrency"`
	UnconvertedCurrencies []string         `json:"unconvertedCurrencies,omitempty"`
	UnconvertedRevenue    []ScreenMoneyDTO `json:"unconvertedRevenue,omitempty"`
	ConvertedCurrencies   []string         `json:"convertedCurrencies,omitempty"`
	GrossProfitBase       *float64         `json:"grossProfitBase,omitempty"`
	MarginPercent         *float64         `json:"marginPercent,omitempty"`
}

// ScreenMoneyDTO is one original-currency amount that could not be converted
// to the base currency (no manual rate): shown explicitly on the screen and
// never mixed into salesBase.
type ScreenMoneyDTO struct {
	Currency string  `json:"currency"`
	Amount   float64 `json:"amount"`
}

// ScreenTodoDTO is one of the five order-lifecycle todo counters.
type ScreenTodoDTO struct {
	Key      string `json:"key"`
	Title    string `json:"title"`
	Count    int64  `json:"count"`
	Priority string `json:"priority"`
	Link     string `json:"link"`
}

// ScreenFunnelStageDTO is one order status funnel stage (近 7 天创建的订单).
type ScreenFunnelStageDTO struct {
	Key   string `json:"key"`
	Title string `json:"title"`
	Count int64  `json:"count"`
}

// ScreenTrendPointDTO is one hourly bucket of the last-24h order trend.
type ScreenTrendPointDTO struct {
	Hour       string `json:"hour"` // RFC3339, local hour start
	OrderCount int64  `json:"orderCount"`
	PaidCount  int64  `json:"paidCount"`
}

// ScreenAlertDTO is one row in the scrolling exception / low-stock ticker.
type ScreenAlertDTO struct {
	Type       string     `json:"type"` // task_alert | low_stock | out_of_stock
	Severity   string     `json:"severity"`
	Title      string     `json:"title"`
	Detail     string     `json:"detail,omitempty"`
	Link       string     `json:"link"`
	OccurredAt *time.Time `json:"occurredAt,omitempty"`
}

// ScreenDTO is GET /api/v1/dashboard/screen: one read-only aggregate for the
// full-screen operations board. All queries are tenant / shop scoped.
type ScreenDTO struct {
	GeneratedAt string                 `json:"generatedAt"`
	FunnelDays  int                    `json:"funnelDays"`
	TrendHours  int                    `json:"trendHours"`
	Today       ScreenTodayDTO         `json:"today"`
	Todos       []ScreenTodoDTO        `json:"todos"`
	Funnel      []ScreenFunnelStageDTO `json:"funnel"`
	Trend       []ScreenTrendPointDTO  `json:"trend"`
	Alerts      []ScreenAlertDTO       `json:"alerts"`
	// Cards is the tenant's card layout (default layout when unset), so the
	// frontend renders order / visibility from the same aggregate response.
	Cards []ScreenCardDTO `json:"cards"`
}

// GetScreen aggregates the big-screen board. Each block is one grouped SQL
// query (no per-row N+1); today's sales / profit are attached by the handler.
// enabled (nil = all) lets the handler skip aggregation for cards the tenant
// disabled, so a trimmed layout is also cheaper, never slower.
func (s *Service) GetScreen(ctx context.Context, q Query, sc Scope, enabled map[string]bool) (*ScreenDTO, error) {
	if s == nil || s.DB == nil {
		return nil, fmt.Errorf("operationdashboard: no db")
	}
	on := func(key string) bool { return enabled == nil || enabled[key] }
	now := time.Now()
	out := &ScreenDTO{
		GeneratedAt: now.UTC().Format(time.RFC3339),
		FunnelDays:  screenFunnelDays,
		TrendHours:  screenTrendHours,
	}

	todayStart := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
	if on(CardKPIOrders) {
		todayTx := s.screenOrderScope(ctx, q).Where("created_at >= ?", todayStart)
		_ = todayTx.Count(&out.Today.OrderCount).Error
	}

	if on(CardFunnel) {
		out.Funnel = s.screenFunnel(ctx, q, todayStart.AddDate(0, 0, -(screenFunnelDays-1)))
	}
	if on(CardTrend) {
		out.Trend = s.screenTrend(ctx, q, now)
	}
	if on(CardTodos) {
		out.Todos = s.screenTodos(ctx, q)
	}
	if on(CardAlerts) || on(CardKPIAlerts) {
		out.Alerts = s.screenAlerts(ctx, q)
	}
	return out, nil
}

// screenOrderScope returns a tenant + shop scoped orders query.
func (s *Service) screenOrderScope(ctx context.Context, q Query) *gorm.DB {
	tx := q.Scope.applyTenantColumn(s.DB.WithContext(ctx).Model(&order.Order{}), "tenant_id")
	tx = q.Scope.applyShopColumn(tx, "shop_id")
	if v := strings.TrimSpace(q.ShopID); v != "" {
		tx = tx.Where("shop_id = ?", v)
	}
	if pl := strings.TrimSpace(q.Platform); pl != "" {
		tx = tx.Where("LOWER(platform) = ?", strings.ToLower(pl))
	}
	return tx
}

// screenFunnel counts the order lifecycle stages over orders created since
// the window start, in a single grouped scan-free SQL statement.
func (s *Service) screenFunnel(ctx context.Context, q Query, since time.Time) []ScreenFunnelStageDTO {
	var row struct {
		Total     int64 `gorm:"column:total"`
		Paid      int64 `gorm:"column:paid"`
		Procured  int64 `gorm:"column:procured"`
		Shipped   int64 `gorm:"column:shipped"`
		Delivered int64 `gorm:"column:delivered"`
	}
	_ = s.screenOrderScope(ctx, q).Where("created_at >= ?", since).
		Select(`COUNT(*) AS total,
			SUM(CASE WHEN payment_status = ? THEN 1 ELSE 0 END) AS paid,
			SUM(CASE WHEN payment_status = ? AND EXISTS (
				SELECT 1 FROM purchase_order_items poi
				JOIN purchase_orders po2 ON po2.id = poi.purchase_order_id
				WHERE poi.sales_order_id = orders.id
				AND po2.status NOT IN ('cancelled','failed','voided')
			) THEN 1 ELSE 0 END) AS procured,
			SUM(CASE WHEN status IN (?, ?) THEN 1 ELSE 0 END) AS shipped,
			SUM(CASE WHEN status = ? THEN 1 ELSE 0 END) AS delivered`,
			order.PaymentPaid, order.PaymentPaid,
			order.StatusShipped, order.StatusDelivered, order.StatusDelivered).
		Scan(&row).Error
	return []ScreenFunnelStageDTO{
		{Key: "created", Title: "新建订单", Count: row.Total},
		{Key: "paid", Title: "已付款", Count: row.Paid},
		{Key: "procured", Title: "已生成采购", Count: row.Procured},
		{Key: "shipped", Title: "已发货", Count: row.Shipped},
		{Key: "delivered", Title: "已送达", Count: row.Delivered},
	}
}

// screenTrend buckets the last 24 hours of orders by local hour in one
// grouped SQL query, then fills empty buckets in Go.
func (s *Service) screenTrend(ctx context.Context, q Query, now time.Time) []ScreenTrendPointDTO {
	hourStart := now.Truncate(time.Hour)
	since := hourStart.Add(-time.Duration(screenTrendHours-1) * time.Hour)
	var rows []struct {
		Bucket time.Time `gorm:"column:bucket"`
		N      int64     `gorm:"column:n"`
		Paid   int64     `gorm:"column:paid"`
	}
	_ = s.screenOrderScope(ctx, q).Where("created_at >= ?", since).
		Select("date_trunc('hour', created_at) AS bucket, COUNT(*) AS n, SUM(CASE WHEN payment_status = ? THEN 1 ELSE 0 END) AS paid", order.PaymentPaid).
		Group("date_trunc('hour', created_at)").
		Scan(&rows).Error
	byHour := make(map[int64]*ScreenTrendPointDTO, len(rows))
	for i := range rows {
		key := rows[i].Bucket.Truncate(time.Hour).Unix()
		byHour[key] = &ScreenTrendPointDTO{OrderCount: rows[i].N, PaidCount: rows[i].Paid}
	}
	out := make([]ScreenTrendPointDTO, 0, screenTrendHours)
	for i := 0; i < screenTrendHours; i++ {
		h := since.Add(time.Duration(i) * time.Hour)
		p := ScreenTrendPointDTO{Hour: h.Format(time.RFC3339)}
		if b := byHour[h.Unix()]; b != nil {
			p.OrderCount = b.OrderCount
			p.PaidCount = b.PaidCount
		}
		out = append(out, p)
	}
	return out
}

// screenTodos returns the five order-lifecycle todo counters (same query
// 口径 as the workbench overview).
func (s *Service) screenTodos(ctx context.Context, q Query) []ScreenTodoDTO {
	var awaitPayment, awaitProcurement, awaitShipment, inTransit, exceptions int64
	_ = s.screenOrderScope(ctx, q).
		Where("payment_status = ? AND status NOT IN ?", order.PaymentUnpaid,
			[]string{order.StatusCancelled, order.StatusRefunded, order.StatusClosed}).
		Count(&awaitPayment).Error
	_ = s.screenOrderScope(ctx, q).
		Where("payment_status = ? AND fulfillment_status = ? AND status NOT IN ?",
			order.PaymentPaid, order.FulfillmentUnfulfilled,
			[]string{order.StatusShipped, order.StatusDelivered, order.StatusCancelled, order.StatusRefunded, order.StatusClosed}).
		Where("NOT EXISTS (SELECT 1 FROM purchase_order_items poi JOIN purchase_orders po2 ON po2.id = poi.purchase_order_id WHERE poi.sales_order_id = orders.id AND po2.status NOT IN ('cancelled','failed','voided'))").
		Count(&awaitProcurement).Error
	_ = s.screenOrderScope(ctx, q).
		Where("payment_status = ? AND fulfillment_status = ? AND status NOT IN ?",
			order.PaymentPaid, order.FulfillmentUnfulfilled,
			[]string{order.StatusShipped, order.StatusDelivered, order.StatusCancelled, order.StatusRefunded, order.StatusClosed}).
		Count(&awaitShipment).Error
	_ = s.screenOrderScope(ctx, q).Where("status = ?", order.StatusShipped).Count(&inTransit).Error
	if s.OrderExceptions != nil {
		exShops := q.Scope.AllowedShopIDs
		if q.Scope.IsAdmin {
			exShops = nil
		}
		if ex, err := s.OrderExceptions.DashboardSummary(ctx, strings.TrimSpace(q.Platform), strings.TrimSpace(q.ShopID), nil, nil, q.Scope.TenantID, exShops); err == nil {
			exceptions = ex.TotalOpen
		}
	}
	return []ScreenTodoDTO{
		{Key: "await_payment", Title: "待收款确认", Count: awaitPayment, Priority: "P1", Link: "/orders/list?payStatus=unpaid"},
		{Key: "await_procurement", Title: "待采购", Count: awaitProcurement, Priority: "P1", Link: "/orders/list?payStatus=paid&hasPurchase=0"},
		{Key: "await_shipment", Title: "待发货", Count: awaitShipment, Priority: "P1", Link: "/orders/list?payStatus=paid&fulfillmentStatus=unfulfilled"},
		{Key: "in_transit", Title: "在途待送达", Count: inTransit, Priority: "P2", Link: "/orders/list?status=shipped"},
		{Key: "order_exceptions", Title: "订单异常", Count: exceptions, Priority: "P0", Link: "/orders/exceptions"},
	}
}

// screenAlerts merges open task alerts with low-stock / out-of-stock SKUs
// into one time-ordered ticker list.
func (s *Service) screenAlerts(ctx context.Context, q Query) []ScreenAlertDTO {
	out := make([]ScreenAlertDTO, 0, screenAlertsLimit)

	var alerts []taskcenter.TaskAlert
	_ = q.Scope.applyTenantColumn(s.DB.WithContext(ctx).Model(&taskcenter.TaskAlert{}), "tenant_id").
		Where("status = ?", taskcenter.TaskAlertStatusOpen).
		Order("last_seen_at DESC").Limit(screenAlertsLimit).
		Find(&alerts).Error
	for _, a := range alerts {
		t := a.LastSeenAt
		out = append(out, ScreenAlertDTO{
			Type:       "task_alert",
			Severity:   a.Severity,
			Title:      clip(a.Title, 80),
			Detail:     clip(a.Message, 120),
			Link:       "/ops/task-center/alerts",
			OccurredAt: &t,
		})
	}

	out = append(out, s.screenInventoryAlerts(ctx, q, inventory.AlertTypeOutOfStock, "out_of_stock", "断货")...)
	out = append(out, s.screenInventoryAlerts(ctx, q, inventory.AlertTypeLowStock, "low_stock", "低库存")...)

	sort.SliceStable(out, func(i, j int) bool {
		ti, tj := out[i].OccurredAt, out[j].OccurredAt
		if ti == nil {
			return false
		}
		if tj == nil {
			return true
		}
		return ti.After(*tj)
	})
	if len(out) > screenAlertsLimit {
		out = out[:screenAlertsLimit]
	}
	return out
}

func (s *Service) screenInventoryAlerts(ctx context.Context, q Query, alertType, typ, label string) []ScreenAlertDTO {
	if s.Inventory == nil {
		return nil
	}
	invQ := inventory.AlertsListQuery{
		TenantID:  q.Scope.TenantID,
		Platform:  strings.TrimSpace(q.Platform),
		ShopID:    shopFilterUUID(q, q.Scope),
		AlertType: alertType,
		Page:      1, PageSize: screenAlertsLimit / 2,
	}
	if !q.Scope.IsAdmin && invQ.ShopID == nil {
		// Non-admin without explicit shop filter: query the first allowed shop
		// per call is wrong; list per allowed shop and merge (bounded count).
		// An empty allowed set yields no rows rather than a tenant-wide list.
		var merged []ScreenAlertDTO
		for i := range q.Scope.AllowedShopIDs {
			sq := invQ
			sq.ShopID = &q.Scope.AllowedShopIDs[i]
			merged = append(merged, s.invAlertRows(ctx, sq, typ, label)...)
			if len(merged) >= screenAlertsLimit/2 {
				break
			}
		}
		return merged
	}
	return s.invAlertRows(ctx, invQ, typ, label)
}

func (s *Service) invAlertRows(ctx context.Context, invQ inventory.AlertsListQuery, typ, label string) []ScreenAlertDTO {
	res, err := s.Inventory.ListInventoryAlerts(ctx, invQ)
	if err != nil || res == nil {
		return nil
	}
	out := make([]ScreenAlertDTO, 0, len(res.Items))
	for _, it := range res.Items {
		severity := "medium"
		if typ == "out_of_stock" {
			severity = "high"
		}
		out = append(out, ScreenAlertDTO{
			Type:       typ,
			Severity:   severity,
			Title:      clip(fmt.Sprintf("%s：%s", label, it.ProductTitle), 80),
			Detail:     clip(fmt.Sprintf("SKU %s 库存 %d（预警线 %d）", it.SKUCode, it.Stock, it.WarningStock), 120),
			Link:       "/inventory/alerts",
			OccurredAt: it.LastInventoryChangeAt,
		})
	}
	return out
}
