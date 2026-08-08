package mcpserver_test

import (
	"testing"

	"github.com/trademind-ai/trademind/backend/internal/modules/mcpaudit"
	"github.com/trademind-ai/trademind/backend/internal/modules/mcpwrite"
	"github.com/trademind-ai/trademind/backend/internal/modules/procurement"
)

// R183 audit: every whitelisted write tool must audit exactly once per call
// inside the write pipeline (dry_run / execute rows carrying mode, params
// summary and confirmation hash). A generic entry-level row for the same call
// would duplicate the trail of a money-bearing action and can reject a call
// whose mutation already committed.
func TestWriteToolsAuditExactlyOncePerCall(t *testing.T) {
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
	args["mode"] = "execute"
	args["confirmationToken"] = res.ConfirmationToken
	if out, res = callTagTool(t, sess, "procurement_mark_paid", args); res == nil {
		t.Fatalf("execute error: %s", toolErrorText(t, out))
	}
	var got procurement.PurchaseOrder
	if err := db.First(&got, "id = ?", po.ID).Error; err != nil {
		t.Fatal(err)
	}
	if got.Status != procurement.StatusPaid {
		t.Fatalf("po status after execute = %s", got.Status)
	}

	var rows []mcpaudit.ToolCallLog
	if err := db.Where("tool = ?", "procurement_mark_paid").Find(&rows).Error; err != nil {
		t.Fatal(err)
	}
	if len(rows) != 2 {
		t.Fatalf("mark_paid audit rows = %d, want 2 (dry_run + execute)", len(rows))
	}
	modes := map[string]int{}
	for _, r := range rows {
		modes[r.Mode]++
	}
	if modes[mcpwrite.ModeDryRun] != 1 || modes[mcpwrite.ModeExecute] != 1 {
		t.Fatalf("mark_paid audit modes = %v, want one dry_run + one execute", modes)
	}
}
