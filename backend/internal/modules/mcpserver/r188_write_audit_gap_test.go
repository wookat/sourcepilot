package mcpserver_test

import (
	"context"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/trademind-ai/trademind/backend/internal/modules/mcpaudit"
	"github.com/trademind-ai/trademind/backend/internal/modules/mcptoken"
	"github.com/trademind-ai/trademind/backend/internal/modules/procurement"
)

// R188: no tools/call may bypass the audit trail (docs/mcp.md). Write tools
// are skipped by the entry-level audit middleware because the write pipeline
// audits them itself — but a call rejected before it reaches the pipeline
// (parameter validation) or refused because the tool is not registered for
// the caller's scope never enters the pipeline, so it must still leave an
// entry-level row.

func markPaidArgsMode(po procurement.PurchaseOrder, amount float64, currency, mode string) map[string]any {
	args := markPaidArgs(po, amount, currency)
	args["mode"] = mode
	return args
}

// TestWriteToolPreflightRejectionAudited covers parameter validation
// rejections that return before mcpwrite.Run.
func TestWriteToolPreflightRejectionAudited(t *testing.T) {
	db := openW2TestDB(t)
	enableTenantWrite(t, db, 1)
	setMarkPaidLimits(t, db, 1, "100", "300")
	po := seedPaidReadyPO(t, db, 1, 88, "CNY")
	srv, tokens, _ := newW2TestServer(t, db)
	sess := connect(t, srv.URL, newWriteToken(t, tokens, 1))

	cases := []struct {
		name string
		tool string
		args map[string]any
	}{
		{"empty orderNo", "orders_add_tag", map[string]any{"orderNo": "", "tagName": "x", "mode": "dry_run"}},
		{"blank tagName", "orders_remove_tag", map[string]any{"orderNo": "T1-W2", "tagName": "  ", "mode": "dry_run"}},
		{"negative amount", "procurement_mark_paid", markPaidArgsMode(po, -1, "CNY", "dry_run")},
		{"three decimals", "procurement_mark_paid", markPaidArgsMode(po, 88.005, "CNY", "dry_run")},
		{"amount over 1e10", "procurement_mark_paid", markPaidArgsMode(po, 1e12, "CNY", "dry_run")},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var before, after int64
			db.Model(&mcpaudit.ToolCallLog{}).Count(&before)
			out, res := callTagTool(t, sess, tc.tool, tc.args)
			if res != nil {
				t.Fatalf("%s: expected rejection, got result", tc.name)
			}
			_ = toolErrorText(t, out)
			db.Model(&mcpaudit.ToolCallLog{}).Count(&after)
			if after-before != 1 {
				t.Fatalf("%s: audit rows written = %d, want 1 (no call may bypass the audit trail)", tc.name, after-before)
			}
			var row mcpaudit.ToolCallLog
			if err := db.Order("created_at DESC").First(&row).Error; err != nil {
				t.Fatal(err)
			}
			if row.Tool != tc.tool || row.Status != mcpaudit.StatusError {
				t.Fatalf("%s: audit row = tool %q status %q, want %q/error", tc.name, row.Tool, row.Status, tc.tool)
			}
		})
	}
}

// TestWriteToolProbeByReadonlyTokenAudited covers the reconnaissance case: a
// readonly-scope token does not get the write tools registered, so the call
// fails as "unknown tool" without ever reaching the write pipeline. Such
// probes of the write whitelist must still be auditable.
func TestWriteToolProbeByReadonlyTokenAudited(t *testing.T) {
	db := openW2TestDB(t)
	enableTenantWrite(t, db, 1)
	srv, tokens, _ := newW2TestServer(t, db)
	res, err := tokens.CreateScoped(context.Background(), 1, "reader", "",
		[]string{mcptoken.ScopeReadonly}, nil, nil)
	if err != nil {
		t.Fatal(err)
	}

	for _, tool := range []string{"orders_add_tag", "procurement_mark_paid"} {
		// The transport drops the session after a protocol-level error, so
		// each probe opens its own connection.
		sess := connect(t, srv.URL, res.Plaintext)
		var before, after int64
		db.Model(&mcpaudit.ToolCallLog{}).Count(&before)
		if _, cerr := sess.CallTool(context.Background(), &mcp.CallToolParams{
			Name: tool, Arguments: map[string]any{"mode": "dry_run"},
		}); cerr == nil {
			t.Fatalf("%s: readonly token must not be able to call a write tool", tool)
		}
		db.Model(&mcpaudit.ToolCallLog{}).Count(&after)
		if after-before != 1 {
			t.Fatalf("%s probe by readonly token: audit rows written = %d, want 1", tool, after-before)
		}
	}
}
