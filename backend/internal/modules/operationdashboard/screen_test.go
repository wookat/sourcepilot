package operationdashboard

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
)

// 大屏订单查询必须同时带租户列过滤和店铺 scope（operator 有限店铺）。
func TestScreenOrderScopeTenantAndShop(t *testing.T) {
	db := newDryRunDB(t)
	tid := int64(3)
	shop := uuid.New()
	q := Query{Scope: Scope{TenantID: &tid, AllowedShopIDs: []uuid.UUID{shop}}}
	s := &Service{DB: db}

	tx := s.screenOrderScope(context.Background(), q)
	sql := tx.Count(new(int64)).Statement.SQL.String()
	if !strings.Contains(sql, "tenant_id = ?") {
		t.Fatalf("expected tenant predicate, got: %s", sql)
	}
	if !strings.Contains(sql, "shop_id IN") {
		t.Fatalf("expected shop scope predicate, got: %s", sql)
	}
}

// readonly（无店铺授权）必须得到恒假条件，不得回退为全量。
func TestScreenOrderScopeNoShopsFailClosed(t *testing.T) {
	db := newDryRunDB(t)
	tid := int64(3)
	q := Query{Scope: Scope{TenantID: &tid}}
	s := &Service{DB: db}

	sql := s.screenOrderScope(context.Background(), q).Count(new(int64)).Statement.SQL.String()
	if !strings.Contains(sql, "1 = 0") {
		t.Fatalf("expected fail-closed predicate for empty shop scope, got: %s", sql)
	}
}

// 24h 趋势必须补齐全部 24 个整点桶（无数据小时为 0），且首尾相差 23 小时。
func TestScreenTrendFillsAllHourBuckets(t *testing.T) {
	db := newDryRunDB(t)
	s := &Service{DB: db}
	now := time.Date(2026, 8, 6, 15, 30, 0, 0, time.UTC)

	points := s.screenTrend(context.Background(), Query{Scope: Scope{IsAdmin: true}}, now)
	if len(points) != screenTrendHours {
		t.Fatalf("expected %d buckets, got %d", screenTrendHours, len(points))
	}
	first, err := time.Parse(time.RFC3339, points[0].Hour)
	if err != nil {
		t.Fatalf("invalid first bucket hour: %v", err)
	}
	last, err := time.Parse(time.RFC3339, points[len(points)-1].Hour)
	if err != nil {
		t.Fatalf("invalid last bucket hour: %v", err)
	}
	if last.Sub(first) != time.Duration(screenTrendHours-1)*time.Hour {
		t.Fatalf("expected 23h span, got %v", last.Sub(first))
	}
	if !last.Equal(now.Truncate(time.Hour)) {
		t.Fatalf("last bucket should be the current hour, got %v", last)
	}
}

// 漏斗与待办的键序固定：前端按键渲染，顺序变化视为契约破坏。
func TestScreenFunnelAndTodoKeys(t *testing.T) {
	db := newDryRunDB(t)
	s := &Service{DB: db}
	q := Query{Scope: Scope{IsAdmin: true}}

	funnel := s.screenFunnel(context.Background(), q, time.Now().AddDate(0, 0, -7))
	wantFunnel := []string{"created", "paid", "procured", "shipped", "delivered"}
	if len(funnel) != len(wantFunnel) {
		t.Fatalf("expected %d funnel stages, got %d", len(wantFunnel), len(funnel))
	}
	for i, k := range wantFunnel {
		if funnel[i].Key != k {
			t.Fatalf("funnel[%d] = %s, want %s", i, funnel[i].Key, k)
		}
	}

	todos := s.screenTodos(context.Background(), q)
	wantTodos := []string{"await_payment", "await_procurement", "await_shipment", "in_transit", "order_exceptions"}
	if len(todos) != len(wantTodos) {
		t.Fatalf("expected %d todos, got %d", len(wantTodos), len(todos))
	}
	for i, k := range wantTodos {
		if todos[i].Key != k {
			t.Fatalf("todos[%d] = %s, want %s", i, todos[i].Key, k)
		}
	}
}
