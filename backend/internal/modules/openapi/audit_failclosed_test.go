package openapi_test

import (
	"context"
	"net/http"
	"strings"
	"testing"

	"github.com/trademind-ai/trademind/backend/internal/modules/mcptoken"
)

// An unavailable audit store must withhold the query result instead of serving
// an unaudited read (same fail-closed policy as the MCP entry).
func TestAuditWriteFailureRejectsCall(t *testing.T) {
	db := openTestDB(t)
	seedOrders(t, db)
	srv, tokens := newTestServer(t, db, 100, 100)
	tok, err := tokens.Create(context.Background(), 1, "audited", mcptoken.PurposeOpenAPI, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	if res, _ := get(t, srv, "/api/open/v1/orders", tok.Plaintext); res.StatusCode != http.StatusOK {
		t.Fatalf("precondition: want 200 got %d", res.StatusCode)
	}
	if err := db.Exec(`DROP TABLE mcp_tool_call_logs`).Error; err != nil {
		t.Fatal(err)
	}

	res, body := get(t, srv, "/api/open/v1/orders", tok.Plaintext)
	if res.StatusCode != http.StatusInternalServerError {
		t.Fatalf("want 500 when the audit store is down, got %d (%v)", res.StatusCode, body)
	}
	if body["data"] != nil {
		t.Fatalf("query result leaked without an audit row: %v", body["data"])
	}
	if msg, _ := body["message"].(string); !strings.Contains(msg, "audit") {
		t.Fatalf("unexpected error message %q", msg)
	}
}
