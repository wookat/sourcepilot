package demoseed

import (
	"context"
	"testing"

	"github.com/trademind-ai/trademind/backend/internal/modules/collectrule"
	"github.com/trademind-ai/trademind/backend/internal/modules/order"
)

// Round 135: demo采集规则样本 + 订单标签样本 seed / idempotency / cleanup /
// verify leave zero residue.
func TestFullDemoSeedRound135TagsAndCollectRules(t *testing.T) {
	db := openFullDemoTestDB(t)
	s := &FullDemoSeeder{DB: db, TenantID: 1, AppEnv: "development"}

	if _, err := s.Seed(context.Background()); err != nil {
		t.Fatal(err)
	}

	// 采集规则：启用 + 停用样本，开箱非空。
	var crCount int64
	if err := db.Model(&collectrule.CollectRule{}).Where("name LIKE ?", "DEMO-%").Count(&crCount).Error; err != nil {
		t.Fatal(err)
	}
	if crCount != 2 {
		t.Fatalf("collect rules = %d, want 2", crCount)
	}
	var enabled collectrule.CollectRule
	if err := db.First(&enabled, "name LIKE ? AND status = ?", "DEMO-%", collectrule.StatusEnabled).Error; err != nil {
		t.Fatalf("expected enabled demo collect rule: %v", err)
	}
	if err := collectrule.ValidateRuleJSON(enabled.Rule); err != nil {
		t.Fatalf("enabled rule json invalid: %v", err)
	}
	if err := collectrule.ValidateRuleForEnable(enabled.Rule); err != nil {
		t.Fatalf("enabled rule must satisfy enable requirements: %v", err)
	}

	// 订单标签：3 个租户标签 + 打标样本 + add_tag 规则 + success log。
	var tagCount int64
	if err := db.Model(&order.OrderTag{}).Where("name LIKE ?", "DEMO-%").Count(&tagCount).Error; err != nil {
		t.Fatal(err)
	}
	if tagCount != 3 {
		t.Fatalf("order tags = %d, want 3", tagCount)
	}
	var linkCount int64
	if err := db.Model(&order.OrderTagLink{}).Count(&linkCount).Error; err != nil {
		t.Fatal(err)
	}
	if linkCount == 0 {
		t.Fatal("expected demo order tag links")
	}
	var tagRule order.OrderAutomationRule
	if err := db.First(&tagRule, "action = ?", order.AutomationActionAddTag).Error; err != nil {
		t.Fatalf("expected add_tag demo rule: %v", err)
	}
	if !tagRule.Enabled || len(tagRule.TagIDs) == 0 {
		t.Fatalf("unexpected add_tag rule: %+v", tagRule)
	}
	var tagLog order.OrderAutomationLog
	if err := db.First(&tagLog, "rule_id = ? AND status = ?", tagRule.ID, order.AutomationLogSuccess).Error; err != nil {
		t.Fatalf("expected success log for add_tag rule: %v", err)
	}

	// 幂等：重复 seed 数量不翻倍。
	if _, err := s.Seed(context.Background()); err != nil {
		t.Fatal(err)
	}
	var again int64
	db.Model(&order.OrderTag{}).Where("name LIKE ?", "DEMO-%").Count(&again)
	if again != 3 {
		t.Fatalf("tags after reseed = %d, want 3", again)
	}
	db.Model(&collectrule.CollectRule{}).Where("name LIKE ?", "DEMO-%").Count(&again)
	if again != 2 {
		t.Fatalf("collect rules after reseed = %d, want 2", again)
	}

	// clean + verify 零残留。
	if _, err := s.Cleanup(context.Background()); err != nil {
		t.Fatal(err)
	}
	res, err := s.VerifyClean(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	for _, table := range []string{"order_tags", "order_tag_links", "collect_rules"} {
		if res.Counts[table] != 0 {
			t.Fatalf("verify residual %s = %d", table, res.Counts[table])
		}
	}
	var residual int64
	db.Model(&order.OrderTagLink{}).Count(&residual)
	if residual != 0 {
		t.Fatalf("order_tag_links residual = %d", residual)
	}
}
