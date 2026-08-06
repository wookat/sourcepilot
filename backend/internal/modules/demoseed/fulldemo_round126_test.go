package demoseed

import (
	"context"
	"testing"

	"github.com/trademind-ai/trademind/backend/internal/modules/order"
)

// Round 126: demo自动化动作面扩展（自动应用发货规则 / 自动分仓）seed / cleanup /
// verify leave zero residue.
func TestFullDemoSeedRound126AutoActions(t *testing.T) {
	db := openFullDemoTestDB(t)
	s := &FullDemoSeeder{DB: db, TenantID: 1, AppEnv: "development"}

	if _, err := s.Seed(context.Background()); err != nil {
		t.Fatal(err)
	}

	var shipRule order.OrderAutomationRule
	if err := db.First(&shipRule, "action = ? AND shipping_apply_mode = ?",
		order.AutomationActionApplyShippingRule, order.ShippingApplyModeApply).Error; err != nil {
		t.Fatalf("expected apply_shipping_rule demo rule: %v", err)
	}
	if shipRule.ShippingApplyMode != order.ShippingApplyModeApply || !shipRule.Enabled {
		t.Fatalf("unexpected shipping rule params: %+v", shipRule)
	}
	var whRule order.OrderAutomationRule
	if err := db.First(&whRule, "action = ?", order.AutomationActionAssignWarehouse).Error; err != nil {
		t.Fatalf("expected assign_warehouse demo rule: %v", err)
	}
	if whRule.WarehouseStrategy != order.AutomationWarehouseStrategyStockFirst {
		t.Fatalf("unexpected warehouse rule params: %+v", whRule)
	}

	// 正向样本：DEMO-AT-1201 已按发货规则落物流商 + success log。
	var applied order.Order
	if err := db.First(&applied, "order_no = ?", "DEMO-AT-1201").Error; err != nil {
		t.Fatalf("expected DEMO-AT-1201 sample order: %v", err)
	}
	if applied.PlannedCarrierCode != "sf" || applied.PlannedCarrierMode != order.ShippingApplyModeApply {
		t.Fatalf("unexpected DEMO-AT-1201 carrier plan: %+v", applied)
	}
	var appliedLog order.OrderAutomationLog
	if err := db.First(&appliedLog, "order_id = ? AND status = ?", applied.ID, order.AutomationLogSuccess).Error; err != nil {
		t.Fatalf("expected success log for DEMO-AT-1201: %v", err)
	}

	// 负向样本：DEMO-AT-1202 自动分仓因库存不足失败留痕。
	var short order.Order
	if err := db.First(&short, "order_no = ?", "DEMO-AT-1202").Error; err != nil {
		t.Fatalf("expected DEMO-AT-1202 sample order: %v", err)
	}
	if short.AssignedWarehouseID != nil {
		t.Fatalf("DEMO-AT-1202 must not have a warehouse assigned: %+v", short)
	}
	var shortLog order.OrderAutomationLog
	if err := db.First(&shortLog, "order_id = ? AND status = ?", short.ID, order.AutomationLogFailed).Error; err != nil {
		t.Fatalf("expected failed log for DEMO-AT-1202: %v", err)
	}
	if shortLog.Action != order.AutomationActionAssignWarehouse {
		t.Fatalf("unexpected failed log action: %+v", shortLog)
	}

	if _, err := s.Cleanup(context.Background()); err != nil {
		t.Fatal(err)
	}
	verify, err := s.VerifyClean(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	for _, table := range []string{"orders", "order_automation_rules", "order_automation_logs"} {
		if verify.Counts[table] != 0 {
			t.Fatalf("expected zero %s residue after cleanup, got %d", table, verify.Counts[table])
		}
	}
	var residue int64
	db.Model(&order.Order{}).Where("order_no LIKE ?", "DEMO-AT-12%").Count(&residue)
	if residue != 0 {
		t.Fatalf("round126 demo orders residue: %d", residue)
	}
}
