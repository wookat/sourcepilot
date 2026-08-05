package demoseed

import (
	"fmt"
	"time"

	"gorm.io/gorm"

	"github.com/trademind-ai/trademind/backend/internal/modules/order"
	"github.com/trademind-ai/trademind/backend/internal/modules/shop"
)

// seedRound114OrderReview adds demo审单规则 plus hit-sample orders so the
// review workbench (待审/挂起, 命中原因, 放行/拒绝) demos out of the box.
// Everything is DEMO- prefixed and removed by Cleanup / checked by VerifyClean.
func (s *FullDemoSeeder) seedRound114OrderReview(tx *gorm.DB, res *FullDemoResult, now time.Time, shops []shop.Shop) error {
	if len(shops) == 0 {
		return nil
	}
	count := func(table string, n int64) { res.Counts[table] += n }

	minAmount := 500.0
	holdRule := order.OrderReviewRule{
		TenantID: s.TenantID, Name: "DEMO-超高额订单挂起", Priority: 1, Enabled: true,
		Action: order.ReviewActionHold, MinAmount: &minAmount,
	}
	if err := tx.Create(&holdRule).Error; err != nil {
		return fmt.Errorf("demoseed: review hold rule: %w", err)
	}
	reviewMin := 100.0
	reviewRule := order.OrderReviewRule{
		TenantID: s.TenantID, Name: "DEMO-高额订单人工审核", Priority: 2, Enabled: true,
		Action: order.ReviewActionReview, MinAmount: &reviewMin,
	}
	if err := tx.Create(&reviewRule).Error; err != nil {
		return fmt.Errorf("demoseed: review pending rule: %w", err)
	}
	remarkRule := order.OrderReviewRule{
		TenantID: s.TenantID, Name: "DEMO-备注敏感词审核", Priority: 3, Enabled: true,
		Action: order.ReviewActionReview, RemarkKeywords: mustJSONStrings("改地址", "加急"),
	}
	if err := tx.Create(&remarkRule).Error; err != nil {
		return fmt.Errorf("demoseed: review remark rule: %w", err)
	}
	disabledRule := order.OrderReviewRule{
		TenantID: s.TenantID, Name: "DEMO-黑名单地区拦截（停用示例）", Priority: 4,
		Action: order.ReviewActionHold, AddressKeywords: mustJSONStrings("演示黑名单区"),
	}
	if err := tx.Create(&disabledRule).Error; err != nil {
		return fmt.Errorf("demoseed: review disabled rule: %w", err)
	}
	if err := tx.Model(&order.OrderReviewRule{}).Where("id = ?", disabledRule.ID).
		Update("enabled", false).Error; err != nil {
		return err
	}
	count("order_review_rules", 4)

	samples := []struct {
		orderNo string
		amount  float64
		remark  string
		status  string
		rule    *order.OrderReviewRule
		reason  string
	}{
		{"DEMO-RV-1001", 880, "", order.ReviewStatusHeld, &holdRule, "订单金额 880.00 落入阈值区间"},
		{"DEMO-RV-1002", 260, "", order.ReviewStatusPending, &reviewRule, "订单金额 260.00 落入阈值区间"},
		{"DEMO-RV-1003", 45, "麻烦改地址到新收货点", order.ReviewStatusPending, &remarkRule, "买家备注含关键词「改地址」"},
	}
	for i, sp := range samples {
		created := now.Add(-time.Duration(i+1) * time.Hour)
		o := order.Order{
			TenantID: s.TenantID, Platform: shops[0].Platform, ShopID: &shops[0].ID,
			OrderNo: sp.orderNo, CustomerName: "DEMO-审单买家", CustomerPhone: "13800000114",
			Status: order.StatusPaid, ReviewStatus: sp.status,
			PaymentStatus: order.PaymentPaid, FulfillmentStatus: order.FulfillmentUnfulfilled,
			Currency: "CNY", TotalAmount: sp.amount, Remark: sp.remark,
			OrderedAt: &created,
		}
		if err := tx.Create(&o).Error; err != nil {
			return fmt.Errorf("demoseed: review sample order %s: %w", sp.orderNo, err)
		}
		item := order.OrderItem{
			OrderID: o.ID, ProductTitle: "DEMO-审单演示商品", SKUCode: "DEMO-RV-SKU",
			Quantity: 1, UnitPrice: sp.amount, TotalPrice: sp.amount,
		}
		if err := tx.Create(&item).Error; err != nil {
			return fmt.Errorf("demoseed: review sample item: %w", err)
		}
		hit := order.OrderReviewHit{
			TenantID: s.TenantID, OrderID: o.ID,
			RuleID: sp.rule.ID, RuleName: sp.rule.Name, Action: sp.rule.Action,
			Reason: sp.reason, Decisive: true,
		}
		if err := tx.Create(&hit).Error; err != nil {
			return fmt.Errorf("demoseed: review sample hit: %w", err)
		}
		count("orders", 1)
		count("order_items", 1)
		count("order_review_hits", 1)
	}
	return nil
}
