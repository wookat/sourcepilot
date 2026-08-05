package order_test

import (
	"encoding/json"
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

func openReviewTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	dsn := fmt.Sprintf("file:order_review_%s?mode=memory&cache=shared&_pragma=busy_timeout(5000)", uuid.New().String())
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Skipf("sqlite unavailable: %v", err)
	}
	sqlDB, _ := db.DB()
	sqlDB.SetMaxOpenConns(1)
	if err := db.AutoMigrate(&order.Order{}, &order.OrderItem{}, &order.OrderShipment{},
		&order.OrderReviewRule{}, &order.OrderReviewHit{}, &shop.Shop{}); err != nil {
		t.Fatal(err)
	}
	return db
}

func reviewTestCtx(tenantID int64) *gin.Context {
	gin.SetMode(gin.TestMode)
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Request = httptest.NewRequest("POST", "/api/v1/order-review", nil)
	c.Set(ctxkey.TenantID, tenantID)
	return c
}

func strs(v ...string) *[]string { return &v }
func iptr(v int) *int            { return &v }
func fptr(v float64) *float64    { return &v }

func reviewOrderBody(orderNo string, amount float64, remark string) order.CreateBody {
	return order.CreateBody{
		OrderNo:      orderNo,
		CustomerName: "买家甲",
		Currency:     "USD",
		TotalAmount:  amount,
		Remark:       remark,
		Items: []order.OrderItemInput{
			{ProductTitle: "测试商品", SKUCode: "SKU-1", Quantity: 2, UnitPrice: amount / 2, TotalPrice: amount},
		},
	}
}

func TestReviewRuleCRUDAndTenantIsolation(t *testing.T) {
	db := openReviewTestDB(t)
	svc := &order.Service{DB: db}
	c1 := reviewTestCtx(1)
	c2 := reviewTestCtx(2)

	row, err := svc.CreateReviewRule(c1, order.ReviewRuleBody{
		Name: "高额待审", Action: order.ReviewActionReview, MinAmount: fptr(100),
	}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if !row.Enabled || row.TenantID != 1 {
		t.Fatalf("unexpected rule: %+v", row)
	}

	// Cross-tenant update/delete → not found (handler maps to 404).
	if _, err := svc.UpdateReviewRule(c2, row.ID, order.ReviewRuleBody{Name: "x"}, nil); err != order.ErrReviewRuleNotFound {
		t.Fatalf("expected not found, got %v", err)
	}
	if err := svc.DeleteReviewRule(c2, row.ID, nil); err != order.ErrReviewRuleNotFound {
		t.Fatalf("expected not found, got %v", err)
	}
	rules2, err := svc.ListReviewRules(c2)
	if err != nil || len(rules2) != 0 {
		t.Fatalf("tenant 2 should see no rules: %v %d", err, len(rules2))
	}

	// No-condition rule rejected (accidental catch-all guard).
	if _, err := svc.CreateReviewRule(c1, order.ReviewRuleBody{Name: "空", Action: order.ReviewActionHold}, nil); err == nil {
		t.Fatal("expected error for rule without conditions")
	}
	// Invalid action rejected.
	if _, err := svc.CreateReviewRule(c1, order.ReviewRuleBody{Name: "x", Action: "boom", MinAmount: fptr(1)}, nil); err == nil {
		t.Fatal("expected error for invalid action")
	}
	// min > max rejected.
	if _, err := svc.CreateReviewRule(c1, order.ReviewRuleBody{
		Name: "区间错", Action: order.ReviewActionReview, MinAmount: fptr(10), MaxAmount: fptr(5),
	}, nil); err == nil {
		t.Fatal("expected error for min>max")
	}

	up, err := svc.UpdateReviewRule(c1, row.ID, order.ReviewRuleBody{Priority: iptr(5), Enabled: bptr(false)}, nil)
	if err != nil || up.Priority != 5 || up.Enabled {
		t.Fatalf("update failed: %+v %v", up, err)
	}
	if err := svc.DeleteReviewRule(c1, row.ID, nil); err != nil {
		t.Fatal(err)
	}
}

func bptr(v bool) *bool { return &v }

func TestReviewEngineOnCreateAmountAndPriority(t *testing.T) {
	db := openReviewTestDB(t)
	svc := &order.Service{DB: db}
	c := reviewTestCtx(1)

	// Priority 1: hold on amount >= 500; Priority 2: review on amount >= 100.
	if _, err := svc.CreateReviewRule(c, order.ReviewRuleBody{
		Name: "超高额挂起", Action: order.ReviewActionHold, Priority: iptr(1), MinAmount: fptr(500),
	}, nil); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.CreateReviewRule(c, order.ReviewRuleBody{
		Name: "高额待审", Action: order.ReviewActionReview, Priority: iptr(2), MinAmount: fptr(100),
	}, nil); err != nil {
		t.Fatal(err)
	}

	low, err := svc.Create(c, reviewOrderBody("SO-R1", 50, ""), nil)
	if err != nil {
		t.Fatal(err)
	}
	if low.ReviewStatus != order.ReviewStatusNone {
		t.Fatalf("low amount should not trigger, got %q", low.ReviewStatus)
	}

	mid, err := svc.Create(c, reviewOrderBody("SO-R2", 100, ""), nil) // boundary: min inclusive
	if err != nil {
		t.Fatal(err)
	}
	if mid.ReviewStatus != order.ReviewStatusPending {
		t.Fatalf("mid amount should be pending_review, got %q", mid.ReviewStatus)
	}

	high, err := svc.Create(c, reviewOrderBody("SO-R3", 600, ""), nil)
	if err != nil {
		t.Fatal(err)
	}
	if high.ReviewStatus != order.ReviewStatusHeld {
		t.Fatalf("high amount should be held (priority first), got %q", high.ReviewStatus)
	}
	// Both rules hit on the high order; decisive is the hold rule.
	var hits []order.OrderReviewHit
	if err := db.Where("order_id = ?", high.ID).Order("decisive DESC").Find(&hits).Error; err != nil {
		t.Fatal(err)
	}
	if len(hits) != 2 || !hits[0].Decisive || hits[0].Action != order.ReviewActionHold {
		t.Fatalf("unexpected hits: %+v", hits)
	}
}

func TestReviewEngineConditions(t *testing.T) {
	db := openReviewTestDB(t)
	svc := &order.Service{DB: db}
	c := reviewTestCtx(1)

	if _, err := svc.CreateReviewRule(c, order.ReviewRuleBody{
		Name: "备注关键词", Action: order.ReviewActionReview, Priority: iptr(1), RemarkKeywords: strs("加急", "改地址"),
	}, nil); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.CreateReviewRule(c, order.ReviewRuleBody{
		Name: "黑名单地区", Action: order.ReviewActionHold, Priority: iptr(2), AddressKeywords: strs("某某区"),
	}, nil); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.CreateReviewRule(c, order.ReviewRuleBody{
		Name: "单SKU超量", Action: order.ReviewActionReview, Priority: iptr(3), MaxSKUQuantity: iptr(5),
	}, nil); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.CreateReviewRule(c, order.ReviewRuleBody{
		Name: "指定平台自动通过", Action: order.ReviewActionPass, Priority: iptr(0), Platforms: strs("manual_vip"),
	}, nil); err != nil {
		t.Fatal(err)
	}

	// Remark keyword hit.
	o1, err := svc.Create(c, reviewOrderBody("SO-C1", 10, "买家说要改地址"), nil)
	if err != nil {
		t.Fatal(err)
	}
	if o1.ReviewStatus != order.ReviewStatusPending {
		t.Fatalf("remark keyword should pend, got %q", o1.ReviewStatus)
	}

	// Address keyword from rawData → hold.
	b2 := reviewOrderBody("SO-C2", 10, "")
	b2.RawData = json.RawMessage(`{"receiver":{"address":"某某省某某市某某区幸福路1号"}}`)
	o2, err := svc.Create(c, b2, nil)
	if err != nil {
		t.Fatal(err)
	}
	if o2.ReviewStatus != order.ReviewStatusHeld {
		t.Fatalf("address keyword should hold, got %q", o2.ReviewStatus)
	}

	// Per-SKU quantity boundary: qty 5 not > 5 → no hit; qty 6 hits.
	b3 := reviewOrderBody("SO-C3", 10, "")
	b3.Items[0].Quantity = 5
	o3, _ := svc.Create(c, b3, nil)
	if o3.ReviewStatus != order.ReviewStatusNone {
		t.Fatalf("qty 5 should not trigger, got %q", o3.ReviewStatus)
	}
	b4 := reviewOrderBody("SO-C4", 10, "")
	b4.Items[0].Quantity = 6
	o4, _ := svc.Create(c, b4, nil)
	if o4.ReviewStatus != order.ReviewStatusPending {
		t.Fatalf("qty 6 should pend, got %q", o4.ReviewStatus)
	}

	// Platform pass rule (priority 0) wins over remark rule.
	b5 := reviewOrderBody("SO-C5", 10, "加急")
	b5.Platform = "manual_vip"
	o5, err := svc.Create(c, b5, nil)
	if err != nil {
		t.Fatal(err)
	}
	if o5.ReviewStatus != order.ReviewStatusAutoPassed {
		t.Fatalf("platform pass rule should auto pass, got %q", o5.ReviewStatus)
	}
}

func TestReviewEngineRepeatReceiver(t *testing.T) {
	db := openReviewTestDB(t)
	svc := &order.Service{DB: db}
	c := reviewTestCtx(1)

	if _, err := svc.CreateReviewRule(c, order.ReviewRuleBody{
		Name: "重复收件人", Action: order.ReviewActionReview,
		RepeatReceiverMinOrders: iptr(2), RepeatReceiverWindowDays: iptr(7),
	}, nil); err != nil {
		t.Fatal(err)
	}

	o1, err := svc.Create(c, reviewOrderBody("SO-D1", 10, ""), nil)
	if err != nil {
		t.Fatal(err)
	}
	if o1.ReviewStatus != order.ReviewStatusNone {
		t.Fatalf("first order should not trigger, got %q", o1.ReviewStatus)
	}
	o2, err := svc.Create(c, reviewOrderBody("SO-D2", 10, ""), nil)
	if err != nil {
		t.Fatal(err)
	}
	if o2.ReviewStatus != order.ReviewStatusPending {
		t.Fatalf("second order same receiver should pend, got %q", o2.ReviewStatus)
	}
}

func TestReviewImportTriggersAndWorkbenchFlow(t *testing.T) {
	db := openReviewTestDB(t)
	svc := &order.Service{DB: db}
	c := reviewTestCtx(1)

	if _, err := svc.CreateReviewRule(c, order.ReviewRuleBody{
		Name: "高额待审", Action: order.ReviewActionReview, MinAmount: fptr(100),
	}, nil); err != nil {
		t.Fatal(err)
	}

	sum, err := svc.ImportOrders(c, order.ImportBody{Orders: []order.CreateBody{
		reviewOrderBody("SO-I1", 200, ""),
		reviewOrderBody("SO-I2", 20, ""),
	}}, nil)
	if err != nil || sum.Created != 2 {
		t.Fatalf("import failed: %+v %v", sum, err)
	}

	res, err := svc.ListReviewWorkbench(c, order.ReviewWorkbenchQuery{})
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Items) != 1 || res.Items[0].OrderNo != "SO-I1" || res.PendingTotal != 1 {
		t.Fatalf("unexpected workbench: %+v", res)
	}
	if len(res.Items[0].Hits) != 1 || res.Items[0].Hits[0].RuleName != "高额待审" {
		t.Fatalf("hits missing: %+v", res.Items[0].Hits)
	}

	// Tenant isolation on workbench.
	c2 := reviewTestCtx(2)
	res2, err := svc.ListReviewWorkbench(c2, order.ReviewWorkbenchQuery{})
	if err != nil || len(res2.Items) != 0 {
		t.Fatalf("tenant 2 should see nothing: %v %d", err, len(res2.Items))
	}

	oid := res.Items[0].ID

	// Cross-tenant approve fails.
	dec2, err := svc.ApproveReviewOrders(c2, order.ReviewDecisionBody{OrderIDs: []string{oid.String()}}, nil)
	if err != nil || dec2.Done != 0 || dec2.Failed != 1 {
		t.Fatalf("cross-tenant approve should fail per-row: %+v %v", dec2, err)
	}

	// Approve → back to normal flow.
	dec, err := svc.ApproveReviewOrders(c, order.ReviewDecisionBody{OrderIDs: []string{oid.String()}}, nil)
	if err != nil || dec.Done != 1 {
		t.Fatalf("approve failed: %+v %v", dec, err)
	}
	var after order.Order
	db.First(&after, "id = ?", oid)
	if after.ReviewStatus != order.ReviewStatusApproved {
		t.Fatalf("expected approved, got %q", after.ReviewStatus)
	}
	// Second approve is rejected (not in pending state anymore).
	dec3, _ := svc.ApproveReviewOrders(c, order.ReviewDecisionBody{OrderIDs: []string{oid.String()}}, nil)
	if dec3.Done != 0 || dec3.Failed != 1 {
		t.Fatalf("re-approve should fail: %+v", dec3)
	}
}

func TestReviewRejectEntersCancelFlow(t *testing.T) {
	db := openReviewTestDB(t)
	svc := &order.Service{DB: db}
	c := reviewTestCtx(1)

	if _, err := svc.CreateReviewRule(c, order.ReviewRuleBody{
		Name: "高额待审", Action: order.ReviewActionReview, MinAmount: fptr(100),
	}, nil); err != nil {
		t.Fatal(err)
	}
	o, err := svc.Create(c, reviewOrderBody("SO-J1", 300, ""), nil)
	if err != nil {
		t.Fatal(err)
	}
	dec, err := svc.RejectReviewOrders(c, order.ReviewDecisionBody{OrderIDs: []string{o.ID.String()}}, nil)
	if err != nil || dec.Done != 1 {
		t.Fatalf("reject failed: %+v %v", dec, err)
	}
	var after order.Order
	db.First(&after, "id = ?", o.ID)
	if after.ReviewStatus != order.ReviewStatusRejected || after.Status != order.StatusCancelled {
		t.Fatalf("expected rejected+cancelled, got %q %q", after.ReviewStatus, after.Status)
	}
}

func TestReviewBlocksShipment(t *testing.T) {
	db := openReviewTestDB(t)
	svc := &order.Service{DB: db}
	c := reviewTestCtx(1)

	if _, err := svc.CreateReviewRule(c, order.ReviewRuleBody{
		Name: "超高额挂起", Action: order.ReviewActionHold, MinAmount: fptr(500),
	}, nil); err != nil {
		t.Fatal(err)
	}
	o, err := svc.Create(c, reviewOrderBody("SO-S1", 900, ""), nil)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := svc.AppendShipment(c, o.ID, order.OrderShipmentInput{Carrier: "顺丰", TrackingNo: "SF1"}, nil); err == nil {
		t.Fatal("shipment should be blocked for held order")
	}

	// After approve, shipment goes through.
	if _, err := svc.ApproveReviewOrders(c, order.ReviewDecisionBody{OrderIDs: []string{o.ID.String()}}, nil); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.AppendShipment(c, o.ID, order.OrderShipmentInput{Carrier: "顺丰", TrackingNo: "SF1"}, nil); err != nil {
		t.Fatalf("shipment should pass after approve: %v", err)
	}
}

func TestReviewDryRun(t *testing.T) {
	db := openReviewTestDB(t)
	svc := &order.Service{DB: db}
	c := reviewTestCtx(1)

	for i, amt := range []float64{50, 150, 250} {
		if _, err := svc.Create(c, reviewOrderBody(fmt.Sprintf("SO-DR%d", i), amt, ""), nil); err != nil {
			t.Fatal(err)
		}
	}
	// Other tenant order must not be scanned.
	c2 := reviewTestCtx(2)
	if _, err := svc.Create(c2, reviewOrderBody("SO-DRX", 999, ""), nil); err != nil {
		t.Fatal(err)
	}

	res, err := svc.DryRunReviewRule(c, order.ReviewRuleBody{MinAmount: fptr(100)})
	if err != nil {
		t.Fatal(err)
	}
	if res.Scanned != 3 || res.Matched != 2 || len(res.Samples) != 2 {
		t.Fatalf("unexpected dry-run: %+v", res)
	}
	var cnt int64
	db.Model(&order.OrderReviewHit{}).Count(&cnt)
	if cnt != 0 {
		t.Fatalf("dry-run must not write hits, got %d", cnt)
	}
}
