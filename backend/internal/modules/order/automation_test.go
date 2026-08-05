package order_test

import (
	"context"
	"fmt"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"github.com/google/uuid"
	"github.com/trademind-ai/trademind/backend/internal/modules/order"
	"github.com/trademind-ai/trademind/backend/internal/modules/shop"
	"github.com/trademind-ai/trademind/backend/internal/pkg/ctxkey"
	"gorm.io/gorm"
)

func openAutomationTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	dsn := fmt.Sprintf("file:order_automation_%s?mode=memory&cache=shared&_pragma=busy_timeout(5000)", uuid.New().String())
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Skipf("sqlite unavailable: %v", err)
	}
	sqlDB, _ := db.DB()
	sqlDB.SetMaxOpenConns(1)
	if err := db.AutoMigrate(&order.Order{}, &order.OrderItem{}, &order.OrderShipment{},
		&order.OrderAutomationRule{}, &order.OrderAutomationLog{}, &shop.Shop{}); err != nil {
		t.Fatal(err)
	}
	return db
}

func automationTestCtx(tenantID int64) *gin.Context {
	gin.SetMode(gin.TestMode)
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Request = httptest.NewRequest("POST", "/api/v1/order-automation-rules", nil)
	c.Set(ctxkey.TenantID, tenantID)
	return c
}

func newAutomationOrder(t *testing.T, db *gorm.DB, tenantID int64, amount float64, reviewStatus string) *order.Order {
	t.Helper()
	o := &order.Order{
		TenantID: tenantID, Platform: "tiktok", OrderNo: "AUTO-" + uuid.New().String()[:8],
		CustomerName: "买家", Status: order.StatusPending, ReviewStatus: reviewStatus,
		PaymentStatus: order.PaymentUnpaid, FulfillmentStatus: "unfulfilled",
		Currency: "USD", TotalAmount: amount,
	}
	if err := db.Create(o).Error; err != nil {
		t.Fatal(err)
	}
	return o
}

func createAutomationRule(t *testing.T, db *gorm.DB, svc *order.Service, tenantID int64, body order.AutomationRuleBody) *order.OrderAutomationRule {
	t.Helper()
	row, err := svc.CreateAutomationRule(automationTestCtx(tenantID), body, nil)
	if err != nil {
		t.Fatal(err)
	}
	return row
}

func intPtr(v int) *int         { return &v }
func f64Ptr(v float64) *float64 { return &v }
func boolPtrA(v bool) *bool     { return &v }
func logsFor(t *testing.T, db *gorm.DB, orderID uuid.UUID) []order.OrderAutomationLog {
	t.Helper()
	var rows []order.OrderAutomationLog
	if err := db.Where("order_id = ?", orderID).Order("created_at ASC").Find(&rows).Error; err != nil {
		t.Fatal(err)
	}
	return rows
}

func TestAutomationRuleValidation(t *testing.T) {
	db := openAutomationTestDB(t)
	svc := &order.Service{DB: db}
	c := automationTestCtx(1)

	if _, err := svc.CreateAutomationRule(c, order.AutomationRuleBody{
		TriggerEvent: order.AutomationEventOrderCreated, Action: order.AutomationActionConfirmPayment,
		MaxAmount: f64Ptr(100),
	}, nil); err == nil {
		t.Fatal("expected error for empty name")
	}
	if _, err := svc.CreateAutomationRule(c, order.AutomationRuleBody{
		Name: "规则", TriggerEvent: "bogus", Action: order.AutomationActionConfirmPayment,
	}, nil); err == nil {
		t.Fatal("expected error for invalid event")
	}
	if _, err := svc.CreateAutomationRule(c, order.AutomationRuleBody{
		Name: "规则", TriggerEvent: order.AutomationEventOrderCreated, Action: order.AutomationActionNotifyShipping,
	}, nil); err == nil {
		t.Fatal("expected error for event/action mismatch")
	}
	// confirm_payment低风险限定: MaxAmount required.
	if _, err := svc.CreateAutomationRule(c, order.AutomationRuleBody{
		Name: "规则", TriggerEvent: order.AutomationEventOrderCreated, Action: order.AutomationActionConfirmPayment,
	}, nil); err == nil {
		t.Fatal("expected error for confirm_payment without max amount")
	}
	row, err := svc.CreateAutomationRule(c, order.AutomationRuleBody{
		Name: "低额自动付款", TriggerEvent: order.AutomationEventOrderCreated,
		Action: order.AutomationActionConfirmPayment, MaxAmount: f64Ptr(100), Priority: intPtr(5),
	}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if row.Priority != 5 || !row.Enabled {
		t.Fatalf("unexpected rule row: %+v", row)
	}
	// Clearing the max amount from a confirm_payment rule must be rejected.
	if _, err := svc.UpdateAutomationRule(c, row.ID, order.AutomationRuleBody{ClearMaxAmount: true}, nil); err == nil {
		t.Fatal("expected error when clearing max amount on confirm_payment rule")
	}
}

func TestAutomationRuleTenantIsolation(t *testing.T) {
	db := openAutomationTestDB(t)
	svc := &order.Service{DB: db}
	row := createAutomationRule(t, db, svc, 1, order.AutomationRuleBody{
		Name: "T1规则", TriggerEvent: order.AutomationEventOrderCreated,
		Action: order.AutomationActionMarkPrinted, MaxAmount: f64Ptr(50),
	})

	// Cross-tenant update / delete must return not-found.
	if _, err := svc.UpdateAutomationRule(automationTestCtx(2), row.ID, order.AutomationRuleBody{Name: "x"}, nil); err != order.ErrAutomationRuleNotFound {
		t.Fatalf("expected not found, got %v", err)
	}
	if err := svc.DeleteAutomationRule(automationTestCtx(2), row.ID, nil); err != order.ErrAutomationRuleNotFound {
		t.Fatalf("expected not found, got %v", err)
	}
	// Tenant 2 rules never run for tenant 1 orders.
	o := newAutomationOrder(t, db, 2, 30, "")
	svc.FireOrderEvent(context.Background(), 2, o.ID, order.AutomationEventOrderCreated)
	if rows := logsFor(t, db, o.ID); len(rows) != 0 {
		t.Fatalf("expected no logs for other tenant, got %d", len(rows))
	}
}

func TestAutomationConfirmPaymentAndChain(t *testing.T) {
	db := openAutomationTestDB(t)
	generated := 0
	svc := &order.Service{DB: db, Automation: &order.AutomationHooks{
		GenerateProcurement: func(ctx context.Context, tenantID int64, orderID uuid.UUID, key string) (string, error) {
			generated++
			return "已自动生成 1 张采购单", nil
		},
	}}
	createAutomationRule(t, db, svc, 1, order.AutomationRuleBody{
		Name: "低额自动付款", TriggerEvent: order.AutomationEventOrderCreated,
		Action: order.AutomationActionConfirmPayment, MaxAmount: f64Ptr(100),
	})
	createAutomationRule(t, db, svc, 1, order.AutomationRuleBody{
		Name: "付款后自动采购", TriggerEvent: order.AutomationEventOrderPaid,
		Action: order.AutomationActionGenerateProcurement, MaxAmount: f64Ptr(500),
	})

	o := newAutomationOrder(t, db, 1, 60, "")
	svc.FireOrderEvent(context.Background(), 1, o.ID, order.AutomationEventOrderCreated)

	var after order.Order
	if err := db.First(&after, "id = ?", o.ID).Error; err != nil {
		t.Fatal(err)
	}
	if after.PaymentStatus != order.PaymentPaid || after.Status != order.StatusPaid || after.PaidAt == nil {
		t.Fatalf("expected auto-paid order, got %+v", after)
	}
	if generated != 1 {
		t.Fatalf("expected chained procurement generation once, got %d", generated)
	}
	rows := logsFor(t, db, o.ID)
	if len(rows) != 2 {
		t.Fatalf("expected 2 logs, got %d: %+v", len(rows), rows)
	}
	for _, r := range rows {
		if r.Status != order.AutomationLogSuccess {
			t.Fatalf("expected success, got %+v", r)
		}
	}

	// Duplicate event: idempotent, no re-execution, no extra logs.
	svc.FireOrderEvent(context.Background(), 1, o.ID, order.AutomationEventOrderCreated)
	svc.FireOrderEvent(context.Background(), 1, o.ID, order.AutomationEventOrderPaid)
	if generated != 1 {
		t.Fatalf("expected no re-generation on duplicate events, got %d", generated)
	}
	if rows := logsFor(t, db, o.ID); len(rows) != 2 {
		t.Fatalf("expected 2 logs after duplicate events, got %d", len(rows))
	}

	// Over-threshold order: conditions not matched, nothing runs.
	big := newAutomationOrder(t, db, 1, 900, "")
	svc.FireOrderEvent(context.Background(), 1, big.ID, order.AutomationEventOrderCreated)
	if rows := logsFor(t, db, big.ID); len(rows) != 0 {
		t.Fatalf("expected no logs for unmatched order, got %d", len(rows))
	}
}

func TestAutomationSkipOutcomeRecordsSkippedWithoutRetry(t *testing.T) {
	db := openAutomationTestDB(t)
	calls := 0
	svc := &order.Service{DB: db, Automation: &order.AutomationHooks{
		GenerateProcurement: func(ctx context.Context, tenantID int64, orderID uuid.UUID, key string) (string, error) {
			calls++
			return "", &order.AutomationSkip{Reason: "跳过生成采购单：订单不存在或没有商品行"}
		},
	}}
	createAutomationRule(t, db, svc, 1, order.AutomationRuleBody{
		Name: "付款后自动采购", TriggerEvent: order.AutomationEventOrderPaid,
		Action: order.AutomationActionGenerateProcurement, MaxAmount: f64Ptr(500),
	})
	o := newAutomationOrder(t, db, 1, 60, "")
	if err := db.Model(&order.Order{}).Where("id = ?", o.ID).
		Update("payment_status", order.PaymentPaid).Error; err != nil {
		t.Fatal(err)
	}
	svc.FireOrderEvent(context.Background(), 1, o.ID, order.AutomationEventOrderPaid)
	if calls != 1 {
		t.Fatalf("skip outcome must not be retried inline, got %d calls", calls)
	}
	rows := logsFor(t, db, o.ID)
	if len(rows) != 1 || rows[0].Status != order.AutomationLogSkipped || rows[0].Attempts != 1 {
		t.Fatalf("expected one skipped log with 1 attempt, got %+v", rows)
	}
	if rows[0].Reason != "跳过生成采购单：订单不存在或没有商品行" {
		t.Fatalf("unexpected skip reason: %q", rows[0].Reason)
	}
}

func TestAutomationSafetyBoundaryBlocksReview(t *testing.T) {
	db := openAutomationTestDB(t)
	svc := &order.Service{DB: db}
	createAutomationRule(t, db, svc, 1, order.AutomationRuleBody{
		Name: "低额自动付款", TriggerEvent: order.AutomationEventOrderCreated,
		Action: order.AutomationActionConfirmPayment, MaxAmount: f64Ptr(100),
	})
	for _, rs := range []string{order.ReviewStatusPending, order.ReviewStatusHeld} {
		o := newAutomationOrder(t, db, 1, 60, rs)
		svc.FireOrderEvent(context.Background(), 1, o.ID, order.AutomationEventOrderCreated)
		var after order.Order
		if err := db.First(&after, "id = ?", o.ID).Error; err != nil {
			t.Fatal(err)
		}
		if after.PaymentStatus != order.PaymentUnpaid {
			t.Fatalf("review-blocked order %s must not be automated", rs)
		}
		rows := logsFor(t, db, o.ID)
		if len(rows) != 1 || rows[0].Status != order.AutomationLogSkipped {
			t.Fatalf("expected one skipped log for %s, got %+v", rs, rows)
		}
	}
}

func TestAutomationRequireReviewPassedCondition(t *testing.T) {
	db := openAutomationTestDB(t)
	svc := &order.Service{DB: db}
	createAutomationRule(t, db, svc, 1, order.AutomationRuleBody{
		Name: "审单通过自动打单", TriggerEvent: order.AutomationEventOrderCreated,
		Action: order.AutomationActionMarkPrinted, RequireReviewPassed: boolPtrA(true), MaxAmount: f64Ptr(1000),
	})
	// Review status empty: condition not met, no execution.
	o1 := newAutomationOrder(t, db, 1, 60, "")
	svc.FireOrderEvent(context.Background(), 1, o1.ID, order.AutomationEventOrderCreated)
	if rows := logsFor(t, db, o1.ID); len(rows) != 0 {
		t.Fatalf("expected no logs, got %d", len(rows))
	}
	// Auto-passed review: rule runs.
	o2 := newAutomationOrder(t, db, 1, 60, order.ReviewStatusAutoPassed)
	svc.FireOrderEvent(context.Background(), 1, o2.ID, order.AutomationEventOrderCreated)
	var after order.Order
	if err := db.First(&after, "id = ?", o2.ID).Error; err != nil {
		t.Fatal(err)
	}
	if after.WaybillPrintedAt == nil {
		t.Fatal("expected auto mark printed")
	}
}

func TestAutomationFailureRetryAndManualRetry(t *testing.T) {
	db := openAutomationTestDB(t)
	fail := true
	calls := 0
	svc := &order.Service{DB: db, Automation: &order.AutomationHooks{
		GenerateProcurement: func(ctx context.Context, tenantID int64, orderID uuid.UUID, key string) (string, error) {
			calls++
			if fail {
				return "", fmt.Errorf("上游暂时不可用")
			}
			return "已自动生成 1 张采购单", nil
		},
	}}
	createAutomationRule(t, db, svc, 1, order.AutomationRuleBody{
		Name: "付款后自动采购", TriggerEvent: order.AutomationEventOrderPaid,
		Action: order.AutomationActionGenerateProcurement, MaxAmount: f64Ptr(500),
	})
	o := newAutomationOrder(t, db, 1, 60, "")
	if err := db.Model(&order.Order{}).Where("id = ?", o.ID).
		Update("payment_status", order.PaymentPaid).Error; err != nil {
		t.Fatal(err)
	}
	svc.FireOrderEvent(context.Background(), 1, o.ID, order.AutomationEventOrderPaid)
	if calls != 3 {
		t.Fatalf("expected 3 inline attempts, got %d", calls)
	}
	rows := logsFor(t, db, o.ID)
	if len(rows) != 1 || rows[0].Status != order.AutomationLogFailed || rows[0].Attempts != 3 {
		t.Fatalf("expected failed log with 3 attempts, got %+v", rows)
	}

	// Cross-tenant retry must 404.
	if _, err := svc.RetryAutomationLog(automationTestCtx(2), rows[0].ID, nil); err != order.ErrAutomationLogNotFound {
		t.Fatalf("expected not found, got %v", err)
	}
	// Manual retry succeeds once upstream recovers.
	fail = false
	updated, err := svc.RetryAutomationLog(automationTestCtx(1), rows[0].ID, nil)
	if err != nil {
		t.Fatal(err)
	}
	if updated.Status != order.AutomationLogSuccess {
		t.Fatalf("expected success after retry, got %+v", updated)
	}
	// Retrying a non-failed log is rejected.
	if _, err := svc.RetryAutomationLog(automationTestCtx(1), rows[0].ID, nil); err == nil {
		t.Fatal("expected error retrying non-failed log")
	}
}

func TestAutomationNotifyShippingIdempotent(t *testing.T) {
	db := openAutomationTestDB(t)
	svc := &order.Service{DB: db}
	createAutomationRule(t, db, svc, 1, order.AutomationRuleBody{
		Name: "签收通知发货", TriggerEvent: order.AutomationEventProcurementDelivered,
		Action: order.AutomationActionNotifyShipping, MaxAmount: f64Ptr(1000),
	})
	r2 := createAutomationRule(t, db, svc, 1, order.AutomationRuleBody{
		Name: "揽收通知发货", TriggerEvent: order.AutomationEventLogisticsCollected,
		Action: order.AutomationActionNotifyShipping, MaxAmount: f64Ptr(1000),
	})
	o := newAutomationOrder(t, db, 1, 60, "")
	svc.FireOrderEvent(context.Background(), 1, o.ID, order.AutomationEventProcurementDelivered)
	var after order.Order
	if err := db.First(&after, "id = ?", o.ID).Error; err != nil {
		t.Fatal(err)
	}
	if after.ShipReadyNotifiedAt == nil {
		t.Fatal("expected ship-ready notification stamp")
	}
	// A different rule/event later sees the stamp and records a skip.
	svc.FireOrderEvent(context.Background(), 1, o.ID, order.AutomationEventLogisticsCollected)
	rows := logsFor(t, db, o.ID)
	if len(rows) != 2 {
		t.Fatalf("expected 2 logs, got %d", len(rows))
	}
	var skipped *order.OrderAutomationLog
	for i := range rows {
		if rows[i].RuleID == r2.ID {
			skipped = &rows[i]
		}
	}
	if skipped == nil || skipped.Status != order.AutomationLogSkipped {
		t.Fatalf("expected skipped log for second notify rule, got %+v", rows)
	}
}

func TestAutomationDryRun(t *testing.T) {
	db := openAutomationTestDB(t)
	svc := &order.Service{DB: db}
	newAutomationOrder(t, db, 1, 60, "")
	newAutomationOrder(t, db, 1, 900, "")
	newAutomationOrder(t, db, 1, 50, order.ReviewStatusPending)
	newAutomationOrder(t, db, 2, 60, "")

	res, err := svc.DryRunAutomationRule(automationTestCtx(1), order.AutomationRuleBody{
		Name: "dry", TriggerEvent: order.AutomationEventOrderCreated,
		Action: order.AutomationActionConfirmPayment, MaxAmount: f64Ptr(100),
	})
	if err != nil {
		t.Fatal(err)
	}
	if res.Scanned != 3 || res.Matched != 2 || res.Blocked != 1 {
		t.Fatalf("unexpected dry-run result: %+v", res)
	}
	// Dry run never writes logs or mutates orders.
	var n int64
	if err := db.Model(&order.OrderAutomationLog{}).Count(&n).Error; err != nil {
		t.Fatal(err)
	}
	if n != 0 {
		t.Fatalf("dry run must not write logs, got %d", n)
	}
}

func TestAutomationLogListFilters(t *testing.T) {
	db := openAutomationTestDB(t)
	svc := &order.Service{DB: db}
	createAutomationRule(t, db, svc, 1, order.AutomationRuleBody{
		Name: "低额自动付款", TriggerEvent: order.AutomationEventOrderCreated,
		Action: order.AutomationActionConfirmPayment, MaxAmount: f64Ptr(100),
	})
	o := newAutomationOrder(t, db, 1, 60, "")
	blockedOrder := newAutomationOrder(t, db, 1, 60, order.ReviewStatusHeld)
	svc.FireOrderEvent(context.Background(), 1, o.ID, order.AutomationEventOrderCreated)
	svc.FireOrderEvent(context.Background(), 1, blockedOrder.ID, order.AutomationEventOrderCreated)

	res, err := svc.ListAutomationLogs(automationTestCtx(1), order.AutomationLogQuery{Page: 1, PageSize: 10})
	if err != nil {
		t.Fatal(err)
	}
	if res.Total != 2 {
		t.Fatalf("expected 2 logs, got %d", res.Total)
	}
	res, err = svc.ListAutomationLogs(automationTestCtx(1), order.AutomationLogQuery{
		Page: 1, PageSize: 10, Status: order.AutomationLogSkipped,
	})
	if err != nil {
		t.Fatal(err)
	}
	if res.Total != 1 || res.Items[0].OrderID != blockedOrder.ID {
		t.Fatalf("unexpected filtered result: %+v", res)
	}
	// Cross-tenant sees nothing.
	res, err = svc.ListAutomationLogs(automationTestCtx(2), order.AutomationLogQuery{Page: 1, PageSize: 10})
	if err != nil {
		t.Fatal(err)
	}
	if res.Total != 0 {
		t.Fatalf("expected tenant isolation on logs, got %d", res.Total)
	}
}
