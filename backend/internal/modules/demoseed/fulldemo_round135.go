package demoseed

import (
	"fmt"

	"gorm.io/datatypes"
	"gorm.io/gorm"

	"github.com/trademind-ai/trademind/backend/internal/modules/collectrule"
	"github.com/trademind-ai/trademind/backend/internal/modules/order"
	"github.com/trademind-ai/trademind/backend/internal/modules/shop"
)

// seedRound135CollectRules adds custom采集规则 samples so the collector-rules
// page opens non-empty out of the box (启用 + 停用 each covered). Everything is
// DEMO- prefixed and removed by Cleanup / checked by VerifyClean.
func (s *FullDemoSeeder) seedRound135CollectRules(tx *gorm.DB, res *FullDemoResult) error {
	count := func(table string, n int64) { res.Counts[table] += n }

	enabledRule := collectrule.CollectRule{
		TenantID:     s.TenantID,
		Name:         "DEMO-通用商品页采集规则",
		Source:       collectrule.SourceCustom,
		Domain:       "demo-shop.example.com",
		MatchPattern: `/product/\d+`,
		Status:       collectrule.StatusEnabled,
		Priority:     100,
		Rule: datatypes.JSON([]byte(`{
			"title": {"selectors": ["h1.product-title"], "attr": "text"},
			"price": {"selectors": [".price .value"], "attr": "text"},
			"currency": {"selectors": [".price .currency"], "attr": "text"},
			"mainImages": {"selectors": [".gallery img"], "attr": "src", "multiple": true, "limit": 10},
			"fallbacks": {"openGraph": true, "jsonLd": true}
		}`)),
		Remark: "DEMO- 演示：启用状态的自定义采集规则（含主图与兜底解析）",
	}
	if err := tx.Create(&enabledRule).Error; err != nil {
		return fmt.Errorf("demoseed: collect rule enabled: %w", err)
	}
	disabledRule := collectrule.CollectRule{
		TenantID: s.TenantID,
		Name:     "DEMO-草稿采集规则（仅标题）",
		Source:   collectrule.SourceCustom,
		Domain:   "demo-mall.example.com",
		Status:   collectrule.StatusDisabled,
		Priority: 200,
		Rule: datatypes.JSON([]byte(`{
			"title": {"selectors": ["h1"], "attr": "text"}
		}`)),
		Remark: "DEMO- 演示：停用状态的规则草稿（未配置主图，暂不可启用采集）",
	}
	if err := tx.Create(&disabledRule).Error; err != nil {
		return fmt.Errorf("demoseed: collect rule disabled: %w", err)
	}
	count("collect_rules", 2)
	return nil
}

// seedRound135OrderTags adds tenant order tags, manual tag links on existing
// demo orders, an add_tag automation rule and one execution-log sample so the
// R135 打标签 line demos out of the box. Everything is DEMO- prefixed (or
// linked to DEMO- orders) and removed by Cleanup / checked by VerifyClean.
func (s *FullDemoSeeder) seedRound135OrderTags(tx *gorm.DB, res *FullDemoResult, shops []shop.Shop) error {
	if len(shops) == 0 {
		return nil
	}
	count := func(table string, n int64) { res.Counts[table] += n }

	urgent := order.OrderTag{TenantID: s.TenantID, Name: "DEMO-加急", Color: "red"}
	vip := order.OrderTag{TenantID: s.TenantID, Name: "DEMO-大客户", Color: "gold"}
	gift := order.OrderTag{TenantID: s.TenantID, Name: "DEMO-赠品单", Color: "green"}
	for _, t := range []*order.OrderTag{&urgent, &vip, &gift} {
		if err := tx.Create(t).Error; err != nil {
			return fmt.Errorf("demoseed: order tag %s: %w", t.Name, err)
		}
	}
	count("order_tags", 3)

	// 手工打标样本：给既有 DEMO 自动化订单补标签，订单列表标签列开箱非空。
	manualLinks := []struct {
		orderNo string
		tag     *order.OrderTag
	}{
		{"DEMO-AT-1001", &urgent},
		{"DEMO-AT-1001", &vip},
		{"DEMO-AT-1201", &vip},
		{"DEMO-AT-1004", &gift},
	}
	links := 0
	for _, ml := range manualLinks {
		var o order.Order
		if err := tx.Where("tenant_id = ? AND order_no = ?", s.TenantID, ml.orderNo).
			First(&o).Error; err != nil {
			continue
		}
		link := order.OrderTagLink{
			TenantID: s.TenantID, OrderID: o.ID, TagID: ml.tag.ID,
			Source: order.TagLinkSourceManual,
		}
		if err := tx.Create(&link).Error; err != nil {
			return fmt.Errorf("demoseed: order tag link %s: %w", ml.orderNo, err)
		}
		links++
	}
	count("order_tag_links", int64(links))

	// 自动打标签规则 + 成功执行日志样本（挂到 DEMO-AT-1201）。
	tagRule := order.OrderAutomationRule{
		TenantID: s.TenantID, Name: "DEMO-付款后自动打「大客户」标签", Priority: 8, Enabled: true,
		TriggerEvent: order.AutomationEventOrderPaid,
		Action:       order.AutomationActionAddTag,
		TagIDs:       datatypes.JSON([]byte(fmt.Sprintf(`["%s"]`, vip.ID))),
	}
	if err := tx.Create(&tagRule).Error; err != nil {
		return fmt.Errorf("demoseed: automation tag rule: %w", err)
	}
	count("order_automation_rules", 1)

	var tagged order.Order
	if err := tx.Where("tenant_id = ? AND order_no = ?", s.TenantID, "DEMO-AT-1201").
		First(&tagged).Error; err == nil {
		tagLog := order.OrderAutomationLog{
			TenantID: s.TenantID, RuleID: tagRule.ID, RuleName: tagRule.Name,
			OrderID: tagged.ID, OrderNo: tagged.OrderNo, ShopID: tagged.ShopID,
			TriggerEvent: tagRule.TriggerEvent, Action: tagRule.Action,
			Status:   order.AutomationLogSuccess,
			Reason:   "已自动打标签：「DEMO-大客户」",
			Attempts: 1,
			DedupKey: fmt.Sprintf("%d:%s:%s:%s", s.TenantID, tagRule.ID, tagged.ID, tagRule.TriggerEvent),
		}
		if err := tx.Create(&tagLog).Error; err != nil {
			return fmt.Errorf("demoseed: automation tag log: %w", err)
		}
		count("order_automation_logs", 1)
	}
	return nil
}
