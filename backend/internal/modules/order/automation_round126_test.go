package order_test

import (
	"context"
	"fmt"
	"testing"

	"github.com/google/uuid"
	"github.com/trademind-ai/trademind/backend/internal/modules/order"
	"github.com/trademind-ai/trademind/backend/internal/modules/waybill"
	"gorm.io/gorm"
)

func openRound126TestDB(t *testing.T) *gorm.DB {
	t.Helper()
	db := openAutomationTestDB(t)
	if err := db.AutoMigrate(&waybill.ShippingRule{}); err != nil {
		t.Fatal(err)
	}
	return db
}

func newShippingRule(t *testing.T, db *gorm.DB, tenantID int64, name, carrierCode string) {
	t.Helper()
	if err := db.Create(&waybill.ShippingRule{
		TenantID: tenantID, Name: name, Priority: 10, Enabled: true, CarrierCode: carrierCode,
	}).Error; err != nil {
		t.Fatal(err)
	}
}

func TestAutomationRound126RuleValidation(t *testing.T) {
	db := openRound126TestDB(t)
	svc := &order.Service{DB: db}
	c := automationTestCtx(1)

	// Invalid parameter values are rejected.
	bad := "bogus"
	if _, err := svc.CreateAutomationRule(c, order.AutomationRuleBody{
		Name: "规则", TriggerEvent: order.AutomationEventOrderPaid,
		Action: order.AutomationActionApplyShippingRule, ShippingApplyMode: &bad,
	}, nil); err == nil {
		t.Fatal("expected error for invalid shipping apply mode")
	}
	if _, err := svc.CreateAutomationRule(c, order.AutomationRuleBody{
		Name: "规则", TriggerEvent: order.AutomationEventOrderPaid,
		Action: order.AutomationActionAssignWarehouse, WarehouseStrategy: &bad,
	}, nil); err == nil {
		t.Fatal("expected error for invalid warehouse strategy")
	}
	// assign_warehouse is not allowed on order_created.
	if _, err := svc.CreateAutomationRule(c, order.AutomationRuleBody{
		Name: "规则", TriggerEvent: order.AutomationEventOrderCreated,
		Action: order.AutomationActionAssignWarehouse,
	}, nil); err == nil {
		t.Fatal("expected event/action mismatch for assign_warehouse on order_created")
	}
	// Defaults land when the parameter is omitted.
	shipRule := createAutomationRule(t, db, svc, 1, order.AutomationRuleBody{
		Name: "自动应用发货规则", TriggerEvent: order.AutomationEventOrderPaid,
		Action: order.AutomationActionApplyShippingRule,
	})
	if shipRule.ShippingApplyMode != order.ShippingApplyModeRecommend {
		t.Fatalf("expected default recommend mode, got %q", shipRule.ShippingApplyMode)
	}
	whRule := createAutomationRule(t, db, svc, 1, order.AutomationRuleBody{
		Name: "自动分仓", TriggerEvent: order.AutomationEventOrderPaid,
		Action: order.AutomationActionAssignWarehouse,
	})
	if whRule.WarehouseStrategy != order.AutomationWarehouseStrategyDefault {
		t.Fatalf("expected default warehouse strategy, got %q", whRule.WarehouseStrategy)
	}
}

func TestAutomationApplyShippingRule(t *testing.T) {
	db := openRound126TestDB(t)
	svc := &order.Service{DB: db, Waybill: &waybill.Service{DB: db}}
	newShippingRule(t, db, 1, "高客单价走顺丰", "sf")
	mode := order.ShippingApplyModeApply
	createAutomationRule(t, db, svc, 1, order.AutomationRuleBody{
		Name: "付款后自动应用发货规则", TriggerEvent: order.AutomationEventOrderPaid,
		Action: order.AutomationActionApplyShippingRule, ShippingApplyMode: &mode,
	})

	o := newAutomationOrder(t, db, 1, 60, "")
	svc.FireOrderEvent(context.Background(), 1, o.ID, order.AutomationEventOrderPaid)
	var after order.Order
	if err := db.First(&after, "id = ?", o.ID).Error; err != nil {
		t.Fatal(err)
	}
	if after.PlannedCarrierCode != "sf" || after.PlannedCarrierMode != order.ShippingApplyModeApply ||
		after.PlannedCarrierRule != "高客单价走顺丰" || after.PlannedCarrierAt == nil {
		t.Fatalf("expected planned carrier landed, got %+v", after)
	}
	rows := logsFor(t, db, o.ID)
	if len(rows) != 1 || rows[0].Status != order.AutomationLogSuccess {
		t.Fatalf("expected one success log, got %+v", rows)
	}

	// Duplicate event: dedup, no extra logs.
	svc.FireOrderEvent(context.Background(), 1, o.ID, order.AutomationEventOrderPaid)
	if rows := logsFor(t, db, o.ID); len(rows) != 1 {
		t.Fatalf("expected 1 log after duplicate event, got %d", len(rows))
	}

	// Manual override preserved: an order that already has a plan is skipped
	// by a different rule/event instead of being overwritten.
	createAutomationRule(t, db, svc, 1, order.AutomationRuleBody{
		Name: "签收后再应用发货规则", TriggerEvent: order.AutomationEventProcurementDelivered,
		Action: order.AutomationActionApplyShippingRule,
	})
	if err := db.Model(&order.Order{}).Where("id = ?", o.ID).Updates(map[string]any{
		"planned_carrier_code": "yto", "planned_carrier_name": "圆通",
	}).Error; err != nil {
		t.Fatal(err)
	}
	svc.FireOrderEvent(context.Background(), 1, o.ID, order.AutomationEventProcurementDelivered)
	rows = logsFor(t, db, o.ID)
	if len(rows) != 2 || rows[1].Status != order.AutomationLogSkipped {
		t.Fatalf("expected skipped log for existing plan, got %+v", rows)
	}
	if err := db.First(&after, "id = ?", o.ID).Error; err != nil {
		t.Fatal(err)
	}
	if after.PlannedCarrierCode != "yto" {
		t.Fatalf("manual carrier choice must be preserved, got %q", after.PlannedCarrierCode)
	}
}

func TestAutomationApplyShippingRuleNoMatch(t *testing.T) {
	db := openRound126TestDB(t)
	svc := &order.Service{DB: db, Waybill: &waybill.Service{DB: db}}
	createAutomationRule(t, db, svc, 1, order.AutomationRuleBody{
		Name: "付款后自动应用发货规则", TriggerEvent: order.AutomationEventOrderPaid,
		Action: order.AutomationActionApplyShippingRule,
	})
	o := newAutomationOrder(t, db, 1, 60, "")
	svc.FireOrderEvent(context.Background(), 1, o.ID, order.AutomationEventOrderPaid)
	rows := logsFor(t, db, o.ID)
	if len(rows) != 1 || rows[0].Status != order.AutomationLogSkipped {
		t.Fatalf("expected skipped log when no shipping rule matches, got %+v", rows)
	}
	var after order.Order
	if err := db.First(&after, "id = ?", o.ID).Error; err != nil {
		t.Fatal(err)
	}
	if after.PlannedCarrierCode != "" {
		t.Fatalf("expected no planned carrier, got %q", after.PlannedCarrierCode)
	}
}

func TestAutomationApplyShippingRuleBlockedByReview(t *testing.T) {
	db := openRound126TestDB(t)
	svc := &order.Service{DB: db, Waybill: &waybill.Service{DB: db}}
	newShippingRule(t, db, 1, "默认走顺丰", "sf")
	createAutomationRule(t, db, svc, 1, order.AutomationRuleBody{
		Name: "付款后自动应用发货规则", TriggerEvent: order.AutomationEventOrderPaid,
		Action: order.AutomationActionApplyShippingRule,
	})
	o := newAutomationOrder(t, db, 1, 60, order.ReviewStatusPending)
	svc.FireOrderEvent(context.Background(), 1, o.ID, order.AutomationEventOrderPaid)
	rows := logsFor(t, db, o.ID)
	if len(rows) != 1 || rows[0].Status != order.AutomationLogSkipped {
		t.Fatalf("expected review-blocked skip, got %+v", rows)
	}
	var after order.Order
	if err := db.First(&after, "id = ?", o.ID).Error; err != nil {
		t.Fatal(err)
	}
	if after.PlannedCarrierCode != "" {
		t.Fatal("review-blocked order must not get a carrier plan")
	}
}

func newOrderItemWithSKU(t *testing.T, db *gorm.DB, orderID uuid.UUID, qty int) uuid.UUID {
	t.Helper()
	skuID := uuid.New()
	if err := db.Create(&order.OrderItem{
		OrderID: orderID, ProductSKUID: &skuID, SKUCode: "SKU-1",
		ProductTitle: "商品", Quantity: qty,
	}).Error; err != nil {
		t.Fatal(err)
	}
	return skuID
}

func TestAutomationAssignWarehouse(t *testing.T) {
	db := openRound126TestDB(t)
	whID := uuid.New()
	planCalls := 0
	svc := &order.Service{DB: db, Automation: &order.AutomationHooks{
		PlanWarehouse: func(ctx context.Context, tenantID int64, strategy string, demands []order.AutomationWarehouseDemand) (*order.AutomationWarehousePlan, error) {
			planCalls++
			if tenantID != 1 {
				t.Fatalf("unexpected tenant %d", tenantID)
			}
			if strategy != order.AutomationWarehouseStrategyStockFirst {
				t.Fatalf("unexpected strategy %q", strategy)
			}
			if len(demands) != 1 || demands[0].Quantity != 2 {
				t.Fatalf("unexpected demands %+v", demands)
			}
			return &order.AutomationWarehousePlan{WarehouseID: whID, WarehouseName: "华南仓"}, nil
		},
	}}
	strategy := order.AutomationWarehouseStrategyStockFirst
	createAutomationRule(t, db, svc, 1, order.AutomationRuleBody{
		Name: "付款后自动分仓", TriggerEvent: order.AutomationEventOrderPaid,
		Action: order.AutomationActionAssignWarehouse, WarehouseStrategy: &strategy,
	})
	o := newAutomationOrder(t, db, 1, 60, "")
	newOrderItemWithSKU(t, db, o.ID, 2)
	svc.FireOrderEvent(context.Background(), 1, o.ID, order.AutomationEventOrderPaid)
	var after order.Order
	if err := db.First(&after, "id = ?", o.ID).Error; err != nil {
		t.Fatal(err)
	}
	if after.AssignedWarehouseID == nil || *after.AssignedWarehouseID != whID ||
		after.AssignedWarehouseName != "华南仓" ||
		after.AssignedWarehouseStrategy != strategy || after.WarehouseAssignedAt == nil {
		t.Fatalf("expected assigned warehouse, got %+v", after)
	}
	rows := logsFor(t, db, o.ID)
	if len(rows) != 1 || rows[0].Status != order.AutomationLogSuccess {
		t.Fatalf("expected one success log, got %+v", rows)
	}
	// Duplicate event: dedup, plan not re-run.
	svc.FireOrderEvent(context.Background(), 1, o.ID, order.AutomationEventOrderPaid)
	if planCalls != 1 {
		t.Fatalf("expected 1 plan call, got %d", planCalls)
	}
}

func TestAutomationAssignWarehouseInsufficientStock(t *testing.T) {
	db := openRound126TestDB(t)
	svc := &order.Service{DB: db, Automation: &order.AutomationHooks{
		PlanWarehouse: func(ctx context.Context, tenantID int64, strategy string, demands []order.AutomationWarehouseDemand) (*order.AutomationWarehousePlan, error) {
			return nil, fmt.Errorf("库存不足，无法分配发货仓：SKU-1 需 999 件")
		},
	}}
	createAutomationRule(t, db, svc, 1, order.AutomationRuleBody{
		Name: "付款后自动分仓", TriggerEvent: order.AutomationEventOrderPaid,
		Action: order.AutomationActionAssignWarehouse,
	})
	o := newAutomationOrder(t, db, 1, 60, "")
	newOrderItemWithSKU(t, db, o.ID, 999)
	svc.FireOrderEvent(context.Background(), 1, o.ID, order.AutomationEventOrderPaid)
	rows := logsFor(t, db, o.ID)
	if len(rows) != 1 || rows[0].Status != order.AutomationLogFailed {
		t.Fatalf("expected visible failed log, got %+v", rows)
	}
	var after order.Order
	if err := db.First(&after, "id = ?", o.ID).Error; err != nil {
		t.Fatal(err)
	}
	if after.AssignedWarehouseID != nil {
		t.Fatal("insufficient stock must not assign a warehouse")
	}
}

func TestAutomationAssignWarehouseNoMatchedSKU(t *testing.T) {
	db := openRound126TestDB(t)
	svc := &order.Service{DB: db, Automation: &order.AutomationHooks{
		PlanWarehouse: func(ctx context.Context, tenantID int64, strategy string, demands []order.AutomationWarehouseDemand) (*order.AutomationWarehousePlan, error) {
			t.Fatal("plan must not run for orders without matched SKU lines")
			return nil, nil
		},
	}}
	createAutomationRule(t, db, svc, 1, order.AutomationRuleBody{
		Name: "付款后自动分仓", TriggerEvent: order.AutomationEventOrderPaid,
		Action: order.AutomationActionAssignWarehouse,
	})
	o := newAutomationOrder(t, db, 1, 60, "")
	if err := db.Create(&order.OrderItem{
		OrderID: o.ID, ProductTitle: "未匹配商品", Quantity: 1,
	}).Error; err != nil {
		t.Fatal(err)
	}
	svc.FireOrderEvent(context.Background(), 1, o.ID, order.AutomationEventOrderPaid)
	rows := logsFor(t, db, o.ID)
	if len(rows) != 1 || rows[0].Status != order.AutomationLogSkipped {
		t.Fatalf("expected skipped log, got %+v", rows)
	}
}

func TestAutomationDryRunNewActions(t *testing.T) {
	db := openRound126TestDB(t)
	svc := &order.Service{DB: db, Waybill: &waybill.Service{DB: db}}
	newAutomationOrder(t, db, 1, 60, "")
	newAutomationOrder(t, db, 1, 50, order.ReviewStatusPending)

	for _, action := range []string{order.AutomationActionApplyShippingRule, order.AutomationActionAssignWarehouse} {
		res, err := svc.DryRunAutomationRule(automationTestCtx(1), order.AutomationRuleBody{
			Name: "dry", TriggerEvent: order.AutomationEventOrderPaid, Action: action,
		})
		if err != nil {
			t.Fatal(err)
		}
		if res.Scanned != 2 || res.Matched != 2 || res.Blocked != 1 {
			t.Fatalf("unexpected dry-run result for %s: %+v", action, res)
		}
	}
	var n int64
	if err := db.Model(&order.OrderAutomationLog{}).Count(&n).Error; err != nil {
		t.Fatal(err)
	}
	if n != 0 {
		t.Fatalf("dry run must not write logs, got %d", n)
	}
	var orders []order.Order
	if err := db.Find(&orders).Error; err != nil {
		t.Fatal(err)
	}
	for _, o := range orders {
		if o.PlannedCarrierCode != "" || o.AssignedWarehouseID != nil {
			t.Fatalf("dry run must not mutate orders: %+v", o)
		}
	}
}
