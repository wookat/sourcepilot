package mcpserver_test

import (
	"context"
	"fmt"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"github.com/google/uuid"
	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/trademind-ai/trademind/backend/internal/modules/mcpaudit"
	"github.com/trademind-ai/trademind/backend/internal/modules/mcpserver"
	"github.com/trademind-ai/trademind/backend/internal/modules/mcptoken"
	"github.com/trademind-ai/trademind/backend/internal/modules/mcpwrite"
	"github.com/trademind-ai/trademind/backend/internal/modules/order"
	"github.com/trademind-ai/trademind/backend/internal/modules/orderexception"
	"github.com/trademind-ai/trademind/backend/internal/modules/procurement"
	"github.com/trademind-ai/trademind/backend/internal/modules/settings"
	"gorm.io/gorm"
)

// R180 W2: exceptions_mark / procurement_mark_placed /
// procurement_fill_logistics run through the W1 governed pipeline —
// dry-run → confirmation → execute, idempotency, cross-tenant 404 semantics,
// state-machine validation and audit fields.

func openW2TestDB(t *testing.T) *gorm.DB {
	t.Helper()
	dsn := fmt.Sprintf("file:mcpw2_%s?mode=memory&cache=shared&_pragma=busy_timeout(5000)", uuid.New().String())
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Skipf("sqlite unavailable: %v", err)
	}
	sqlDB, _ := db.DB()
	sqlDB.SetMaxOpenConns(1)
	if err := db.AutoMigrate(
		&mcptoken.Token{}, &mcpaudit.ToolCallLog{}, &mcpwrite.Confirmation{},
		&order.Order{}, &settings.Setting{},
		&orderexception.OrderExceptionMark{},
		&procurement.PurchaseOrder{}, &procurement.PurchaseOrderItem{},
		&procurement.PurchaseOrderEvent{}, &procurement.PurchaseLogistics{},
	); err != nil {
		t.Fatal(err)
	}
	if err := db.Exec(`CREATE TABLE tenants (id integer primary key, status text, deleted_at datetime)`).Error; err != nil {
		t.Fatal(err)
	}
	return db
}

func newW2TestServer(t *testing.T, db *gorm.DB) (*httptest.Server, *mcptoken.Service, *mcpaudit.Service) {
	t.Helper()
	gin.SetMode(gin.TestMode)
	tokens := &mcptoken.Service{DB: db}
	audits := &mcpaudit.Service{DB: db}
	r := gin.New()
	r.POST("/api/mcp", mcpserver.GinHandler(&mcpserver.Deps{
		DB:           db,
		Tokens:       tokens,
		Audits:       audits,
		RateRPS:      1000,
		RateBurst:    1000,
		Version:      "test",
		WriteEnabled: true,
		Orders:       &order.Service{DB: db},
		Exceptions:   &orderexception.Service{DB: db},
		Procurement:  &procurement.Service{DB: db},
		Settings:     &settings.Service{DB: db},
	}))
	srv := httptest.NewServer(r)
	t.Cleanup(srv.Close)
	return srv, tokens, audits
}

func seedW2Orders(t *testing.T, db *gorm.DB) (t1 order.Order, t2 order.Order) {
	t.Helper()
	t1 = order.Order{TenantID: 1, OrderNo: "T1-W2", Platform: "douyin", Status: "pending", Currency: "CNY"}
	t2 = order.Order{TenantID: 2, OrderNo: "T2-W2", Platform: "shopee", Status: "pending", Currency: "USD"}
	for _, o := range []*order.Order{&t1, &t2} {
		if err := db.Create(o).Error; err != nil {
			t.Fatal(err)
		}
	}
	return t1, t2
}

func seedPO(t *testing.T, db *gorm.DB, tenantID int64, status string) procurement.PurchaseOrder {
	t.Helper()
	po := procurement.PurchaseOrder{
		TenantID:       tenantID,
		SupplierID:     uuid.New(),
		SupplierName:   "测试供应商",
		Status:         status,
		TotalAmount:    88,
		IdempotencyKey: uuid.NewString(),
	}
	if err := db.Create(&po).Error; err != nil {
		t.Fatal(err)
	}
	return po
}

func dryThenExecute(t *testing.T, sess *mcp.ClientSession, tool string, args map[string]any) *mcpwrite.Result {
	t.Helper()
	args["mode"] = "dry_run"
	out, res := callTagTool(t, sess, tool, args)
	if res == nil {
		t.Fatalf("%s dry_run error: %s", tool, toolErrorText(t, out))
	}
	if res.ConfirmationToken == "" {
		t.Fatalf("%s dry_run returned no confirmation token", tool)
	}
	args["mode"] = "execute"
	args["confirmationToken"] = res.ConfirmationToken
	out, res = callTagTool(t, sess, tool, args)
	if res == nil {
		t.Fatalf("%s execute error: %s", tool, toolErrorText(t, out))
	}
	return res
}

// exceptions_mark: handle → idempotent re-handle → ignore flips → unmark
// removes; all via dry-run + confirmation.
func TestExceptionsMarkLifecycle(t *testing.T) {
	db := openW2TestDB(t)
	o1, _ := seedW2Orders(t, db)
	enableTenantWrite(t, db, 1)
	srv, tokens, _ := newW2TestServer(t, db)
	sess := connect(t, srv.URL, newWriteToken(t, tokens, 1))

	base := func() map[string]any {
		return map[string]any{
			"sourceType":    orderexception.SourceOrder,
			"sourceId":      o1.ID.String(),
			"exceptionType": orderexception.TypeSKUUnmatched,
		}
	}

	args := base()
	args["action"] = "handle"
	args["remark"] = "已线下处理"
	dryThenExecute(t, sess, "exceptions_mark", args)
	var count int64
	db.Model(&orderexception.OrderExceptionMark{}).
		Where("source_id = ? AND mark_type = ?", o1.ID.String(), orderexception.MarkHandled).Count(&count)
	if count != 1 {
		t.Fatalf("handled marks = %d, want 1", count)
	}

	// Idempotent re-handle keeps one row.
	args = base()
	args["action"] = "handle"
	dryThenExecute(t, sess, "exceptions_mark", args)
	db.Model(&orderexception.OrderExceptionMark{}).
		Where("source_id = ? AND mark_type = ?", o1.ID.String(), orderexception.MarkHandled).Count(&count)
	if count != 1 {
		t.Fatalf("handled marks after re-handle = %d, want 1", count)
	}

	// Ignore flips the mark (opposite removed).
	args = base()
	args["action"] = "ignore"
	dryThenExecute(t, sess, "exceptions_mark", args)
	db.Model(&orderexception.OrderExceptionMark{}).
		Where("source_id = ?", o1.ID.String()).Count(&count)
	if count != 1 {
		t.Fatalf("marks after ignore = %d, want 1", count)
	}

	// Unmark removes everything; a second unmark is an idempotent no-op.
	args = base()
	args["action"] = "unmark"
	dryThenExecute(t, sess, "exceptions_mark", args)
	db.Model(&orderexception.OrderExceptionMark{}).
		Where("source_id = ?", o1.ID.String()).Count(&count)
	if count != 0 {
		t.Fatalf("marks after unmark = %d, want 0", count)
	}
	args = base()
	args["action"] = "unmark"
	dryThenExecute(t, sess, "exceptions_mark", args)
}

// Cross-tenant / missing sources answer identical not-found errors.
func TestExceptionsMarkCrossTenant404(t *testing.T) {
	db := openW2TestDB(t)
	_, o2 := seedW2Orders(t, db)
	enableTenantWrite(t, db, 1)
	srv, tokens, _ := newW2TestServer(t, db)
	sess := connect(t, srv.URL, newWriteToken(t, tokens, 1))

	for _, sid := range []string{o2.ID.String(), uuid.NewString()} {
		out, res := callTagTool(t, sess, "exceptions_mark", map[string]any{
			"sourceType":    orderexception.SourceOrder,
			"sourceId":      sid,
			"exceptionType": orderexception.TypeSKUUnmatched,
			"action":        "handle",
			"mode":          "dry_run",
		})
		if res != nil {
			t.Fatalf("cross-tenant/missing source %s accepted", sid)
		}
		if msg := toolErrorText(t, out); !strings.Contains(msg, "记录不存在") {
			t.Fatalf("not-found message = %q", msg)
		}
	}
}

func TestExceptionsMarkRejectsBadInput(t *testing.T) {
	db := openW2TestDB(t)
	o1, _ := seedW2Orders(t, db)
	enableTenantWrite(t, db, 1)
	srv, tokens, _ := newW2TestServer(t, db)
	sess := connect(t, srv.URL, newWriteToken(t, tokens, 1))

	out, res := callTagTool(t, sess, "exceptions_mark", map[string]any{
		"sourceType":    orderexception.SourceOrder,
		"sourceId":      o1.ID.String(),
		"exceptionType": orderexception.TypeSKUUnmatched,
		"action":        "delete-everything",
		"mode":          "dry_run",
	})
	if res != nil {
		t.Fatal("invalid action accepted")
	}
	if msg := toolErrorText(t, out); !strings.Contains(msg, "action 非法") {
		t.Fatalf("bad-action message = %q", msg)
	}
}

// procurement_mark_placed: placing → placed with external order id, audit
// carries mode/paramsSummary/confirmHash; execute without confirmation is
// rejected.
func TestProcurementMarkPlaced(t *testing.T) {
	db := openW2TestDB(t)
	enableTenantWrite(t, db, 1)
	po := seedPO(t, db, 1, procurement.StatusPlacing)
	srv, tokens, audits := newW2TestServer(t, db)
	sess := connect(t, srv.URL, newWriteToken(t, tokens, 1))

	// execute without dry-run/confirmation fails.
	out, res := callTagTool(t, sess, "procurement_mark_placed", map[string]any{
		"purchaseOrderId": po.ID.String(), "externalOrderId": "1688-X1", "mode": "execute",
	})
	if res != nil {
		t.Fatal("execute without confirmation accepted")
	}
	_ = toolErrorText(t, out)

	dryThenExecute(t, sess, "procurement_mark_placed", map[string]any{
		"purchaseOrderId": po.ID.String(), "externalOrderId": "1688-X1",
	})
	var got procurement.PurchaseOrder
	if err := db.First(&got, "id = ?", po.ID).Error; err != nil {
		t.Fatal(err)
	}
	if got.Status != procurement.StatusPlaced || got.ExternalOrderID != "1688-X1" {
		t.Fatalf("po after execute: status=%s ext=%s", got.Status, got.ExternalOrderID)
	}

	// Audit rows carry the W1 fields surfaced in the admin UI.
	logs, err := audits.List(context.Background(), 1, mcpaudit.ListFilter{Tool: "procurement_mark_placed", Page: 1, PageSize: 10})
	if err != nil {
		t.Fatal(err)
	}
	var sawExecute bool
	for _, r := range logs.Items {
		if r.Status != "success" {
			continue
		}
		if r.ParamsSummary == "" || r.ConfirmHash == "" {
			t.Fatalf("audit row missing summary/hash: %+v", r)
		}
		if r.Mode == mcpwrite.ModeExecute {
			sawExecute = true
		}
	}
	if !sawExecute {
		t.Fatal("no execute audit row recorded")
	}
}

// Illegal state transitions are rejected at dry-run (no confirmation issued).
func TestProcurementMarkPlacedIllegalTransition(t *testing.T) {
	db := openW2TestDB(t)
	enableTenantWrite(t, db, 1)
	po := seedPO(t, db, 1, procurement.StatusDraft)
	srv, tokens, _ := newW2TestServer(t, db)
	sess := connect(t, srv.URL, newWriteToken(t, tokens, 1))

	out, res := callTagTool(t, sess, "procurement_mark_placed", map[string]any{
		"purchaseOrderId": po.ID.String(), "externalOrderId": "1688-X2", "mode": "dry_run",
	})
	if res != nil {
		t.Fatal("illegal transition accepted at dry_run")
	}
	if msg := toolErrorText(t, out); !strings.Contains(msg, "不允许") {
		t.Fatalf("illegal-transition message = %q", msg)
	}
}

// Cross-tenant / missing purchase orders answer identical not-found errors.
func TestProcurementWriteCrossTenant404(t *testing.T) {
	db := openW2TestDB(t)
	enableTenantWrite(t, db, 1)
	po2 := seedPO(t, db, 2, procurement.StatusPlacing)
	srv, tokens, _ := newW2TestServer(t, db)
	sess := connect(t, srv.URL, newWriteToken(t, tokens, 1))

	for _, id := range []string{po2.ID.String(), uuid.NewString(), "not-a-uuid"} {
		out, res := callTagTool(t, sess, "procurement_mark_placed", map[string]any{
			"purchaseOrderId": id, "externalOrderId": "1688-X3", "mode": "dry_run",
		})
		if res != nil {
			t.Fatalf("cross-tenant/missing po %s accepted", id)
		}
		if msg := toolErrorText(t, out); !strings.Contains(msg, "采购单不存在") {
			t.Fatalf("not-found message = %q", msg)
		}
	}
}

// procurement_fill_logistics: paid → shipped, creates the logistics row.
func TestProcurementFillLogistics(t *testing.T) {
	db := openW2TestDB(t)
	enableTenantWrite(t, db, 1)
	po := seedPO(t, db, 1, procurement.StatusPaid)
	srv, tokens, _ := newW2TestServer(t, db)
	sess := connect(t, srv.URL, newWriteToken(t, tokens, 1))

	dryThenExecute(t, sess, "procurement_fill_logistics", map[string]any{
		"purchaseOrderId": po.ID.String(), "trackingNo": "SF123456", "carrier": "顺丰",
	})
	var got procurement.PurchaseOrder
	if err := db.First(&got, "id = ?", po.ID).Error; err != nil {
		t.Fatal(err)
	}
	if got.Status != procurement.StatusShipped {
		t.Fatalf("po status = %s, want shipped", got.Status)
	}
	var lg procurement.PurchaseLogistics
	if err := db.First(&lg, "purchase_order_id = ?", po.ID).Error; err != nil {
		t.Fatal(err)
	}
	if lg.TrackingNo != "SF123456" || lg.Carrier != "顺丰" || lg.TenantID != 1 {
		t.Fatalf("logistics row = %+v", lg)
	}

	// Not paid yet → illegal transition at dry-run.
	po2 := seedPO(t, db, 1, procurement.StatusPlaced)
	out, res := callTagTool(t, sess, "procurement_fill_logistics", map[string]any{
		"purchaseOrderId": po2.ID.String(), "trackingNo": "SF999", "mode": "dry_run",
	})
	if res != nil {
		t.Fatal("illegal transition accepted at dry_run")
	}
	if msg := toolErrorText(t, out); !strings.Contains(msg, "不允许") {
		t.Fatalf("illegal-transition message = %q", msg)
	}
}
