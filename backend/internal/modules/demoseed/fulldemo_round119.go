package demoseed

import (
	"fmt"
	"time"

	"gorm.io/gorm"

	"github.com/trademind-ai/trademind/backend/internal/modules/order"
	"github.com/trademind-ai/trademind/backend/internal/modules/product"
	"github.com/trademind-ai/trademind/backend/internal/modules/shop"
)

// seedRound119OrderAutomation adds demo自动化订单规则 plus execution-log samples
// (成功/失败/跳过) so the automation rules page and execution log page demo out
// of the box. Everything is DEMO- prefixed and removed by Cleanup / checked by
// VerifyClean.
func (s *FullDemoSeeder) seedRound119OrderAutomation(tx *gorm.DB, res *FullDemoResult, now time.Time, shops []shop.Shop, products []product.Product, skus []product.ProductSKU) error {
	if len(shops) == 0 {
		return nil
	}
	count := func(table string, n int64) { res.Counts[table] += n }

	payMax := 100.0
	payRule := order.OrderAutomationRule{
		TenantID: s.TenantID, Name: "DEMO-低额订单自动确认付款", Priority: 1, Enabled: true,
		TriggerEvent: order.AutomationEventOrderCreated,
		Action:       order.AutomationActionConfirmPayment, MaxAmount: &payMax,
	}
	if err := tx.Create(&payRule).Error; err != nil {
		return fmt.Errorf("demoseed: automation pay rule: %w", err)
	}
	genMax := 500.0
	genRule := order.OrderAutomationRule{
		TenantID: s.TenantID, Name: "DEMO-付款后自动生成采购单", Priority: 2, Enabled: true,
		TriggerEvent: order.AutomationEventOrderPaid,
		Action:       order.AutomationActionGenerateProcurement,
		MaxAmount:    &genMax, RequireReviewPassed: false,
	}
	if err := tx.Create(&genRule).Error; err != nil {
		return fmt.Errorf("demoseed: automation generate rule: %w", err)
	}
	notifyRule := order.OrderAutomationRule{
		TenantID: s.TenantID, Name: "DEMO-采购签收自动通知发货", Priority: 3, Enabled: true,
		TriggerEvent: order.AutomationEventProcurementDelivered,
		Action:       order.AutomationActionNotifyShipping,
		Platforms:    mustJSONStrings(shops[0].Platform),
	}
	if err := tx.Create(&notifyRule).Error; err != nil {
		return fmt.Errorf("demoseed: automation notify rule: %w", err)
	}
	disabledRule := order.OrderAutomationRule{
		TenantID: s.TenantID, Name: "DEMO-揽收自动通知（停用示例）", Priority: 4,
		TriggerEvent: order.AutomationEventLogisticsCollected,
		Action:       order.AutomationActionNotifyShipping,
	}
	if err := tx.Create(&disabledRule).Error; err != nil {
		return fmt.Errorf("demoseed: automation disabled rule: %w", err)
	}
	if err := tx.Model(&order.OrderAutomationRule{}).Where("id = ?", disabledRule.ID).
		Update("enabled", false).Error; err != nil {
		return err
	}
	count("order_automation_rules", 4)

	samples := []struct {
		orderNo string
		amount  float64
		review  string
		rule    *order.OrderAutomationRule
		status  string
		reason  string
	}{
		{"DEMO-AT-1001", 68, order.ReviewStatusAutoPassed, &payRule,
			order.AutomationLogSuccess, "已自动确认付款（低风险条件）"},
		{"DEMO-AT-1002", 120, order.ReviewStatusAutoPassed, &genRule,
			order.AutomationLogFailed, "执行失败（本轮尝试 3 次）：生成采购单被阻断：SKU 未匹配货源"},
		{"DEMO-AT-1003", 88, order.ReviewStatusPending, &payRule,
			order.AutomationLogSkipped, "订单审单待审/挂起，按安全边界跳过自动化"},
	}
	for i, sp := range samples {
		created := now.Add(-time.Duration(i+1) * time.Hour)
		o := order.Order{
			TenantID: s.TenantID, Platform: shops[0].Platform, ShopID: &shops[0].ID,
			OrderNo: sp.orderNo, CustomerName: "DEMO-自动化买家", CustomerPhone: "13800000119",
			Status: order.StatusPending, ReviewStatus: sp.review,
			PaymentStatus: order.PaymentUnpaid, FulfillmentStatus: order.FulfillmentUnfulfilled,
			Currency: "CNY", TotalAmount: sp.amount, OrderedAt: &created,
		}
		if err := tx.Create(&o).Error; err != nil {
			return fmt.Errorf("demoseed: automation sample order %s: %w", sp.orderNo, err)
		}
		item := order.OrderItem{
			OrderID: o.ID, ProductTitle: "DEMO-自动化演示商品", SKUCode: "DEMO-AT-SKU",
			Quantity: 1, UnitPrice: sp.amount, TotalPrice: sp.amount,
		}
		if err := tx.Create(&item).Error; err != nil {
			return fmt.Errorf("demoseed: automation sample item: %w", err)
		}
		attempts := 1
		if sp.status == order.AutomationLogFailed {
			attempts = 3
		}
		log := order.OrderAutomationLog{
			TenantID: s.TenantID, RuleID: sp.rule.ID, RuleName: sp.rule.Name,
			OrderID: o.ID, OrderNo: o.OrderNo, ShopID: o.ShopID,
			TriggerEvent: sp.rule.TriggerEvent, Action: sp.rule.Action,
			Status: sp.status, Reason: sp.reason, Attempts: attempts,
			DedupKey: fmt.Sprintf("%d:%s:%s:%s", s.TenantID, sp.rule.ID, o.ID, sp.rule.TriggerEvent),
		}
		if err := tx.Create(&log).Error; err != nil {
			return fmt.Errorf("demoseed: automation sample log: %w", err)
		}
		count("orders", 1)
		count("order_items", 1)
		count("order_automation_logs", 1)
	}

	// 正向样本：本地 SKU 已匹配 + 主货源/映射齐全的未付款订单。demo 中批量
	// 「标记已付款」即可真实触发「自动生成采购单」成功动线（开箱可演示）。
	if len(products) > 0 && len(skus) > 0 {
		amount := 129.0
		created := now.Add(-30 * time.Minute)
		matched := order.Order{
			TenantID: s.TenantID, Platform: shops[0].Platform, ShopID: &shops[0].ID,
			OrderNo: "DEMO-AT-1004", CustomerName: "DEMO-自动化买家", CustomerPhone: "13800000119",
			Status: order.StatusPending, ReviewStatus: order.ReviewStatusAutoPassed,
			PaymentStatus: order.PaymentUnpaid, FulfillmentStatus: order.FulfillmentUnfulfilled,
			Currency: "CNY", TotalAmount: amount, OrderedAt: &created,
		}
		if err := tx.Create(&matched).Error; err != nil {
			return fmt.Errorf("demoseed: automation matched order: %w", err)
		}
		matchedItem := order.OrderItem{
			OrderID: matched.ID, ProductID: &products[0].ID, ProductSKUID: &skus[0].ID,
			ProductTitle: products[0].Title, SKUCode: skus[0].SKUCode, SKUName: skus[0].SKUName,
			Quantity: 1, UnitPrice: amount, TotalPrice: amount,
		}
		if err := tx.Create(&matchedItem).Error; err != nil {
			return fmt.Errorf("demoseed: automation matched item: %w", err)
		}
		count("orders", 1)
		count("order_items", 1)
	}
	return nil
}
