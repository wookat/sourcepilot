package demoseed

import (
	"context"
	"strings"
	"testing"

	"github.com/trademind-ai/trademind/backend/internal/modules/order"
	"github.com/trademind-ai/trademind/backend/internal/modules/platformtenant"
)

// Round 130: execution-log demo coverage closure — recommend（仅推荐）mode
// samples on the operator-granted manual shop plus second-tenant
// success/failed/skipped logs, all cleaned with zero residue and idempotent.
func TestFullDemoSeedRound130AutomationLogSamples(t *testing.T) {
	db := openFullDemoTestDB(t)
	s := &FullDemoSeeder{DB: db, TenantID: 1, AppEnv: "development"}

	if _, err := s.Seed(context.Background()); err != nil {
		t.Fatal(err)
	}

	// 主租户 recommend 模式规则 + 成功日志（覆盖仅推荐文案）。
	var recRule order.OrderAutomationRule
	if err := db.First(&recRule, "tenant_id = ? AND action = ? AND shipping_apply_mode = ?",
		s.TenantID, order.AutomationActionApplyShippingRule, order.ShippingApplyModeRecommend).Error; err != nil {
		t.Fatalf("expected recommend-mode demo rule: %v", err)
	}
	var recLog order.OrderAutomationLog
	if err := db.First(&recLog, "rule_id = ? AND status = ?", recRule.ID, order.AutomationLogSuccess).Error; err != nil {
		t.Fatalf("expected recommend success log: %v", err)
	}
	if !strings.Contains(recLog.Reason, "仅推荐，发货时人工确认") {
		t.Fatalf("recommend success log must carry recommend wording, got %q", recLog.Reason)
	}
	var recOrder order.Order
	if err := db.First(&recOrder, "id = ?", recLog.OrderID).Error; err != nil {
		t.Fatalf("expected recommend sample order: %v", err)
	}
	if recOrder.PlannedCarrierMode != order.ShippingApplyModeRecommend || recOrder.Platform != "manual" {
		t.Fatalf("unexpected recommend sample order plan: %+v", recOrder)
	}

	// 手工渠道店（operator 授权店）三种状态齐全，operator 视角不再空态。
	for _, st := range []string{order.AutomationLogSuccess, order.AutomationLogFailed, order.AutomationLogSkipped} {
		var n int64
		db.Model(&order.OrderAutomationLog{}).
			Joins("JOIN orders ON orders.id = order_automation_logs.order_id").
			Where("order_automation_logs.tenant_id = ? AND orders.platform = ? AND order_automation_logs.status = ?",
				s.TenantID, "manual", st).Count(&n)
		if n == 0 {
			t.Fatalf("expected manual-shop automation log with status %s", st)
		}
	}

	// 执行日志必须带 shop_id 快照，否则店铺 scope 过滤后 operator 视角为空。
	var missingShop int64
	db.Model(&order.OrderAutomationLog{}).
		Where("rule_name LIKE ? AND shop_id IS NULL", "DEMO-%").Count(&missingShop)
	if missingShop != 0 {
		t.Fatalf("expected all demo automation logs to carry shop_id, %d missing", missingShop)
	}

	// 第二租户三种状态齐全，且成功样本覆盖 recommend 文案。
	var tenant platformtenant.Tenant
	if err := db.First(&tenant, "name = ?", DemoTenant2Name).Error; err != nil {
		t.Fatalf("expected second demo tenant: %v", err)
	}
	for _, st := range []string{order.AutomationLogSuccess, order.AutomationLogFailed, order.AutomationLogSkipped} {
		var n int64
		db.Model(&order.OrderAutomationLog{}).Where("tenant_id = ? AND status = ?", tenant.ID, st).Count(&n)
		if n == 0 {
			t.Fatalf("expected second-tenant automation log with status %s", st)
		}
	}
	var t2Success order.OrderAutomationLog
	if err := db.First(&t2Success, "tenant_id = ? AND status = ?", tenant.ID, order.AutomationLogSuccess).Error; err != nil {
		t.Fatalf("expected second-tenant success log: %v", err)
	}
	if !strings.Contains(t2Success.Reason, "仅推荐，发货时人工确认") {
		t.Fatalf("second-tenant success log must carry recommend wording, got %q", t2Success.Reason)
	}

	// 幂等重跑：样本订单 / 日志不翻倍。
	if _, err := s.Seed(context.Background()); err != nil {
		t.Fatal(err)
	}
	var sampleOrders int64
	db.Model(&order.Order{}).Where("order_no IN ?",
		[]string{"DEMO-AT-1301", "DEMO-AT-1302", "DEMO-AT-1303",
			"DEMO-T2-AT-0001", "DEMO-T2-AT-0002", "DEMO-T2-AT-0003"}).Count(&sampleOrders)
	if sampleOrders != 6 {
		t.Fatalf("reseed not idempotent: expected 6 round130 sample orders, got %d", sampleOrders)
	}

	if _, err := s.Cleanup(context.Background()); err != nil {
		t.Fatal(err)
	}
	verify, err := s.VerifyClean(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	for _, table := range []string{"orders", "order_items", "order_automation_rules", "order_automation_logs"} {
		if verify.Counts[table] != 0 {
			t.Fatalf("expected zero %s residue after cleanup, got %d", table, verify.Counts[table])
		}
	}
	var residue int64
	db.Unscoped().Model(&order.OrderAutomationLog{}).Where("rule_name LIKE ?", "DEMO-%").Count(&residue)
	if residue != 0 {
		t.Fatalf("round130 automation log residue: %d", residue)
	}
}
