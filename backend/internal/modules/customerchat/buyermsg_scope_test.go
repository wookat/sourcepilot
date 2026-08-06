package customerchat

import (
	"context"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/trademind-ai/trademind/backend/internal/modules/order"
	"github.com/trademind-ai/trademind/backend/internal/modules/shop"
)

func backdateOrder(t *testing.T, svc *Service, o order.Order, d time.Duration) {
	t.Helper()
	past := time.Now().Add(-d)
	if err := svc.DB.Model(&order.Order{}).Where("id = ?", o.ID).
		Update("created_at", past).Error; err != nil {
		t.Fatal(err)
	}
}

// 规则默认只对生效后的新订单事件生成草稿，不回溯存量订单。
func TestBuyerMsgRuleDefaultNoBackfill(t *testing.T) {
	svc := newBuyerMsgTestSvc(t)
	c := buyerMsgCtx(t, 7)
	tpl := mustTemplate(t, svc, c, "e2e-发货通知", "订单{订单号}已发货")

	// 存量订单：创建时间早于规则生效时间
	old := seedBuyerMsgOrder(t, svc, 7, "SO-OLD-1", order.StatusShipped, order.PaymentPaid, nil)
	backdateOrder(t, svc, old, 24*time.Hour)

	rule, err := svc.CreateBuyerMsgRule(c, BuyerMsgRuleBody{
		Name: "e2e-发货", Node: BuyerMsgNodeShipped, TemplateID: tpl.ID.String(),
	}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if rule.Backfill || rule.EffectiveFrom == nil {
		t.Fatalf("default rule must not backfill: %+v", rule)
	}

	created, err := svc.GenerateBuyerMsgDrafts(context.Background(), 7)
	if err != nil {
		t.Fatal(err)
	}
	if created != 0 {
		t.Fatalf("存量订单不得生成草稿, got %d", created)
	}

	// 规则生效后的新订单事件正常生成
	seedBuyerMsgOrder(t, svc, 7, "SO-NEW-1", order.StatusShipped, order.PaymentPaid, nil)
	created, err = svc.GenerateBuyerMsgDrafts(context.Background(), 7)
	if err != nil {
		t.Fatal(err)
	}
	if created != 1 {
		t.Fatalf("新订单应生成 1 条草稿, got %d", created)
	}
	var cnt int64
	if err := svc.DB.Model(&BuyerMessageDraft{}).
		Where("tenant_id = ? AND order_id = ?", 7, old.ID).Count(&cnt).Error; err != nil {
		t.Fatal(err)
	}
	if cnt != 0 {
		t.Fatal("存量订单不得有草稿")
	}
}

// 事件时间口径：节点时间戳晚于规则生效时间的存量订单（新事件）仍生成草稿。
func TestBuyerMsgRuleEffectiveUsesEventTime(t *testing.T) {
	svc := newBuyerMsgTestSvc(t)
	c := buyerMsgCtx(t, 7)
	tpl := mustTemplate(t, svc, c, "e2e-发货通知", "订单{订单号}已发货")

	// 老订单，规则创建时尚未发货
	old := seedBuyerMsgOrder(t, svc, 7, "SO-OLD-2", order.StatusPending, order.PaymentUnpaid, nil)
	backdateOrder(t, svc, old, 24*time.Hour)

	if _, err := svc.CreateBuyerMsgRule(c, BuyerMsgRuleBody{
		Name: "e2e-发货", Node: BuyerMsgNodeShipped, TemplateID: tpl.ID.String(),
	}, nil); err != nil {
		t.Fatal(err)
	}

	// 规则生效后老订单才发货（新事件），应生成草稿
	shippedAt := time.Now().Add(time.Minute)
	if err := svc.DB.Model(&order.Order{}).Where("id = ?", old.ID).
		Updates(map[string]any{"status": order.StatusShipped, "shipped_at": shippedAt}).Error; err != nil {
		t.Fatal(err)
	}
	created, err := svc.GenerateBuyerMsgDrafts(context.Background(), 7)
	if err != nil {
		t.Fatal(err)
	}
	if created != 1 {
		t.Fatalf("生效后发生的发货事件应生成草稿, got %d", created)
	}
}

// 回溯存量开关：预估与生成口径一致；开启后对存量订单生成草稿。
func TestBuyerMsgRuleBackfillEstimateAndGenerate(t *testing.T) {
	svc := newBuyerMsgTestSvc(t)
	c := buyerMsgCtx(t, 7)
	tpl := mustTemplate(t, svc, c, "e2e-发货通知", "订单{订单号}已发货")

	for _, no := range []string{"SO-OLD-A", "SO-OLD-B"} {
		o := seedBuyerMsgOrder(t, svc, 7, no, order.StatusShipped, order.PaymentPaid, nil)
		backdateOrder(t, svc, o, 24*time.Hour)
	}
	// 其他租户存量订单不计入预估
	o8 := seedBuyerMsgOrder(t, svc, 8, "SO-T8-OLD", order.StatusShipped, order.PaymentPaid, nil)
	backdateOrder(t, svc, o8, 24*time.Hour)

	est, err := svc.EstimateBuyerMsgBackfill(c, BuyerMsgNodeShipped, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	if est != 2 {
		t.Fatalf("estimate: got %d, want 2", est)
	}
	// 非法节点被拒绝
	if _, err := svc.EstimateBuyerMsgBackfill(c, "bogus", nil, nil); err == nil {
		t.Fatal("invalid node must be rejected")
	}

	// 创建规则时显式开启回溯
	on := true
	rule, err := svc.CreateBuyerMsgRule(c, BuyerMsgRuleBody{
		Name: "e2e-发货回溯", Node: BuyerMsgNodeShipped, TemplateID: tpl.ID.String(), Backfill: &on,
	}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if !rule.Backfill || rule.EffectiveFrom != nil {
		t.Fatalf("backfill rule: %+v", rule)
	}
	created, err := svc.GenerateBuyerMsgDrafts(context.Background(), 7)
	if err != nil {
		t.Fatal(err)
	}
	if created != int(est) {
		t.Fatalf("回溯生成数应与预估一致: got %d, want %d", created, est)
	}
	// 幂等：再次扫描不重复生成
	created, err = svc.GenerateBuyerMsgDrafts(context.Background(), 7)
	if err != nil {
		t.Fatal(err)
	}
	if created != 0 {
		t.Fatalf("second scan must create 0, got %d", created)
	}
	// 已有草稿后预估归零
	est, err = svc.EstimateBuyerMsgBackfill(c, BuyerMsgNodeShipped, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	if est != 0 {
		t.Fatalf("estimate after generate: got %d, want 0", est)
	}
}

// 停用→重新启用会重置生效时间；编辑规则开启回溯会清除生效时间。
func TestBuyerMsgRuleReEnableResetsEffectiveFrom(t *testing.T) {
	svc := newBuyerMsgTestSvc(t)
	c := buyerMsgCtx(t, 7)
	tpl := mustTemplate(t, svc, c, "e2e-发货通知", "订单{订单号}已发货")

	rule, err := svc.CreateBuyerMsgRule(c, BuyerMsgRuleBody{
		Name: "e2e-发货", Node: BuyerMsgNodeShipped, TemplateID: tpl.ID.String(),
	}, nil)
	if err != nil {
		t.Fatal(err)
	}
	first := rule.EffectiveFrom
	if first == nil {
		t.Fatal("effectiveFrom must be set on create")
	}

	off, on := false, true
	if _, err := svc.UpdateBuyerMsgRule(c, rule.ID, BuyerMsgRuleBody{Enabled: &off}, nil); err != nil {
		t.Fatal(err)
	}
	// 停用期间的存量订单
	stale := seedBuyerMsgOrder(t, svc, 7, "SO-STALE-1", order.StatusShipped, order.PaymentPaid, nil)
	time.Sleep(10 * time.Millisecond)

	upd, err := svc.UpdateBuyerMsgRule(c, rule.ID, BuyerMsgRuleBody{Enabled: &on}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if upd.EffectiveFrom == nil || !upd.EffectiveFrom.After(*first) {
		t.Fatalf("re-enable must reset effectiveFrom: %+v vs %+v", upd.EffectiveFrom, first)
	}
	created, err := svc.GenerateBuyerMsgDrafts(context.Background(), 7)
	if err != nil {
		t.Fatal(err)
	}
	if created != 0 {
		t.Fatalf("停用期间的订单不得回溯, got %d", created)
	}
	var cnt int64
	if err := svc.DB.Model(&BuyerMessageDraft{}).
		Where("order_id = ?", stale.ID).Count(&cnt).Error; err != nil {
		t.Fatal(err)
	}
	if cnt != 0 {
		t.Fatal("停用期间订单不得有草稿")
	}

	// 编辑规则显式开启回溯 → 生效时间清空
	upd, err = svc.UpdateBuyerMsgRule(c, rule.ID, BuyerMsgRuleBody{Backfill: &on}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if !upd.Backfill || upd.EffectiveFrom != nil {
		t.Fatalf("backfill update: %+v", upd)
	}
	created, err = svc.GenerateBuyerMsgDrafts(context.Background(), 7)
	if err != nil {
		t.Fatal(err)
	}
	if created != 1 {
		t.Fatalf("开启回溯后应补齐存量草稿, got %d", created)
	}

	// 回溯存量 → 关闭回溯：恢复生效时间，只对之后的新事件生成
	upd, err = svc.UpdateBuyerMsgRule(c, rule.ID, BuyerMsgRuleBody{Backfill: &off}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if upd.Backfill || upd.EffectiveFrom == nil {
		t.Fatalf("backfill off update: %+v", upd)
	}
}

// HTTP：回溯预估端点鉴权与返回结构。
func TestBuyerMsgBackfillEstimateRoute(t *testing.T) {
	f := setupBuyerMsgHTTPFixture(t)
	if err := f.db.AutoMigrate(&order.Order{}, &order.OrderItem{}, &order.OrderShipment{}, &shop.Shop{}); err != nil {
		t.Fatal(err)
	}
	o := order.Order{
		TenantID: 1, Platform: "douyin_shop", OrderNo: "SO-EST-1", CustomerName: "买家",
		Status: order.StatusShipped, PaymentStatus: order.PaymentPaid,
		FulfillmentStatus: order.FulfillmentUnfulfilled, Currency: "CNY",
	}
	if err := f.db.Create(&o).Error; err != nil {
		t.Fatal(err)
	}

	w := f.do(t, http.MethodGet, "/api/v1/customer/buyer-message-rules/backfill-estimate?node=shipped", f.adminT1, "1", "")
	if w.Code != http.StatusOK {
		t.Fatalf("estimate: got %d (%s)", w.Code, w.Body.String())
	}
	if got := w.Body.String(); !strings.Contains(got, `"estimated":1`) {
		t.Fatalf("estimate body: %s", got)
	}
	// 跨租户不计入
	w = f.do(t, http.MethodGet, "/api/v1/customer/buyer-message-rules/backfill-estimate?node=shipped", f.adminT2, "2", "")
	if w.Code != http.StatusOK {
		t.Fatalf("estimate t2: got %d", w.Code)
	}
	if got := w.Body.String(); !strings.Contains(got, `"estimated":0`) {
		t.Fatalf("estimate t2 body: %s", got)
	}
	// 非法节点 400
	w = f.do(t, http.MethodGet, "/api/v1/customer/buyer-message-rules/backfill-estimate?node=bogus", f.adminT1, "1", "")
	if w.Code != http.StatusBadRequest {
		t.Fatalf("estimate bogus node: got %d", w.Code)
	}
}
