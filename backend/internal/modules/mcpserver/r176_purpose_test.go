package mcpserver_test

import (
	"context"
	"net/http"
	"strings"
	"testing"

	"github.com/trademind-ai/trademind/backend/internal/modules/mcptoken"
)

// TestOpenAPIPurposeTokenRejectedAtMCPEntry closes the reverse direction of the
// token purpose gate (R176): the Open API entry already rejects mcp-purpose
// tokens, and the MCP entry must likewise reject openapi-purpose tokens while
// still accepting both-purpose tokens.
func TestOpenAPIPurposeTokenRejectedAtMCPEntry(t *testing.T) {
	srv, tokens := newTestServer(t, openTestDB(t), 100, 100)

	openTok, err := tokens.Create(context.Background(), 1, "openapi-only", mcptoken.PurposeOpenAPI, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	req, _ := http.NewRequest(http.MethodPost, srv.URL+"/api/mcp", strings.NewReader("{}"))
	req.Header.Set("Authorization", "Bearer "+openTok.Plaintext)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("openapi-purpose token at MCP entry: status %d, want 401", resp.StatusCode)
	}

	bothTok, err := tokens.Create(context.Background(), 1, "both", mcptoken.PurposeBoth, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	sess := connect(t, srv.URL, bothTok.Plaintext)
	if _, err := sess.ListTools(context.Background(), nil); err != nil {
		t.Fatalf("both-purpose token at MCP entry: %v", err)
	}
}
