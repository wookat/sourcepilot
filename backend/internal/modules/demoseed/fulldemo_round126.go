package demoseed

import (
	"fmt"
	"time"

	"gorm.io/gorm"

	"github.com/trademind-ai/trademind/backend/internal/modules/order"
	"github.com/trademind-ai/trademind/backend/internal/modules/product"
	"github.com/trademind-ai/trademind/backend/internal/modules/shop"
)

// seedRound126AutoActions adds demo自动化规则 for the R126 actions（自动应用发货
// 规则 / 自动分仓）plus execution-log samples (成功落物流商 / 库存不足失败留痕)
// so the automation pages demo the new actions out of the box. Everything is
// DEMO- prefixed and removed by Cleanup / checked by VerifyClean.
func (s *FullDemoSeeder) seedRound126AutoActions(tx *gorm.DB, res *FullDemoResult, now time.Time, shops []shop.Shop, products []product.Product, skus []product.ProductSKU) error {
	if len(shops) == 0 {
		return nil
	}
	count := func(table string, n int64) { res.Counts[table] += n }

	carrierRule := order.OrderAutomationRule{
		TenantID: s.TenantID, Name: "DEMO-付款后自动应用发货规则", Priority: 5, Enabled: true,
		TriggerEvent:      order.AutomationEventOrderPaid,
		Action:            order.AutomationActionApplyShippingRule,
		ShippingApplyMode: order.ShippingApplyModeApply,
	}
	if err := tx.Create(&carrierRule).Error; err != nil {
		return fmt.Errorf("demoseed: automation carrier rule: %w", err)
	}
	warehouseRule := order.OrderAutomationRule{
		TenantID: s.TenantID, Name: "DEMO-付款后自动分仓（库存充足优先）", Priority: 6, Enabled: true,
		TriggerEvent:      order.AutomationEventOrderPaid,
		Action:            order.AutomationActionAssignWarehouse,
		WarehouseStrategy: order.AutomationWarehouseStrategyStockFirst,
	}
	if err := tx.Create(&warehouseRule).Error; err != nil {
		return fmt.Errorf("demoseed: automation warehouse rule: %w", err)
	}
	count("order_automation_rules", 2)

	// 成功样本：高客单价订单已按 DEMO 发货规则落物流商（顺丰）。
	planAt := now.Add(-50 * time.Minute)
	applied := order.Order{
		TenantID: s.TenantID, Platform: shops[0].Platform, ShopID: &shops[0].ID,
		OrderNo: "DEMO-AT-1201", CustomerName: "DEMO-自动化买家", CustomerPhone: "13800000126",
		Status: order.StatusPaid, ReviewStatus: order.ReviewStatusAutoPassed,
		PaymentStatus: order.PaymentPaid, FulfillmentStatus: order.FulfillmentUnfulfilled,
		Currency: "CNY", TotalAmount: 618, OrderedAt: &planAt, PaidAt: &planAt,
		PlannedCarrierCode: "sf", PlannedCarrierName: "顺丰速运",
		PlannedCarrierMode: order.ShippingApplyModeApply,
		PlannedCarrierRule: "DEMO-高客单价订单走顺丰", PlannedCarrierAt: &planAt,
	}
	if err := tx.Create(&applied).Error; err != nil {
		return fmt.Errorf("demoseed: applied carrier order: %w", err)
	}
	appliedItem := order.OrderItem{
		OrderID: applied.ID, ProductTitle: "DEMO-自动化演示商品", SKUCode: "DEMO-AT-SKU",
		Quantity: 1, UnitPrice: 618, TotalPrice: 618,
	}
	if err := tx.Create(&appliedItem).Error; err != nil {
		return fmt.Errorf("demoseed: applied carrier item: %w", err)
	}
	appliedLog := order.OrderAutomationLog{
		TenantID: s.TenantID, RuleID: carrierRule.ID, RuleName: carrierRule.Name,
		OrderID: applied.ID, OrderNo: applied.OrderNo,
		TriggerEvent: carrierRule.TriggerEvent, Action: carrierRule.Action,
		Status:   order.AutomationLogSuccess,
		Reason:   "已按发货规则「DEMO-高客单价订单走顺丰」应用物流商：顺丰速运（发货时仍可人工改选）",
		Attempts: 1,
		DedupKey: fmt.Sprintf("%d:%s:%s:%s", s.TenantID, carrierRule.ID, applied.ID, carrierRule.TriggerEvent),
	}
	if err := tx.Create(&appliedLog).Error; err != nil {
		return fmt.Errorf("demoseed: applied carrier log: %w", err)
	}

	// 失败样本：自动分仓因库存不足留痕失败（负向可见，可重试）。
	shortAt := now.Add(-40 * time.Minute)
	short := order.Order{
		TenantID: s.TenantID, Platform: shops[0].Platform, ShopID: &shops[0].ID,
		OrderNo: "DEMO-AT-1202", CustomerName: "DEMO-自动化买家", CustomerPhone: "13800000126",
		Status: order.StatusPaid, ReviewStatus: order.ReviewStatusAutoPassed,
		PaymentStatus: order.PaymentPaid, FulfillmentStatus: order.FulfillmentUnfulfilled,
		Currency: "CNY", TotalAmount: 208, OrderedAt: &shortAt, PaidAt: &shortAt,
	}
	if err := tx.Create(&short).Error; err != nil {
		return fmt.Errorf("demoseed: short-stock order: %w", err)
	}
	shortQty := 999
	shortItem := order.OrderItem{
		OrderID: short.ID, ProductTitle: "DEMO-自动化演示商品", SKUCode: "DEMO-AT-SKU",
		Quantity: shortQty, UnitPrice: 0.2, TotalPrice: 208,
	}
	if len(products) > 0 && len(skus) > 0 {
		shortItem.ProductID = &products[0].ID
		shortItem.ProductSKUID = &skus[0].ID
		shortItem.ProductTitle = products[0].Title
		shortItem.SKUCode = skus[0].SKUCode
		shortItem.SKUName = skus[0].SKUName
	}
	if err := tx.Create(&shortItem).Error; err != nil {
		return fmt.Errorf("demoseed: short-stock item: %w", err)
	}
	shortLog := order.OrderAutomationLog{
		TenantID: s.TenantID, RuleID: warehouseRule.ID, RuleName: warehouseRule.Name,
		OrderID: short.ID, OrderNo: short.OrderNo,
		TriggerEvent: warehouseRule.TriggerEvent, Action: warehouseRule.Action,
		Status:   order.AutomationLogFailed,
		Reason:   fmt.Sprintf("执行失败（本轮尝试 3 次）：库存不足，无法分配发货仓：所有仓库均无法整单覆盖（如 %s：%s 需 %d 件）", "默认仓", shortItem.SKUCode, shortQty),
		Attempts: 3,
		DedupKey: fmt.Sprintf("%d:%s:%s:%s", s.TenantID, warehouseRule.ID, short.ID, warehouseRule.TriggerEvent),
	}
	if err := tx.Create(&shortLog).Error; err != nil {
		return fmt.Errorf("demoseed: short-stock log: %w", err)
	}
	count("orders", 2)
	count("order_items", 2)
	count("order_automation_logs", 2)
	return nil
}
