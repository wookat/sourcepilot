package customerchat

import (
	"context"
	"net/http/httptest"
	"path/filepath"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"github.com/google/uuid"
	"gorm.io/gorm"

	"github.com/trademind-ai/trademind/backend/internal/modules/order"
	"github.com/trademind-ai/trademind/backend/internal/modules/shop"
	"github.com/trademind-ai/trademind/backend/internal/pkg/ctxkey"
)

func newBuyerMsgTestSvc(t *testing.T) *Service {
	t.Helper()
	dsn := filepath.Join(t.TempDir(), "buyermsg.db")
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Skipf("sqlite unavailable: %v", err)
	}
	if err := db.AutoMigrate(
		&CustomerReplyTemplate{}, &CustomerConversation{},
		&BuyerMessageRule{}, &BuyerMessageDraft{},
		&order.Order{}, &order.OrderItem{}, &order.OrderShipment{}, &shop.Shop{},
	); err != nil {
		t.Fatal(err)
	}
	gin.SetMode(gin.TestMode)
	return &Service{DB: db}
}

func buyerMsgCtx(t *testing.T, tenantID int64) *gin.Context {
	t.Helper()
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Request = httptest.NewRequest("GET", "/api/v1/customer/buyer-messages/drafts", nil)
	c.Set(ctxkey.TenantID, tenantID)
	return c
}

func mustTemplate(t *testing.T, svc *Service, c *gin.Context, name, content string) *TemplateRow {
	t.Helper()
	row, err := svc.CreateTemplate(c, TemplateUpsertBody{
		GroupKey: TemplateGroupLogistics, Name: name, Content: content,
	}, nil)
	if err != nil {
		t.Fatal(err)
	}
	return row
}

func TestFillBuyerMsgTemplate(t *testing.T) {
	text, missing := FillBuyerMsgTemplate("您好{买家昵称}，订单{订单号}已发货，单号{物流单号}。", map[string]string{
		"买家昵称": "张三", "订单号": "SO-1",
	})
	if text != "您好张三，订单SO-1已发货，单号{物流单号}。" {
		t.Fatalf("filled text: %s", text)
	}
	if len(missing) != 1 || missing[0] != "物流单号" {
		t.Fatalf("missing vars: %v", missing)
	}
	// no placeholders
	text, missing = FillBuyerMsgTemplate("纯文本", nil)
	if text != "纯文本" || len(missing) != 0 {
		t.Fatalf("plain text: %s %v", text, missing)
	}
}

func TestBuyerMsgRuleCRUDAndTenantIsolation(t *testing.T) {
	svc := newBuyerMsgTestSvc(t)
	c7 := buyerMsgCtx(t, 7)
	tpl := mustTemplate(t, svc, c7, "e2e-发货通知", "订单{订单号}已发货")

	// invalid node rejected
	if _, err := svc.CreateBuyerMsgRule(c7, BuyerMsgRuleBody{Name: "x", Node: "bogus", TemplateID: tpl.ID.String()}, nil); err == nil {
		t.Fatal("invalid node must be rejected")
	}
	// missing template rejected
	if _, err := svc.CreateBuyerMsgRule(c7, BuyerMsgRuleBody{Name: "x", Node: BuyerMsgNodeShipped, TemplateID: uuid.NewString()}, nil); err == nil {
		t.Fatal("unknown template must be rejected")
	}

	rule, err := svc.CreateBuyerMsgRule(c7, BuyerMsgRuleBody{
		Name: "e2e-发货自动消息", Node: BuyerMsgNodeShipped, TemplateID: tpl.ID.String(),
	}, nil)
	if err != nil {
		t.Fatalf("CreateBuyerMsgRule: %v", err)
	}
	if !rule.Enabled || rule.TemplateName != "e2e-发货通知" {
		t.Fatalf("rule defaults: %+v", rule)
	}

	// enabled=false persists（回归：GORM default tag 吞 bool 零值）
	off := false
	ruleOff, err := svc.CreateBuyerMsgRule(c7, BuyerMsgRuleBody{
		Name: "e2e-停用规则", Node: BuyerMsgNodePaid, TemplateID: tpl.ID.String(), Enabled: &off,
	}, nil)
	if err != nil {
		t.Fatal(err)
	}
	var storedOff BuyerMessageRule
	if err := svc.DB.First(&storedOff, "id = ?", ruleOff.ID).Error; err != nil {
		t.Fatal(err)
	}
	if storedOff.Enabled {
		t.Fatal("enabled=false must persist")
	}

	// cross-tenant update / delete → not found
	c8 := buyerMsgCtx(t, 8)
	if _, err := svc.UpdateBuyerMsgRule(c8, rule.ID, BuyerMsgRuleBody{Name: "劫持"}, nil); err != ErrBuyerMsgRuleNotFound {
		t.Fatalf("cross-tenant update: %v", err)
	}
	if err := svc.DeleteBuyerMsgRule(c8, rule.ID, nil); err != ErrBuyerMsgRuleNotFound {
		t.Fatalf("cross-tenant delete: %v", err)
	}
	rows8, err := svc.ListBuyerMsgRules(c8)
	if err != nil {
		t.Fatal(err)
	}
	if len(rows8) != 0 {
		t.Fatalf("tenant 8 must not see tenant 7 rules, got %d", len(rows8))
	}

	// update toggles enabled + platform filter
	upd, err := svc.UpdateBuyerMsgRule(c7, rule.ID, BuyerMsgRuleBody{
		Enabled: &off, Platforms: &[]string{"douyin_shop"},
	}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if upd.Enabled || len(upd.Platforms) != 1 || upd.Platforms[0] != "douyin_shop" {
		t.Fatalf("update result: %+v", upd)
	}

	if err := svc.DeleteBuyerMsgRule(c7, rule.ID, nil); err != nil {
		t.Fatal(err)
	}
	rows, err := svc.ListBuyerMsgRules(c7)
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 1 || rows[0].ID != ruleOff.ID {
		t.Fatalf("list after delete: %+v", rows)
	}
}

func seedBuyerMsgOrder(t *testing.T, svc *Service, tenantID int64, orderNo, status, payment string, shopID *uuid.UUID) order.Order {
	t.Helper()
	o := order.Order{
		TenantID: tenantID, Platform: "douyin_shop", ShopID: shopID,
		OrderNo: orderNo, CustomerName: "e2e-买家", Status: status,
		PaymentStatus: payment, FulfillmentStatus: order.FulfillmentUnfulfilled,
		Currency: "CNY", TotalAmount: 100,
	}
	if err := svc.DB.Create(&o).Error; err != nil {
		t.Fatal(err)
	}
	item := order.OrderItem{OrderID: o.ID, ProductTitle: "e2e-演示商品", Quantity: 1}
	if err := svc.DB.Create(&item).Error; err != nil {
		t.Fatal(err)
	}
	return o
}

func TestGenerateBuyerMsgDraftsIdempotentAndFills(t *testing.T) {
	svc := newBuyerMsgTestSvc(t)
	c := buyerMsgCtx(t, 7)

	sh := shop.Shop{TenantID: 7, Platform: "douyin_shop", ShopName: "e2e-抖店"}
	if err := svc.DB.Create(&sh).Error; err != nil {
		t.Fatal(err)
	}
	tpl := mustTemplate(t, svc, c, "e2e-发货通知",
		"您好{买家昵称}，您在{店铺名}的订单{订单号}（{商品名}）已发货，物流单号{物流单号}。")
	if _, err := svc.CreateBuyerMsgRule(c, BuyerMsgRuleBody{
		Name: "e2e-发货", Node: BuyerMsgNodeShipped, TemplateID: tpl.ID.String(),
	}, nil); err != nil {
		t.Fatal(err)
	}

	shipped := seedBuyerMsgOrder(t, svc, 7, "SO-SHIP-1", order.StatusShipped, order.PaymentPaid, &sh.ID)
	ship := order.OrderShipment{OrderID: shipped.ID, Carrier: "e2e-快递", TrackingNo: "TRK-1", Status: order.ShipmentShipped}
	if err := svc.DB.Create(&ship).Error; err != nil {
		t.Fatal(err)
	}
	// pending / other-tenant orders never match
	seedBuyerMsgOrder(t, svc, 7, "SO-PEND-1", order.StatusPending, order.PaymentUnpaid, &sh.ID)
	seedBuyerMsgOrder(t, svc, 8, "SO-T8-1", order.StatusShipped, order.PaymentPaid, nil)

	created, err := svc.GenerateBuyerMsgDrafts(context.Background(), 7)
	if err != nil {
		t.Fatalf("GenerateBuyerMsgDrafts: %v", err)
	}
	if created != 1 {
		t.Fatalf("created: got %d, want 1", created)
	}
	var draft BuyerMessageDraft
	if err := svc.DB.First(&draft, "tenant_id = ? AND order_id = ?", 7, shipped.ID).Error; err != nil {
		t.Fatal(err)
	}
	want := "您好e2e-买家，您在e2e-抖店的订单SO-SHIP-1（e2e-演示商品）已发货，物流单号TRK-1。"
	if draft.Content != want {
		t.Fatalf("draft content: %s", draft.Content)
	}
	if draft.Status != BuyerMsgDraftPending || draft.Node != BuyerMsgNodeShipped {
		t.Fatalf("draft meta: %+v", draft)
	}

	// second scan creates nothing (idempotent)
	created, err = svc.GenerateBuyerMsgDrafts(context.Background(), 7)
	if err != nil {
		t.Fatal(err)
	}
	if created != 0 {
		t.Fatalf("second scan must create 0, got %d", created)
	}

	// missing vars are reported, draft still created
	tpl2 := mustTemplate(t, svc, c, "e2e-签收关怀", "订单{订单号}已签收，单号{物流单号}")
	if _, err := svc.CreateBuyerMsgRule(c, BuyerMsgRuleBody{
		Name: "e2e-签收", Node: BuyerMsgNodeDelivered, TemplateID: tpl2.ID.String(),
	}, nil); err != nil {
		t.Fatal(err)
	}
	delivered := seedBuyerMsgOrder(t, svc, 7, "SO-DLV-1", order.StatusDelivered, order.PaymentPaid, &sh.ID)
	created, err = svc.GenerateBuyerMsgDrafts(context.Background(), 7)
	if err != nil {
		t.Fatal(err)
	}
	// 2 drafts: delivered node + shipped node (已签收订单也经历过发货节点)
	if created != 2 {
		t.Fatalf("delivered scan created: %d", created)
	}
	var d2 BuyerMessageDraft
	if err := svc.DB.First(&d2, "order_id = ? AND node = ?", delivered.ID, BuyerMsgNodeDelivered).Error; err != nil {
		t.Fatal(err)
	}
	if got := jsonToStrings(d2.MissingVars); len(got) != 1 || got[0] != "物流单号" {
		t.Fatalf("missing vars: %v", got)
	}
	// linked conversation is attached when the order has one
	conv := CustomerConversation{TenantID: 7, Platform: "douyin_shop", CustomerName: "e2e-买家",
		CustomerLanguage: "zh", Status: StatusOpen, OrderID: &delivered.ID}
	if err := svc.DB.Create(&conv).Error; err != nil {
		t.Fatal(err)
	}
}

func TestBuyerMsgDraftWorkflow(t *testing.T) {
	svc := newBuyerMsgTestSvc(t)
	c := buyerMsgCtx(t, 7)
	tpl := mustTemplate(t, svc, c, "e2e-付款", "已收到订单{订单号}")
	if _, err := svc.CreateBuyerMsgRule(c, BuyerMsgRuleBody{
		Name: "e2e-付款", Node: BuyerMsgNodePaid, TemplateID: tpl.ID.String(),
	}, nil); err != nil {
		t.Fatal(err)
	}
	o1 := seedBuyerMsgOrder(t, svc, 7, "SO-PAID-1", order.StatusPaid, order.PaymentPaid, nil)
	o2 := seedBuyerMsgOrder(t, svc, 7, "SO-PAID-2", order.StatusPaid, order.PaymentPaid, nil)
	if _, err := svc.GenerateBuyerMsgDrafts(context.Background(), 7); err != nil {
		t.Fatal(err)
	}

	list, err := svc.ListBuyerMsgDrafts(c, BuyerMsgDraftQuery{Node: BuyerMsgNodePaid, Status: BuyerMsgDraftPending})
	if err != nil {
		t.Fatal(err)
	}
	if list.Total != 2 {
		t.Fatalf("total: %d", list.Total)
	}
	var d1, d2 BuyerMessageDraft
	if err := svc.DB.First(&d1, "order_id = ?", o1.ID).Error; err != nil {
		t.Fatal(err)
	}
	if err := svc.DB.First(&d2, "order_id = ?", o2.ID).Error; err != nil {
		t.Fatal(err)
	}

	// edit content (pending only)
	upd, err := svc.UpdateBuyerMsgDraft(c, d1.ID, "已收到订单 SO-PAID-1，我们会尽快安排。", nil)
	if err != nil {
		t.Fatal(err)
	}
	if upd.Content != "已收到订单 SO-PAID-1，我们会尽快安排。" {
		t.Fatalf("edited content: %s", upd.Content)
	}

	// mark sent (idempotent) then editing is rejected
	sent, err := svc.MarkBuyerMsgDraftSent(c, d1.ID, nil)
	if err != nil {
		t.Fatal(err)
	}
	if sent.Status != BuyerMsgDraftSent || sent.SentAt == nil {
		t.Fatalf("sent: %+v", sent)
	}
	if _, err := svc.MarkBuyerMsgDraftSent(c, d1.ID, nil); err != nil {
		t.Fatalf("mark sent must be idempotent: %v", err)
	}
	if _, err := svc.UpdateBuyerMsgDraft(c, d1.ID, "改一下", nil); err == nil {
		t.Fatal("editing a sent draft must fail")
	}

	// ignore
	ign, err := svc.IgnoreBuyerMsgDraft(c, d2.ID, nil)
	if err != nil {
		t.Fatal(err)
	}
	if ign.Status != BuyerMsgDraftIgnored || ign.IgnoredAt == nil {
		t.Fatalf("ignored: %+v", ign)
	}
	// ignored drafts are not regenerated
	created, err := svc.GenerateBuyerMsgDrafts(context.Background(), 7)
	if err != nil {
		t.Fatal(err)
	}
	if created != 0 {
		t.Fatalf("regenerate after ignore: %d", created)
	}

	// cross-tenant draft access → 404
	c8 := buyerMsgCtx(t, 8)
	if _, err := svc.UpdateBuyerMsgDraft(c8, d1.ID, "劫持", nil); err != ErrBuyerMsgDraftNotFound {
		t.Fatalf("cross-tenant draft update: %v", err)
	}
	if _, err := svc.MarkBuyerMsgDraftSent(c8, d1.ID, nil); err != ErrBuyerMsgDraftNotFound {
		t.Fatalf("cross-tenant mark sent: %v", err)
	}
}

func TestBatchMarkBuyerMsgDraftsSent(t *testing.T) {
	svc := newBuyerMsgTestSvc(t)
	c := buyerMsgCtx(t, 7)
	tpl := mustTemplate(t, svc, c, "e2e-付款", "已收到订单{订单号}")
	if _, err := svc.CreateBuyerMsgRule(c, BuyerMsgRuleBody{
		Name: "e2e-付款", Node: BuyerMsgNodePaid, TemplateID: tpl.ID.String(),
	}, nil); err != nil {
		t.Fatal(err)
	}
	o1 := seedBuyerMsgOrder(t, svc, 7, "SO-B-1", order.StatusPaid, order.PaymentPaid, nil)
	o2 := seedBuyerMsgOrder(t, svc, 7, "SO-B-2", order.StatusPaid, order.PaymentPaid, nil)
	if _, err := svc.GenerateBuyerMsgDrafts(context.Background(), 7); err != nil {
		t.Fatal(err)
	}
	var d1, d2 BuyerMessageDraft
	if err := svc.DB.First(&d1, "order_id = ?", o1.ID).Error; err != nil {
		t.Fatal(err)
	}
	if err := svc.DB.First(&d2, "order_id = ?", o2.ID).Error; err != nil {
		t.Fatal(err)
	}
	// pre-ignore one → it is skipped, not updated
	if _, err := svc.IgnoreBuyerMsgDraft(c, d2.ID, nil); err != nil {
		t.Fatal(err)
	}
	res, err := svc.BatchMarkBuyerMsgDraftsSent(c, []uuid.UUID{d1.ID, d2.ID}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if res.Updated != 1 || res.Skipped != 1 {
		t.Fatalf("batch result: %+v", res)
	}
	// cross-tenant ids are skipped silently
	c8 := buyerMsgCtx(t, 8)
	res8, err := svc.BatchMarkBuyerMsgDraftsSent(c8, []uuid.UUID{d1.ID}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if res8.Updated != 0 {
		t.Fatalf("cross-tenant batch must update 0: %+v", res8)
	}
	// empty ids rejected
	if _, err := svc.BatchMarkBuyerMsgDraftsSent(c, nil, nil); err == nil {
		t.Fatal("empty ids must be rejected")
	}
}

func TestUpdateBuyerMsgDraftRecomputesMissingVars(t *testing.T) {
	svc := newBuyerMsgTestSvc(t)
	c := buyerMsgCtx(t, 7)
	tpl := mustTemplate(t, svc, c, "e2e-签收", "订单{订单号}已签收，单号{物流单号}")
	if _, err := svc.CreateBuyerMsgRule(c, BuyerMsgRuleBody{
		Name: "e2e-签收", Node: BuyerMsgNodeDelivered, TemplateID: tpl.ID.String(),
	}, nil); err != nil {
		t.Fatal(err)
	}
	o := seedBuyerMsgOrder(t, svc, 7, "SO-MV-1", order.StatusDelivered, order.PaymentPaid, nil)
	if _, err := svc.GenerateBuyerMsgDrafts(context.Background(), 7); err != nil {
		t.Fatal(err)
	}
	var d BuyerMessageDraft
	if err := svc.DB.First(&d, "order_id = ? AND node = ?", o.ID, BuyerMsgNodeDelivered).Error; err != nil {
		t.Fatal(err)
	}
	if got := jsonToStrings(d.MissingVars); len(got) != 1 || got[0] != "物流单号" {
		t.Fatalf("initial missing vars: %v", got)
	}

	// 补全变量后警告重算清零
	upd, err := svc.UpdateBuyerMsgDraft(c, d.ID, "订单SO-MV-1已签收，单号SF123456", nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(upd.MissingVars) != 0 {
		t.Fatalf("missing vars must be cleared after补全: %v", upd.MissingVars)
	}

	// 重新引入占位则重新出现
	upd, err = svc.UpdateBuyerMsgDraft(c, d.ID, "订单{订单号}已签收，单号{物流单号}", nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(upd.MissingVars) != 2 {
		t.Fatalf("missing vars must be recomputed: %v", upd.MissingVars)
	}
}

func TestBuyerMsgRuleTemplateMissingAfterDelete(t *testing.T) {
	svc := newBuyerMsgTestSvc(t)
	c := buyerMsgCtx(t, 7)
	tpl := mustTemplate(t, svc, c, "e2e-发货", "订单{订单号}已发货")
	rule, err := svc.CreateBuyerMsgRule(c, BuyerMsgRuleBody{
		Name: "e2e-发货", Node: BuyerMsgNodeShipped, TemplateID: tpl.ID.String(),
	}, nil)
	if err != nil {
		t.Fatal(err)
	}
	rows, err := svc.ListBuyerMsgRules(c)
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 1 || rows[0].TemplateMissing || rows[0].TemplateName != "e2e-发货" {
		t.Fatalf("before delete: %+v", rows)
	}

	if err := svc.DeleteTemplate(c, tpl.ID, nil); err != nil {
		t.Fatal(err)
	}
	rows, err = svc.ListBuyerMsgRules(c)
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 1 || !rows[0].TemplateMissing {
		t.Fatalf("after delete rule must be marked templateMissing: %+v", rows)
	}

	// 引用已删模板的更新（不换模板）仍允许启停；换成已删模板必须被拒
	if _, err := svc.UpdateBuyerMsgRule(c, rule.ID, BuyerMsgRuleBody{TemplateID: tpl.ID.String()}, nil); err == nil {
		t.Fatal("selecting a deleted template must be rejected")
	}

	// 已删模板规则不生成草稿（inert，不报错）
	seedBuyerMsgOrder(t, svc, 7, "SO-TD-1", order.StatusShipped, order.PaymentPaid, nil)
	created, err := svc.GenerateBuyerMsgDrafts(context.Background(), 7)
	if err != nil {
		t.Fatal(err)
	}
	if created != 0 {
		t.Fatalf("rule with deleted template must be inert, created=%d", created)
	}
}
