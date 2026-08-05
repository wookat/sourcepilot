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
	if err := db.Find(&rules).Error; err != nil {
		t.Fatal(err)
	}
	if len(rules) != 4 {
		t.Fatalf("expected 4 demo automation rules, got %d", len(rules))
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
