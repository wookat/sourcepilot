package demoseed

import (
	"fmt"
	"time"

	"gorm.io/gorm"

	"github.com/trademind-ai/trademind/backend/internal/modules/customerchat"
	"github.com/trademind-ai/trademind/backend/internal/modules/order"
	"github.com/trademind-ai/trademind/backend/internal/modules/shop"
)

// seedRound119BuyerMessages adds demo买家自动消息节点规则 plus待发/已发送/已忽略
// draft samples so the待发消息工作台 demos out of the box. Everything is
// DEMO- prefixed and removed by Cleanup / checked by VerifyClean.
func (s *FullDemoSeeder) seedRound119BuyerMessages(tx *gorm.DB, res *FullDemoResult, now time.Time, shops []shop.Shop) error {
	if len(shops) == 0 {
		return nil
	}
	count := func(table string, n int64) { res.Counts[table] += n }
	demoShop := shops[0]

	var logisticsTpl, refundTpl customerchat.CustomerReplyTemplate
	if err := tx.Where("tenant_id = ? AND name = ?", s.TenantID, "DEMO-物流-查询进度").
		First(&logisticsTpl).Error; err != nil {
		return fmt.Errorf("demoseed: buyer msg logistics template: %w", err)
	}
	if err := tx.Where("tenant_id = ? AND name = ?", s.TenantID, "DEMO-退款-流程说明").
		First(&refundTpl).Error; err != nil {
		return fmt.Errorf("demoseed: buyer msg refund template: %w", err)
	}

	rules := []customerchat.BuyerMessageRule{
		{TenantID: s.TenantID, Name: "DEMO-发货后自动通知买家", Node: customerchat.BuyerMsgNodeShipped,
			TemplateID: logisticsTpl.ID, Enabled: true},
		{TenantID: s.TenantID, Name: "DEMO-退款进度自动告知", Node: customerchat.BuyerMsgNodeRefunded,
			TemplateID: refundTpl.ID, Enabled: true},
		{TenantID: s.TenantID, Name: "DEMO-签收后关怀（停用示例）", Node: customerchat.BuyerMsgNodeDelivered,
			TemplateID: logisticsTpl.ID},
	}
	for i := range rules {
		if err := tx.Create(&rules[i]).Error; err != nil {
			return fmt.Errorf("demoseed: buyer msg rule %s: %w", rules[i].Name, err)
		}
	}
	count("buyer_message_rules", int64(len(rules)))

	shippedRule, refundRule := rules[0], rules[1]

	type draftPlan struct {
		orderNo    string
		status     string // order status
		rule       *customerchat.BuyerMessageRule
		tpl        *customerchat.CustomerReplyTemplate
		trackingNo string
		draft      string // draft status
		missing    bool
		// R152 多语言样本：rawCountry 写入订单 RawData 供收货地语言推断演示；
		// language/langSource 为草稿语言标注（空则回退 zh-CN + fallback）。
		rawCountry string
		language   string
		langSource string
	}
	plans := []draftPlan{
		{orderNo: "DEMO-BM-1001", status: order.StatusShipped, rule: &shippedRule, tpl: &logisticsTpl,
			trackingNo: "DEMO-TRK-BM-1", draft: customerchat.BuyerMsgDraftPending},
		{orderNo: "DEMO-BM-1002", status: order.StatusShipped, rule: &shippedRule, tpl: &logisticsTpl,
			draft: customerchat.BuyerMsgDraftPending, missing: true},
		{orderNo: "DEMO-BM-1003", status: order.StatusShipped, rule: &shippedRule, tpl: &logisticsTpl,
			trackingNo: "DEMO-TRK-BM-3", draft: customerchat.BuyerMsgDraftSent},
		{orderNo: "DEMO-BM-1004", status: order.StatusRefunded, rule: &refundRule, tpl: &refundTpl,
			draft: customerchat.BuyerMsgDraftIgnored},
		// R152 正样本：收货地 US → 英语变体
		{orderNo: "DEMO-BM-1005", status: order.StatusShipped, rule: &shippedRule, tpl: &logisticsTpl,
			trackingNo: "DEMO-TRK-BM-5", draft: customerchat.BuyerMsgDraftPending,
			rawCountry: "US", language: "en", langSource: customerchat.BuyerMsgLangSourceOrderCountry},
		// R152 正样本：收货地 BR → 葡萄牙语变体
		{orderNo: "DEMO-BM-1006", status: order.StatusShipped, rule: &shippedRule, tpl: &logisticsTpl,
			trackingNo: "DEMO-TRK-BM-6", draft: customerchat.BuyerMsgDraftPending,
			rawCountry: "BR", language: "pt", langSource: customerchat.BuyerMsgLangSourceOrderCountry},
	}
	for i, p := range plans {
		created := now.Add(-time.Duration(i+1) * time.Hour)
		o := order.Order{
			TenantID: s.TenantID, Platform: demoShop.Platform, ShopID: &demoShop.ID,
			OrderNo: p.orderNo, CustomerName: "DEMO-消息买家", CustomerPhone: "13800000119",
			Status: p.status, ReviewStatus: order.ReviewStatusApproved,
			PaymentStatus: order.PaymentPaid, FulfillmentStatus: order.FulfillmentUnfulfilled,
			Currency: "CNY", TotalAmount: 128, OrderedAt: &created,
		}
		if p.status == order.StatusRefunded {
			o.PaymentStatus = order.PaymentRefunded
		}
		if p.rawCountry != "" {
			o.RawData = mustJSON(map[string]any{"receiver": map[string]any{"countryCode": p.rawCountry}})
		}
		if err := tx.Create(&o).Error; err != nil {
			return fmt.Errorf("demoseed: buyer msg order %s: %w", p.orderNo, err)
		}
		item := order.OrderItem{
			OrderID: o.ID, ProductTitle: "DEMO-自动消息演示商品", SKUCode: "DEMO-BM-SKU",
			Quantity: 1, UnitPrice: 128, TotalPrice: 128,
		}
		if err := tx.Create(&item).Error; err != nil {
			return fmt.Errorf("demoseed: buyer msg item: %w", err)
		}
		count("orders", 1)
		count("order_items", 1)
		if p.trackingNo != "" {
			ship := order.OrderShipment{
				OrderID: o.ID, Carrier: "DEMO-中通快递", TrackingNo: p.trackingNo,
				Status: order.ShipmentShipped, ShippedAt: &created,
			}
			if err := tx.Create(&ship).Error; err != nil {
				return fmt.Errorf("demoseed: buyer msg shipment: %w", err)
			}
			count("order_shipments", 1)
		}

		vars := map[string]string{
			"买家昵称": o.CustomerName, "订单号": o.OrderNo, "物流单号": p.trackingNo,
			"商品名": item.ProductTitle, "店铺名": demoShop.ShopName,
		}
		tplContent := p.tpl.Content
		language, langSource := "zh-CN", customerchat.BuyerMsgLangSourceFallback
		if p.language != "" {
			var variant customerchat.CustomerReplyTemplateVariant
			if err := tx.Where("tenant_id = ? AND template_id = ? AND language = ?",
				s.TenantID, p.tpl.ID, p.language).First(&variant).Error; err != nil {
				return fmt.Errorf("demoseed: buyer msg variant %s/%s: %w", p.tpl.Name, p.language, err)
			}
			tplContent = variant.Content
			language, langSource = p.language, p.langSource
		}
		content, missing := customerchat.FillBuyerMsgTemplate(tplContent, vars)
		draft := customerchat.BuyerMessageDraft{
			TenantID: s.TenantID, OrderID: o.ID, Node: p.rule.Node,
			RuleID: p.rule.ID, TemplateID: p.tpl.ID, TemplateName: p.tpl.Name,
			Platform: o.Platform, ShopID: o.ShopID, OrderNo: o.OrderNo,
			CustomerName: o.CustomerName, Content: content,
			Language: language, LangSource: langSource,
			MissingVars: mustJSONStrings(missing...), Status: p.draft,
		}
		if p.draft == customerchat.BuyerMsgDraftSent {
			sentAt := created.Add(30 * time.Minute)
			draft.SentAt = &sentAt
		}
		if p.draft == customerchat.BuyerMsgDraftIgnored {
			ignoredAt := created.Add(30 * time.Minute)
			draft.IgnoredAt = &ignoredAt
		}
		if err := tx.Create(&draft).Error; err != nil {
			return fmt.Errorf("demoseed: buyer msg draft %s: %w", p.orderNo, err)
		}
		count("buyer_message_drafts", 1)
	}
	return nil
}
