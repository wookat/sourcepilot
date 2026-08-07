package mcpserver_test

import (
	"net/http"
	"strings"
	"testing"

	"github.com/trademind-ai/trademind/backend/internal/modules/mcpaudit"
	"github.com/trademind-ai/trademind/backend/internal/modules/mcptoken"
)

// R154 P2 regression: rejected credentials at the MCP entry leave a throttled
// auth_failed audit row under tenant 0, so brute-force attempts are visible in
// the audit table and not only in logs.
func TestMcpAuthFailureWritesAuditRow(t *testing.T) {
	db := openTestDB(t)
	srv, _, _ := newAuditedServer(t, db)

	bad := mcptoken.TokenPrefix + strings.Repeat("0", 64)
	for i := 0; i < 3; i++ {
		if st, _ := rpc(t, srv.URL, bad, `{"jsonrpc":"2.0","id":1,"method":"tools/list"}`); st != http.StatusUnauthorized {
			t.Fatalf("attempt %d: status %d, want 401", i, st)
		}
	}

	var n int64
	if err := db.Model(&mcpaudit.ToolCallLog{}).
		Where("tool = ? AND status = ? AND tenant_id = 0", "mcp:auth", mcpaudit.StatusAuthFailed).
		Count(&n).Error; err != nil {
		t.Fatal(err)
	}
	if n != 1 {
		t.Fatalf("want 1 throttled auth_failed audit row, got %d", n)
	}
}
