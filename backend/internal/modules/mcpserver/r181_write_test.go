package mcpserver_test

import (
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/trademind-ai/trademind/backend/internal/modules/mcpaudit"
	"github.com/trademind-ai/trademind/backend/internal/modules/mcpserver"
	"github.com/trademind-ai/trademind/backend/internal/modules/mcpwrite"
	"github.com/trademind-ai/trademind/backend/internal/modules/procurement"
	"github.com/trademind-ai/trademind/backend/internal/modules/settings"
	"gorm.io/gorm"
)

// R181 W3: procurement_mark_paid runs through the W1 governed pipeline and
// adds the decision-brief preconditions — tenant amount ceilings (default 0 =
// unavailable), amount/currency must match the purchase order, dry-run echoes
// the amount and order line details, over-limit / unconfigured calls are
// rejected and audited.

func setMarkPaidLimits(t *testing.T, db *gorm.DB, tenantID int64, single, daily string) {
	t.Helper()
	for key, val := range map[string]string{
		mcpserver.SettingsKeyMarkPaidSingleLimit: single,
		mcpserver.SettingsKeyMarkPaidDailyLimit:  daily,
	} {
		if err := db.Create(&settings.Setting{
			TenantID:  tenantID,
			GroupKey:  mcpwrite.SettingsGroupMCP,
			ItemKey:   key,
			ItemValue: val,
		}).Error; err != nil {
			t.Fatal(err)
		}
	}
}

func seedPaidReadyPO(t *testing.T, db *gorm.DB, tenantID int64, amount float64, currency string) procurement.PurchaseOrder {
	t.Helper()
	po := procurement.PurchaseOrder{
		TenantID:       tenantID,
		SupplierID:     uuid.New(),
		SupplierName:   "测试供应商",
		Status:         procurement.StatusPlaced,
		TotalAmount:    amount,
		Currency:       currency,
		IdempotencyKey: uuid.NewString(),
	}
	if err := db.Create(&po).Error; err != nil {
		t.Fatal(err)
	}
	item := procurement.PurchaseOrderItem{
		TenantID:        tenantID,
		PurchaseOrderID: po.ID,
		ProductTitle:    "测试商品",
		SKUName:         "红色/L",
		Quantity:        2,
	}
	if err := db.Create(&item).Error; err != nil {
		t.Fatal(err)
	}
	return po
}

func markPaidArgs(po procurement.PurchaseOrder, amount float64, currency string) map[string]any {
	return map[string]any{
		"purchaseOrderId": po.ID.String(),
		"amount":          amount,
		"currency":        currency,
	}
}

// Happy path: ceilings configured, amount/currency match → dry-run echoes
// amount + line details, execute flips placed → paid and audits the amount.
func TestMarkPaidHappyPath(t *testing.T) {
	db := openW2TestDB(t)
	enableTenantWrite(t, db, 1)
	setMarkPaidLimits(t, db, 1, "100", "300")
	po := seedPaidReadyPO(t, db, 1, 88, "CNY")
	srv, tokens, _ := newW2TestServer(t, db)
	sess := connect(t, srv.URL, newWriteToken(t, tokens, 1))

	args := markPaidArgs(po, 88, "CNY")
	args["mode"] = "dry_run"
	out, res := callTagTool(t, sess, "procurement_mark_paid", args)
	if res == nil {
		t.Fatalf("dry_run error: %s", toolErrorText(t, out))
	}
	if !strings.Contains(res.Summary, "88.00") || !strings.Contains(res.Summary, "CNY") {
		t.Fatalf("dry-run summary must echo amount+currency: %q", res.Summary)
	}
	preview, ok := res.Preview.(map[string]any)
	if !ok {
		t.Fatalf("preview type %T", res.Preview)
	}
	if preview["amount"] != 88.0 || preview["currency"] != "CNY" {
		t.Fatalf("preview amount/currency = %v/%v", preview["amount"], preview["currency"])
	}
	items, ok := preview["items"].([]any)
	if !ok || len(items) != 1 {
		t.Fatalf("preview items = %v", preview["items"])
	}

	args["mode"] = "execute"
	args["confirmationToken"] = res.ConfirmationToken
	out, res = callTagTool(t, sess, "procurement_mark_paid", args)
	if res == nil {
		t.Fatalf("execute error: %s", toolErrorText(t, out))
	}
	var got procurement.PurchaseOrder
	if err := db.First(&got, "id = ?", po.ID).Error; err != nil {
		t.Fatal(err)
	}
	if got.Status != procurement.StatusPaid || got.PayStatus != procurement.PayStatusPaid || got.PaidAt == nil {
		t.Fatalf("po after execute: status=%s payStatus=%s paidAt=%v", got.Status, got.PayStatus, got.PaidAt)
	}

	var row mcpaudit.ToolCallLog
	if err := db.Where("tool = ? AND mode = ? AND status = ?", "procurement_mark_paid", mcpwrite.ModeExecute, "success").
		First(&row).Error; err != nil {
		t.Fatal(err)
	}
	if row.Amount != 88 {
		t.Fatalf("execute audit amount = %v, want 88", row.Amount)
	}
}

// Unconfigured (or partially / non-positive / malformed configured) ceilings
// keep the tool rejected fail-closed, and the rejection is audited.
func TestMarkPaidUnconfiguredRejectedAndAudited(t *testing.T) {
	db := openW2TestDB(t)
	enableTenantWrite(t, db, 1)
	po := seedPaidReadyPO(t, db, 1, 88, "CNY")
	srv, tokens, _ := newW2TestServer(t, db)
	sess := connect(t, srv.URL, newWriteToken(t, tokens, 1))

	args := markPaidArgs(po, 88, "CNY")
	args["mode"] = "dry_run"
	out, res := callTagTool(t, sess, "procurement_mark_paid", args)
	if res != nil {
		t.Fatal("unconfigured ceilings accepted")
	}
	if msg := toolErrorText(t, out); !strings.Contains(msg, "未配置") {
		t.Fatalf("unconfigured message = %q", msg)
	}
	var count int64
	db.Model(&mcpaudit.ToolCallLog{}).
		Where("tool = ? AND status = ?", "procurement_mark_paid", "error").Count(&count)
	if count == 0 {
		t.Fatal("unconfigured rejection not audited")
	}
}

// Zero / negative / malformed ceiling values all mean unconfigured.
func TestMarkPaidBadLimitValuesRejected(t *testing.T) {
	for _, vals := range [][2]string{{"0", "100"}, {"100", "0"}, {"-5", "100"}, {"abc", "100"}, {"100", ""}} {
		db := openW2TestDB(t)
		enableTenantWrite(t, db, 1)
		setMarkPaidLimits(t, db, 1, vals[0], vals[1])
		po := seedPaidReadyPO(t, db, 1, 88, "CNY")
		srv, tokens, _ := newW2TestServer(t, db)
		sess := connect(t, srv.URL, newWriteToken(t, tokens, 1))
		args := markPaidArgs(po, 88, "CNY")
		args["mode"] = "dry_run"
		out, res := callTagTool(t, sess, "procurement_mark_paid", args)
		if res != nil {
			t.Fatalf("ceilings %v accepted", vals)
		}
		if msg := toolErrorText(t, out); !strings.Contains(msg, "未配置") {
			t.Fatalf("ceilings %v message = %q", vals, msg)
		}
	}
}

// Amount over the per-call ceiling is rejected 403-style and audited.
func TestMarkPaidOverSingleLimit(t *testing.T) {
	db := openW2TestDB(t)
	enableTenantWrite(t, db, 1)
	setMarkPaidLimits(t, db, 1, "50", "300")
	po := seedPaidReadyPO(t, db, 1, 88, "CNY")
	srv, tokens, _ := newW2TestServer(t, db)
	sess := connect(t, srv.URL, newWriteToken(t, tokens, 1))

	args := markPaidArgs(po, 88, "CNY")
	args["mode"] = "dry_run"
	out, res := callTagTool(t, sess, "procurement_mark_paid", args)
	if res != nil {
		t.Fatal("over-single-limit accepted")
	}
	if msg := toolErrorText(t, out); !strings.Contains(msg, "单笔上限") {
		t.Fatalf("over-single message = %q", msg)
	}
	var count int64
	db.Model(&mcpaudit.ToolCallLog{}).
		Where("tool = ? AND status = ?", "procurement_mark_paid", "error").Count(&count)
	if count == 0 {
		t.Fatal("over-limit rejection not audited")
	}
}

// The daily cumulative ceiling sums executed amounts: a second payment that
// would cross the ceiling is rejected — including at execute time, so a
// confirmation issued before the ceiling filled up cannot bypass it.
func TestMarkPaidOverDailyLimit(t *testing.T) {
	db := openW2TestDB(t)
	enableTenantWrite(t, db, 1)
	setMarkPaidLimits(t, db, 1, "100", "150")
	po1 := seedPaidReadyPO(t, db, 1, 88, "CNY")
	po2 := seedPaidReadyPO(t, db, 1, 88, "CNY")
	srv, tokens, _ := newW2TestServer(t, db)
	sess := connect(t, srv.URL, newWriteToken(t, tokens, 1))

	// po2 dry-run passes while the day is still empty: confirmation issued.
	args2 := markPaidArgs(po2, 88, "CNY")
	args2["mode"] = "dry_run"
	out, res2 := callTagTool(t, sess, "procurement_mark_paid", args2)
	if res2 == nil {
		t.Fatalf("po2 dry_run error: %s", toolErrorText(t, out))
	}

	// po1 executes and consumes 88 of the 150 ceiling.
	dryThenExecute(t, sess, "procurement_mark_paid", markPaidArgs(po1, 88, "CNY"))

	// po2 execute with its earlier confirmation must now be rejected.
	args2["mode"] = "execute"
	args2["confirmationToken"] = res2.ConfirmationToken
	out, res2 = callTagTool(t, sess, "procurement_mark_paid", args2)
	if res2 != nil {
		t.Fatal("execute crossing the daily ceiling accepted")
	}
	if msg := toolErrorText(t, out); !strings.Contains(msg, "日累计上限") {
		t.Fatalf("over-daily message = %q", msg)
	}
	var got procurement.PurchaseOrder
	if err := db.First(&got, "id = ?", po2.ID).Error; err != nil {
		t.Fatal(err)
	}
	if got.Status != procurement.StatusPlaced {
		t.Fatalf("po2 mutated despite daily-ceiling rejection: %s", got.Status)
	}
}

// Amount or currency differing from the purchase order is rejected outright.
func TestMarkPaidMismatchRejected(t *testing.T) {
	db := openW2TestDB(t)
	enableTenantWrite(t, db, 1)
	setMarkPaidLimits(t, db, 1, "1000", "3000")
	po := seedPaidReadyPO(t, db, 1, 88, "CNY")
	srv, tokens, _ := newW2TestServer(t, db)
	sess := connect(t, srv.URL, newWriteToken(t, tokens, 1))

	for _, c := range []struct {
		amount   float64
		currency string
	}{
		{87.99, "CNY"}, // amount off by a cent
		{88.01, "CNY"},
		{88, "USD"}, // currency confusion
	} {
		args := markPaidArgs(po, c.amount, c.currency)
		args["mode"] = "dry_run"
		out, res := callTagTool(t, sess, "procurement_mark_paid", args)
		if res != nil {
			t.Fatalf("mismatch %v %s accepted", c.amount, c.currency)
		}
		if msg := toolErrorText(t, out); !strings.Contains(msg, "不一致") {
			t.Fatalf("mismatch message = %q", msg)
		}
	}
}

// Zero, negative and sub-cent precision amounts are invalid inputs.
func TestMarkPaidAmountBoundaries(t *testing.T) {
	db := openW2TestDB(t)
	enableTenantWrite(t, db, 1)
	setMarkPaidLimits(t, db, 1, "1000", "3000")
	po := seedPaidReadyPO(t, db, 1, 88, "CNY")
	srv, tokens, _ := newW2TestServer(t, db)
	sess := connect(t, srv.URL, newWriteToken(t, tokens, 1))

	for _, amount := range []float64{0, -88, 88.001, 1e12} {
		args := markPaidArgs(po, amount, "CNY")
		args["mode"] = "dry_run"
		out, res := callTagTool(t, sess, "procurement_mark_paid", args)
		if res != nil {
			t.Fatalf("amount %v accepted", amount)
		}
		if msg := toolErrorText(t, out); !strings.Contains(msg, "amount 非法") {
			t.Fatalf("amount %v message = %q", amount, msg)
		}
	}
}

// Cross-tenant / missing purchase orders answer identical not-found errors.
func TestMarkPaidCrossTenant404(t *testing.T) {
	db := openW2TestDB(t)
	enableTenantWrite(t, db, 1)
	setMarkPaidLimits(t, db, 1, "1000", "3000")
	po2 := seedPaidReadyPO(t, db, 2, 88, "CNY")
	srv, tokens, _ := newW2TestServer(t, db)
	sess := connect(t, srv.URL, newWriteToken(t, tokens, 1))

	for _, id := range []string{po2.ID.String(), uuid.NewString(), "not-a-uuid"} {
		out, res := callTagTool(t, sess, "procurement_mark_paid", map[string]any{
			"purchaseOrderId": id, "amount": 88, "currency": "CNY", "mode": "dry_run",
		})
		if res != nil {
			t.Fatalf("cross-tenant/missing po %s accepted", id)
		}
		if msg := toolErrorText(t, out); !strings.Contains(msg, "采购单不存在") {
			t.Fatalf("not-found message = %q", msg)
		}
	}
	var got procurement.PurchaseOrder
	if err := db.First(&got, "id = ?", po2.ID).Error; err != nil {
		t.Fatal(err)
	}
	if got.Status != procurement.StatusPlaced {
		t.Fatalf("tenant-2 po mutated: %s", got.Status)
	}
}

// A confirmation issued to tenant 1 cannot be spent by a write token of
// another tenant (token/tenant binding), on top of the same-tenant
// cross-token binding already covered by TestConfirmationHardening.
func TestMarkPaidConfirmationCrossTenantRejected(t *testing.T) {
	db := openW2TestDB(t)
	enableTenantWrite(t, db, 1)
	enableTenantWrite(t, db, 2)
	setMarkPaidLimits(t, db, 1, "1000", "3000")
	setMarkPaidLimits(t, db, 2, "1000", "3000")
	po := seedPaidReadyPO(t, db, 1, 88, "CNY")
	srv, tokens, _ := newW2TestServer(t, db)
	sess1 := connect(t, srv.URL, newWriteToken(t, tokens, 1))
	sess2 := connect(t, srv.URL, newWriteToken(t, tokens, 2))

	args := markPaidArgs(po, 88, "CNY")
	args["mode"] = "dry_run"
	out, res := callTagTool(t, sess1, "procurement_mark_paid", args)
	if res == nil {
		t.Fatalf("dry_run error: %s", toolErrorText(t, out))
	}
	args["mode"] = "execute"
	args["confirmationToken"] = res.ConfirmationToken
	out, hijack := callTagTool(t, sess2, "procurement_mark_paid", args)
	if hijack != nil {
		t.Fatal("tenant-2 token spent tenant-1 confirmation")
	}
	_ = toolErrorText(t, out)
	var got procurement.PurchaseOrder
	if err := db.First(&got, "id = ?", po.ID).Error; err != nil {
		t.Fatal(err)
	}
	if got.Status != procurement.StatusPlaced {
		t.Fatalf("po mutated by cross-tenant execute: %s", got.Status)
	}
}

// Only placed orders may be marked paid (state machine guards the write).
func TestMarkPaidIllegalTransition(t *testing.T) {
	db := openW2TestDB(t)
	enableTenantWrite(t, db, 1)
	setMarkPaidLimits(t, db, 1, "1000", "3000")
	po := seedPO(t, db, 1, procurement.StatusDraft)
	srv, tokens, _ := newW2TestServer(t, db)
	sess := connect(t, srv.URL, newWriteToken(t, tokens, 1))

	out, res := callTagTool(t, sess, "procurement_mark_paid", map[string]any{
		"purchaseOrderId": po.ID.String(), "amount": 88, "currency": "CNY", "mode": "dry_run",
	})
	if res != nil {
		t.Fatal("illegal transition accepted at dry_run")
	}
	if msg := toolErrorText(t, out); !strings.Contains(msg, "不允许") {
		t.Fatalf("illegal-transition message = %q", msg)
	}
}
