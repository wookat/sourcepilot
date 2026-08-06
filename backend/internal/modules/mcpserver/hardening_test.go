package mcpserver_test

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/trademind-ai/trademind/backend/internal/modules/mcptoken"
)

func rpc(t *testing.T, url, token, body string) (int, string) {
	t.Helper()
	req, err := http.NewRequest(http.MethodPost, url+"/api/mcp", strings.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json, text/event-stream")
	req.Header.Set("MCP-Protocol-Version", "2025-06-18")
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	body2, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatal(err)
	}
	return resp.StatusCode, string(body2)
}

// Rejected credentials are bounded per client IP: the per-token bucket can only
// apply after a token resolves, so invalid tokens must be limited separately.
func TestInvalidTokenAttemptsAreRateLimited(t *testing.T) {
	srv, _ := newTestServer(t, openTestDB(t), 100, 100)
	bad := mcptoken.TokenPrefix + strings.Repeat("0", 64)
	saw429 := false
	for i := 0; i < 40; i++ {
		st, body := rpc(t, srv.URL, bad, `{"jsonrpc":"2.0","id":1,"method":"tools/list"}`)
		if st == http.StatusTooManyRequests {
			saw429 = true
			if !strings.Contains(body, "42901") {
				t.Fatalf("429 envelope should carry CodeTooManyRequests: %s", body)
			}
			break
		}
		if st != http.StatusUnauthorized {
			t.Fatalf("attempt %d: status %d, want 401", i, st)
		}
	}
	if !saw429 {
		t.Fatal("invalid-token attempts were never rate limited")
	}
}

// A valid token must not be throttled by the authentication-failure budget.
func TestValidTokenNotChargedByAuthFailureBudget(t *testing.T) {
	srv, tokens := newTestServer(t, openTestDB(t), 100, 100)
	res, err := tokens.Create(context.Background(), 1, "valid", nil)
	if err != nil {
		t.Fatal(err)
	}
	bad := mcptoken.TokenPrefix + strings.Repeat("1", 64)
	for i := 0; i < 5; i++ {
		if st, _ := rpc(t, srv.URL, bad, `{"jsonrpc":"2.0","id":1,"method":"tools/list"}`); st != http.StatusUnauthorized {
			t.Fatalf("bad attempt %d: status %d, want 401", i, st)
		}
	}
	for i := 0; i < 20; i++ {
		st, body := rpc(t, srv.URL, res.Plaintext, `{"jsonrpc":"2.0","id":1,"method":"tools/list"}`)
		if st != http.StatusOK {
			t.Fatalf("valid request %d: status %d body=%s", i, st, body)
		}
		var env struct {
			Result json.RawMessage `json:"result"`
		}
		if err := json.Unmarshal([]byte(body), &env); err != nil || len(env.Result) == 0 {
			t.Fatalf("valid request %d: unexpected body %s", i, body)
		}
	}
}

// A tenant holding several tokens must not multiply its aggregate budget: the
// tenant-level bucket caps total throughput regardless of token count.
func TestTenantBucketCapsMultiTokenTraffic(t *testing.T) {
	db := openTestDB(t)
	srv, tokens := newTestServer(t, db, 2, 2)
	var plaintexts []string
	for i := 0; i < 4; i++ {
		res, err := tokens.Create(context.Background(), 1, "tok", nil)
		if err != nil {
			t.Fatal(err)
		}
		plaintexts = append(plaintexts, res.Plaintext)
	}
	allowed := 0
	for round := 0; round < 4; round++ {
		for _, p := range plaintexts {
			if st, _ := rpc(t, srv.URL, p, `{"jsonrpc":"2.0","id":1,"method":"tools/list"}`); st == http.StatusOK {
				allowed++
			}
		}
	}
	// Per-token burst alone would allow 4 tokens x 2 = 8 immediate requests;
	// the tenant bucket (2 x tenantRateFactor) must cut that down.
	if allowed > 6 {
		t.Fatalf("tenant bucket did not cap multi-token traffic: %d requests allowed", allowed)
	}
	if allowed == 0 {
		t.Fatal("tenant bucket blocked all traffic")
	}
}
