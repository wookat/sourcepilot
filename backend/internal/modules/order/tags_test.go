package order_test

import (
	"fmt"
	"testing"

	"github.com/glebarez/sqlite"
	"github.com/google/uuid"
	"github.com/trademind-ai/trademind/backend/internal/modules/order"
	"github.com/trademind-ai/trademind/backend/internal/modules/shop"
	"gorm.io/gorm"
)

func openTagTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	dsn := fmt.Sprintf("file:order_tags_%s?mode=memory&cache=shared&_pragma=busy_timeout(5000)", uuid.New().String())
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Skipf("sqlite unavailable: %v", err)
	}
	sqlDB, _ := db.DB()
	sqlDB.SetMaxOpenConns(1)
	if err := db.AutoMigrate(&order.Order{}, &order.OrderItem{}, &order.OrderShipment{},
		&order.OrderAutomationRule{}, &order.OrderAutomationLog{},
		&order.OrderTag{}, &order.OrderTagLink{}, &shop.Shop{}); err != nil {
		t.Fatal(err)
	}
	return db
}

func createTag(t *testing.T, svc *order.Service, tenantID int64, name, color string) *order.OrderTag {
	t.Helper()
	row, err := svc.CreateOrderTag(automationTestCtx(tenantID), order.OrderTagBody{Name: name, Color: color}, nil)
	if err != nil {
		t.Fatal(err)
	}
	return row
}

func TestOrderTagCRUDTenantScope(t *testing.T) {
	db := openTagTestDB(t)
	svc := &order.Service{DB: db}

	tag := createTag(t, svc, 1, "加急", "red")
	if tag.Color != "red" {
		t.Fatalf("color = %s", tag.Color)
	}
	// 同租户重名拒绝
	if _, err := svc.CreateOrderTag(automationTestCtx(1), order.OrderTagBody{Name: "加急"}, nil); err == nil {
		t.Fatal("duplicate tag name in one tenant must be rejected")
	}
	// 其他租户可用同名
	if _, err := svc.CreateOrderTag(automationTestCtx(2), order.OrderTagBody{Name: "加急"}, nil); err != nil {
		t.Fatalf("other tenant must reuse name: %v", err)
	}
	// 无效颜色拒绝
	if _, err := svc.CreateOrderTag(automationTestCtx(1), order.OrderTagBody{Name: "怪色", Color: "hotpink"}, nil); err == nil {
		t.Fatal("invalid color must be rejected")
	}
	// 跨租户更新/删除 404
	if _, err := svc.UpdateOrderTag(automationTestCtx(2), tag.ID, order.OrderTagBody{Name: "改名"}, nil); err != order.ErrOrderTagNotFound {
		t.Fatalf("cross-tenant update = %v", err)
	}
	if err := svc.DeleteOrderTag(automationTestCtx(2), tag.ID, nil); err != order.ErrOrderTagNotFound {
		t.Fatalf("cross-tenant delete = %v", err)
	}
	// 正常更新
	upd, err := svc.UpdateOrderTag(automationTestCtx(1), tag.ID, order.OrderTagBody{Name: "特急", Color: "orange"}, nil)
	if err != nil || upd.Name != "特急" || upd.Color != "orange" {
		t.Fatalf("update = %+v, %v", upd, err)
	}
}

func TestOrderTagAttachDetachIdempotent(t *testing.T) {
	db := openTagTestDB(t)
	svc := &order.Service{DB: db}
	tag := createTag(t, svc, 1, "大客户", "gold")
	o := newAutomationOrder(t, db, 1, 100, order.ReviewStatusAutoPassed)

	tags, err := svc.AddOrderTags(automationTestCtx(1), o.ID, []string{tag.ID.String()}, nil)
	if err != nil || len(tags) != 1 {
		t.Fatalf("attach = %v, %v", tags, err)
	}
	// 重复打标幂等：不产生重复行
	if _, err := svc.AddOrderTags(automationTestCtx(1), o.ID, []string{tag.ID.String()}, nil); err != nil {
		t.Fatalf("re-attach: %v", err)
	}
	var n int64
	db.Model(&order.OrderTagLink{}).Where("order_id = ?", o.ID).Count(&n)
	if n != 1 {
		t.Fatalf("links = %d, want 1", n)
	}
	// 跨租户标签拒绝
	other := createTag(t, svc, 2, "别人的", "blue")
	if _, err := svc.AddOrderTags(automationTestCtx(1), o.ID, []string{other.ID.String()}, nil); err == nil {
		t.Fatal("cross-tenant tag must be rejected")
	}
	// 去标
	tags, err = svc.RemoveOrderTag(automationTestCtx(1), o.ID, tag.ID, nil)
	if err != nil || len(tags) != 0 {
		t.Fatalf("detach = %v, %v", tags, err)
	}
	// 删除标签清理链接
	if _, err := svc.AddOrderTags(automationTestCtx(1), o.ID, []string{tag.ID.String()}, nil); err != nil {
		t.Fatal(err)
	}
	if err := svc.DeleteOrderTag(automationTestCtx(1), tag.ID, nil); err != nil {
		t.Fatal(err)
	}
	db.Model(&order.OrderTagLink{}).Where("order_id = ?", o.ID).Count(&n)
	if n != 0 {
		t.Fatalf("links after tag delete = %d, want 0", n)
	}
}

func TestBatchTagOrders(t *testing.T) {
	db := openTagTestDB(t)
	svc := &order.Service{DB: db}
	tag := createTag(t, svc, 1, "批量", "cyan")
	o1 := newAutomationOrder(t, db, 1, 100, order.ReviewStatusAutoPassed)
	o2 := newAutomationOrder(t, db, 1, 200, order.ReviewStatusAutoPassed)

	res, err := svc.BatchTagOrders(automationTestCtx(1), order.BatchOrderTagBody{
		OrderIDs: []string{o1.ID.String(), o2.ID.String()},
		TagIDs:   []string{tag.ID.String()},
	}, nil)
	if err != nil || res.Applied != 2 {
		t.Fatalf("batch add = %+v, %v", res, err)
	}
	// 重复提交幂等：applied 0
	res, err = svc.BatchTagOrders(automationTestCtx(1), order.BatchOrderTagBody{
		OrderIDs: []string{o1.ID.String(), o2.ID.String()},
		TagIDs:   []string{tag.ID.String()},
	}, nil)
	if err != nil || res.Applied != 0 {
		t.Fatalf("batch re-add = %+v, %v", res, err)
	}
	// 跨租户订单整批拒绝
	oOther := newAutomationOrder(t, db, 2, 50, order.ReviewStatusAutoPassed)
	if _, err := svc.BatchTagOrders(automationTestCtx(1), order.BatchOrderTagBody{
		OrderIDs: []string{o1.ID.String(), oOther.ID.String()},
		TagIDs:   []string{tag.ID.String()},
	}, nil); err == nil {
		t.Fatal("batch with cross-tenant order must be rejected")
	}
	// 批量去标
	res, err = svc.BatchTagOrders(automationTestCtx(1), order.BatchOrderTagBody{
		OrderIDs: []string{o1.ID.String(), o2.ID.String()},
		TagIDs:   []string{tag.ID.String()},
		Action:   "remove",
	}, nil)
	if err != nil || res.Removed != 2 {
		t.Fatalf("batch remove = %+v, %v", res, err)
	}
	// 无效动作
	if _, err := svc.BatchTagOrders(automationTestCtx(1), order.BatchOrderTagBody{
		OrderIDs: []string{o1.ID.String()}, TagIDs: []string{tag.ID.String()}, Action: "toggle",
	}, nil); err == nil {
		t.Fatal("invalid action must be rejected")
	}
}

func TestListFilterByTag(t *testing.T) {
	db := openTagTestDB(t)
	svc := &order.Service{DB: db}
	tag := createTag(t, svc, 1, "筛选", "purple")
	o1 := newAutomationOrder(t, db, 1, 100, order.ReviewStatusAutoPassed)
	newAutomationOrder(t, db, 1, 200, order.ReviewStatusAutoPassed)
	if _, err := svc.AddOrderTags(automationTestCtx(1), o1.ID, []string{tag.ID.String()}, nil); err != nil {
		t.Fatal(err)
	}
	res, err := svc.List(automationTestCtx(1), order.ListQuery{Page: 1, PageSize: 20, TagID: &tag.ID})
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Items) != 1 || res.Items[0].ID != o1.ID {
		t.Fatalf("tag filter items = %d", len(res.Items))
	}
	if len(res.Items[0].Tags) != 1 || res.Items[0].Tags[0].Name != "筛选" {
		t.Fatalf("row tags = %+v", res.Items[0].Tags)
	}
}

func TestAutomationAddTagRuleAndEngine(t *testing.T) {
	db := openTagTestDB(t)
	svc := &order.Service{DB: db}
	tag := createTag(t, svc, 1, "自动", "geekblue")

	// add_tag 规则必须选标签
	if _, err := svc.CreateAutomationRule(automationTestCtx(1), order.AutomationRuleBody{
		Name: "打标签规则", TriggerEvent: order.AutomationEventOrderPaid,
		Action: order.AutomationActionAddTag, Enabled: boolPtrA(true),
	}, nil); err == nil {
		t.Fatal("add_tag rule without tags must be rejected")
	}
	// 跨租户/不存在标签拒绝
	if _, err := svc.CreateAutomationRule(automationTestCtx(1), order.AutomationRuleBody{
		Name: "打标签规则", TriggerEvent: order.AutomationEventOrderPaid,
		Action: order.AutomationActionAddTag, Enabled: boolPtrA(true),
		TagIDs: &[]string{uuid.New().String()},
	}, nil); err == nil {
		t.Fatal("add_tag rule with unknown tag must be rejected")
	}
	rule := createAutomationRule(t, db, svc, 1, order.AutomationRuleBody{
		Name: "付款后自动打标", TriggerEvent: order.AutomationEventOrderPaid,
		Action: order.AutomationActionAddTag, Enabled: boolPtrA(true),
		TagIDs: &[]string{tag.ID.String()},
	})

	o := newAutomationOrder(t, db, 1, 100, order.ReviewStatusAutoPassed)
	o.PaymentStatus = order.PaymentPaid
	o.Status = order.StatusPaid
	db.Save(o)

	svc.FireOrderEvent(automationTestCtx(1).Request.Context(), 1, o.ID, order.AutomationEventOrderPaid)
	var n int64
	db.Model(&order.OrderTagLink{}).Where("order_id = ? AND tag_id = ? AND source = ?",
		o.ID, tag.ID, order.TagLinkSourceAutomation).Count(&n)
	if n != 1 {
		t.Fatalf("automation tag links = %d, want 1", n)
	}
	logs := logsFor(t, db, o.ID)
	if len(logs) != 1 || logs[0].Status != order.AutomationLogSuccess || logs[0].RuleID != rule.ID {
		t.Fatalf("logs = %+v", logs)
	}
	// 幂等：重复触发不重复打标、不新增日志
	svc.FireOrderEvent(automationTestCtx(1).Request.Context(), 1, o.ID, order.AutomationEventOrderPaid)
	db.Model(&order.OrderTagLink{}).Where("order_id = ?", o.ID).Count(&n)
	if n != 1 {
		t.Fatalf("links after re-fire = %d", n)
	}
	if got := logsFor(t, db, o.ID); len(got) != 1 {
		t.Fatalf("logs after re-fire = %d", len(got))
	}
	// 审单挂起订单：安全边界跳过（不打标）
	held := newAutomationOrder(t, db, 1, 100, order.ReviewStatusPending)
	svc.FireOrderEvent(automationTestCtx(1).Request.Context(), 1, held.ID, order.AutomationEventOrderPaid)
	db.Model(&order.OrderTagLink{}).Where("order_id = ?", held.ID).Count(&n)
	if n != 0 {
		t.Fatalf("held order must not be tagged, links = %d", n)
	}
	heldLogs := logsFor(t, db, held.ID)
	if len(heldLogs) != 1 || heldLogs[0].Status != order.AutomationLogSkipped {
		t.Fatalf("held logs = %+v", heldLogs)
	}
}
