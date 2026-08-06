package demoseed

import (
	"context"
	"testing"

	"github.com/trademind-ai/trademind/backend/internal/modules/order"
)

// Round 119: demo自动化订单规则 + 执行日志样本 seed / cleanup / verify leave
// zero residue.
func TestFullDemoSeedOrderAutomationSamples(t *testing.T) {
	db := openFullDemoTestDB(t)
	s := &FullDemoSeeder{DB: db, TenantID: 1, AppEnv: "development"}

	if _, err := s.Seed(context.Background()); err != nil {
		t.Fatal(err)
	}

	var rules []order.OrderAutomationRule
	// Round 128 起 seed 还会给第二演示租户建 1 条规则，这里只核对主租户。
	if err := db.Where("tenant_id = ?", 1).Find(&rules).Error; err != nil {
		t.Fatal(err)
	}
	if len(rules) != 7 {
		t.Fatalf("expected 7 demo automation rules (4 round119 + 2 round126 + 1 round130 recommend), got %d", len(rules))
	}
	var disabled int
	for _, r := range rules {
		if r.TenantID != 1 {
			t.Fatalf("rule %s wrong tenant %d", r.Name, r.TenantID)
		}
		if !order.AutomationActionAllowed(r.TriggerEvent, r.Action) {
			t.Fatalf("rule %s has invalid event/action pair %s/%s", r.Name, r.TriggerEvent, r.Action)
		}
		if !r.Enabled {
			disabled++
		}
	}
	if disabled != 1 {
		t.Fatalf("expected exactly one disabled demo rule, got %d", disabled)
	}

	statuses := map[string]int64{}
	for _, st := range []string{order.AutomationLogSuccess, order.AutomationLogFailed, order.AutomationLogSkipped} {
		var n int64
		db.Model(&order.OrderAutomationLog{}).Where("status = ?", st).Count(&n)
		statuses[st] = n
	}
	if statuses[order.AutomationLogSuccess] < 1 || statuses[order.AutomationLogFailed] < 1 || statuses[order.AutomationLogSkipped] < 1 {
		t.Fatalf("expected success/failed/skipped log samples, got %+v", statuses)
	}

	// 正向样本：DEMO-AT-1004 未付款 + 审单通过 + 商品行已匹配本地 SKU，
	// 标记已付款即可真实触发「自动生成采购单」成功动线。
	var matched order.Order
	if err := db.First(&matched, "order_no = ?", "DEMO-AT-1004").Error; err != nil {
		t.Fatalf("expected DEMO-AT-1004 matched-SKU sample order: %v", err)
	}
	if matched.PaymentStatus != order.PaymentUnpaid || matched.ReviewStatus != order.ReviewStatusAutoPassed {
		t.Fatalf("unexpected DEMO-AT-1004 state: %+v", matched)
	}
	var matchedItems []order.OrderItem
	if err := db.Where("order_id = ?", matched.ID).Find(&matchedItems).Error; err != nil {
		t.Fatal(err)
	}
	if len(matchedItems) == 0 {
		t.Fatal("expected DEMO-AT-1004 to have item rows")
	}
	for _, it := range matchedItems {
		if it.ProductID == nil || it.ProductSKUID == nil {
			t.Fatalf("DEMO-AT-1004 item must link a local SKU, got %+v", it)
		}
	}

	if _, err := s.Cleanup(context.Background()); err != nil {
		t.Fatal(err)
	}
	verify, err := s.VerifyClean(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if verify.Counts["order_automation_rules"] != 0 {
		t.Fatalf("expected zero demo automation rules after cleanup, got %d", verify.Counts["order_automation_rules"])
	}
	if verify.Counts["order_automation_logs"] != 0 {
		t.Fatalf("expected zero demo automation logs after cleanup, got %d", verify.Counts["order_automation_logs"])
	}
	var residue int64
	db.Model(&order.OrderAutomationRule{}).Count(&residue)
	if residue != 0 {
		t.Fatalf("automation rules residue: %d", residue)
	}
	db.Model(&order.OrderAutomationLog{}).Count(&residue)
	if residue != 0 {
		t.Fatalf("automation logs residue: %d", residue)
	}
}
