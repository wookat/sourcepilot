package demoseed

import (
	"fmt"
	"time"

	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"

	"github.com/trademind-ai/trademind/backend/internal/modules/admin"
	"github.com/trademind-ai/trademind/backend/internal/modules/order"
	"github.com/trademind-ai/trademind/backend/internal/modules/platformtenant"
	"github.com/trademind-ai/trademind/backend/internal/modules/shop"
	"github.com/trademind-ai/trademind/backend/internal/modules/waybill"
)

// Second demo tenant: an isolated business tenant with its own admin account
// and a small dataset (shop / orders / rules) so multi-tenant isolation is
// testable out of the box. Everything carries the DEMO- prefix (tenant name,
// admin display name, shop code, order numbers, rule names) so Cleanup /
// VerifyClean remove and check it like the rest of the demo dataset.
const (
	// DemoTenant2Name is the platform tenant row created for the second demo
	// business tenant (removed by Cleanup via the name prefix).
	DemoTenant2Name = "DEMO-第二租户"
	// DemoTenant2AdminEmail / DemoTenant2AdminPassword are the out-of-the-box
	// login for the second tenant. Must stay in sync with
	// docs/DEMO_SEEDING_GUIDE.md and docs/development.md.
	DemoTenant2AdminEmail    = "demo_tenant2_admin@trademind.local"
	DemoTenant2AdminPassword = "DemoTenant2Admin123!"
	demoTenant2AdminDisplay  = "DEMO-第二租户管理员"
)

// seedSecondTenant creates the second demo tenant, its admin account and a
// minimal business dataset (1 shop, 2 orders, 1 shipping rule, 1 automation
// rule). Runs after Cleanup inside the seed transaction, so re-seeding is
// idempotent (prior DEMO- tenant rows are removed first).
func (s *FullDemoSeeder) seedSecondTenant(tx *gorm.DB, res *FullDemoResult) error {
	count := func(table string, n int64) { res.Counts[table] += n }
	now := time.Now().UTC()

	tenant := platformtenant.Tenant{Name: DemoTenant2Name, Status: platformtenant.StatusActive}
	if err := tx.Create(&tenant).Error; err != nil {
		return fmt.Errorf("demoseed: second tenant: %w", err)
	}
	if tenant.ID == s.TenantID {
		// The tenants table had no row for the seeding tenant, so autoincrement
		// handed us its id; take the next one to keep the tenants distinct.
		if err := tx.Unscoped().Delete(&platformtenant.Tenant{}, tenant.ID).Error; err != nil {
			return fmt.Errorf("demoseed: second tenant id collision: %w", err)
		}
		tenant = platformtenant.Tenant{Name: DemoTenant2Name, Status: platformtenant.StatusActive}
		if err := tx.Create(&tenant).Error; err != nil {
			return fmt.Errorf("demoseed: second tenant: %w", err)
		}
	}
	count("tenants", 1)

	hash, err := bcrypt.GenerateFromPassword([]byte(DemoTenant2AdminPassword), bcrypt.DefaultCost)
	if err != nil {
		return fmt.Errorf("demoseed: hash tenant2 admin password: %w", err)
	}
	adminUser := admin.AdminUser{
		TenantID:     tenant.ID,
		Username:     admin.NewInternalUsername(),
		Email:        DemoTenant2AdminEmail,
		PasswordHash: string(hash),
		DisplayName:  demoTenant2AdminDisplay,
		Role:         "admin",
		Status:       "active",
	}
	if err := tx.Create(&adminUser).Error; err != nil {
		return fmt.Errorf("demoseed: second tenant admin: %w", err)
	}
	count("admin_users", 1)

	t2Shop := shop.Shop{TenantID: tenant.ID, Platform: "manual",
		ShopName: "DEMO-T2-第二租户演示店", ShopCode: "DEMO-T2-SHOP-1",
		Status: "active", AuthStatus: "authorized", Currency: "CNY",
		Remark: "DEMO- 第二租户演示店铺（种子数据）"}
	if err := tx.Create(&t2Shop).Error; err != nil {
		return fmt.Errorf("demoseed: second tenant shop: %w", err)
	}
	count("shops", 1)

	minAmt := 500.0
	t2ShippingRule := waybill.ShippingRule{TenantID: tenant.ID,
		Name: "DEMO-T2-高客单价订单走顺丰", Priority: 10, Enabled: true,
		MinAmount: &minAmt, CarrierCode: "sf"}
	if err := tx.Create(&t2ShippingRule).Error; err != nil {
		return fmt.Errorf("demoseed: second tenant shipping rule: %w", err)
	}
	count("shipping_rules", 1)

	payMax := 100.0
	t2AutoRule := order.OrderAutomationRule{
		TenantID: tenant.ID, Name: "DEMO-T2-低额订单自动确认付款", Priority: 1, Enabled: true,
		TriggerEvent: order.AutomationEventOrderCreated,
		Action:       order.AutomationActionConfirmPayment, MaxAmount: &payMax,
	}
	if err := tx.Create(&t2AutoRule).Error; err != nil {
		return fmt.Errorf("demoseed: second tenant automation rule: %w", err)
	}
	count("order_automation_rules", 1)

	orders := []struct {
		suffix  string
		status  string
		payment string
		amount  float64
	}{
		{suffix: "0001", status: order.StatusPending, payment: order.PaymentUnpaid, amount: 68},
		{suffix: "0002", status: order.StatusPaid, payment: order.PaymentPaid, amount: 618},
	}
	for i, plan := range orders {
		orderedAt := now.Add(-time.Duration(24-i*6) * time.Hour)
		o := order.Order{TenantID: tenant.ID, Platform: t2Shop.Platform, ShopID: &t2Shop.ID,
			OrderNo:      "DEMO-T2-SO-" + plan.suffix,
			CustomerName: fmt.Sprintf("DEMO-T2-买家%d", i+1), Status: plan.status,
			PaymentStatus: plan.payment, FulfillmentStatus: order.FulfillmentUnfulfilled,
			Currency: "CNY", TotalAmount: plan.amount, OrderedAt: &orderedAt,
			Remark: "DEMO- 第二租户演示订单（种子数据）"}
		if plan.payment == order.PaymentPaid {
			paidAt := orderedAt.Add(30 * time.Minute)
			o.PaidAt = &paidAt
		}
		if err := tx.Create(&o).Error; err != nil {
			return fmt.Errorf("demoseed: second tenant order: %w", err)
		}
		count("orders", 1)
		item := order.OrderItem{OrderID: o.ID,
			ProductTitle: "DEMO-T2-第二租户演示商品", SKUCode: "DEMO-T2-SKU-1",
			Quantity: 1, UnitPrice: plan.amount, TotalPrice: plan.amount}
		if err := tx.Create(&item).Error; err != nil {
			return fmt.Errorf("demoseed: second tenant order item: %w", err)
		}
		count("order_items", 1)
	}

	// 第二租户执行日志样本（成功/失败/跳过）：/orders/automation-logs 在
	// 第二租户视角不再空态，且覆盖 apply_shipping_rule recommend（仅推荐）
	// 模式的成功文案。日志经 rule_name / order_no 的 DEMO- 前缀被 Cleanup 清除。
	t2RecommendRule := order.OrderAutomationRule{
		TenantID: tenant.ID, Name: "DEMO-T2-付款后推荐物流商（仅推荐）", Priority: 2, Enabled: true,
		TriggerEvent:      order.AutomationEventOrderPaid,
		Action:            order.AutomationActionApplyShippingRule,
		ShippingApplyMode: order.ShippingApplyModeRecommend,
	}
	if err := tx.Create(&t2RecommendRule).Error; err != nil {
		return fmt.Errorf("demoseed: second tenant recommend rule: %w", err)
	}
	count("order_automation_rules", 1)

	logSamples := []struct {
		orderNo string
		amount  float64
		review  string
		rule    *order.OrderAutomationRule
		status  string
		reason  string
	}{
		{"DEMO-T2-AT-0001", 668, order.ReviewStatusAutoPassed, &t2RecommendRule,
			order.AutomationLogSuccess,
			"已按发货规则「DEMO-T2-高客单价订单走顺丰」推荐物流商：顺丰速运（仅推荐，发货时人工确认）"},
		{"DEMO-T2-AT-0002", 120, order.ReviewStatusAutoPassed, &t2AutoRule,
			order.AutomationLogFailed,
			"执行失败（本轮尝试 3 次）：自动确认付款被阻断：订单金额超出规则上限"},
		{"DEMO-T2-AT-0003", 88, order.ReviewStatusPending, &t2AutoRule,
			order.AutomationLogSkipped, "订单审单待审/挂起，按安全边界跳过自动化"},
	}
	for i, sp := range logSamples {
		created := now.Add(-time.Duration(i+1) * time.Hour)
		o := order.Order{TenantID: tenant.ID, Platform: t2Shop.Platform, ShopID: &t2Shop.ID,
			OrderNo:      sp.orderNo,
			CustomerName: "DEMO-T2-自动化买家", Status: order.StatusPending,
			ReviewStatus:  sp.review,
			PaymentStatus: order.PaymentUnpaid, FulfillmentStatus: order.FulfillmentUnfulfilled,
			Currency: "CNY", TotalAmount: sp.amount, OrderedAt: &created,
			Remark: "DEMO- 第二租户自动化演示订单（种子数据）"}
		if sp.status == order.AutomationLogSuccess {
			o.PlannedCarrierCode = "sf"
			o.PlannedCarrierName = "顺丰速运"
			o.PlannedCarrierMode = order.ShippingApplyModeRecommend
			o.PlannedCarrierRule = t2ShippingRule.Name
			o.PlannedCarrierAt = &created
		}
		if err := tx.Create(&o).Error; err != nil {
			return fmt.Errorf("demoseed: second tenant automation order %s: %w", sp.orderNo, err)
		}
		item := order.OrderItem{OrderID: o.ID,
			ProductTitle: "DEMO-T2-第二租户演示商品", SKUCode: "DEMO-T2-SKU-1",
			Quantity: 1, UnitPrice: sp.amount, TotalPrice: sp.amount}
		if err := tx.Create(&item).Error; err != nil {
			return fmt.Errorf("demoseed: second tenant automation item: %w", err)
		}
		attempts := 1
		if sp.status == order.AutomationLogFailed {
			attempts = 3
		}
		log := order.OrderAutomationLog{
			TenantID: tenant.ID, RuleID: sp.rule.ID, RuleName: sp.rule.Name,
			OrderID: o.ID, OrderNo: o.OrderNo, ShopID: o.ShopID,
			TriggerEvent: sp.rule.TriggerEvent, Action: sp.rule.Action,
			Status: sp.status, Reason: sp.reason, Attempts: attempts,
			DedupKey: fmt.Sprintf("%d:%s:%s:%s", tenant.ID, sp.rule.ID, o.ID, sp.rule.TriggerEvent),
		}
		if err := tx.Create(&log).Error; err != nil {
			return fmt.Errorf("demoseed: second tenant automation log: %w", err)
		}
		count("orders", 1)
		count("order_items", 1)
		count("order_automation_logs", 1)
	}
	return nil
}

// cleanupSecondTenant removes prefixed demo tenant rows plus the demo admin
// accounts inside them (and their store grants). Business rows (orders,
// shops, rules) are already removed by the prefix-based cleanup passes.
func cleanupSecondTenant(tx *gorm.DB, res *FullDemoResult, like string) error {
	del := func(table string, q *gorm.DB) error {
		if q.Error != nil {
			return fmt.Errorf("demoseed cleanup %s: %w", table, q.Error)
		}
		res.Counts[table] += q.RowsAffected
		return nil
	}
	var demoTenantIDs []int64
	if err := tx.Model(&platformtenant.Tenant{}).Unscoped().
		Where("name LIKE ?", like).Pluck("id", &demoTenantIDs).Error; err != nil {
		return err
	}
	userCond := tx.Model(&admin.AdminUser{}).Unscoped().Where("display_name LIKE ?", like)
	if len(demoTenantIDs) > 0 {
		userCond = tx.Model(&admin.AdminUser{}).Unscoped().
			Where("tenant_id IN ? OR display_name LIKE ?", demoTenantIDs, like)
	}
	var demoUserIDs []string
	if err := userCond.Pluck("id", &demoUserIDs).Error; err != nil {
		return err
	}
	if len(demoUserIDs) > 0 {
		if err := del("user_store_permissions", tx.Unscoped().
			Where("user_id IN ?", demoUserIDs).Delete(&admin.UserStorePermission{})); err != nil {
			return err
		}
		if err := del("admin_users", tx.Unscoped().
			Where("id IN ?", demoUserIDs).Delete(&admin.AdminUser{})); err != nil {
			return err
		}
	}
	if len(demoTenantIDs) > 0 {
		if err := del("tenants", tx.Unscoped().
			Where("id IN ?", demoTenantIDs).Delete(&platformtenant.Tenant{})); err != nil {
			return err
		}
	}
	return nil
}

// secondTenantVerifyChecks counts residual second-tenant rows (tenant rows by
// name prefix, demo admin accounts by display-name prefix); both must be zero
// after cleanup.
func secondTenantVerifyChecks(tx *gorm.DB, like string) []verifyCheck {
	return []verifyCheck{
		{table: "tenants", count: func() (int64, error) {
			var n int64
			if !tx.Migrator().HasTable("tenants") {
				return 0, nil
			}
			return n, tx.Model(&platformtenant.Tenant{}).Unscoped().
				Where("name LIKE ?", like).Count(&n).Error
		}},
		{table: "admin_users", count: func() (int64, error) {
			var n int64
			return n, tx.Model(&admin.AdminUser{}).Unscoped().
				Where("display_name LIKE ?", like).Count(&n).Error
		}},
	}
}
