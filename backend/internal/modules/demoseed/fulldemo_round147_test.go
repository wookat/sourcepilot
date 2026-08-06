package demoseed

import (
	"context"
	"strings"
	"testing"

	"github.com/trademind-ai/trademind/backend/internal/modules/mcptoken"
	"github.com/trademind-ai/trademind/backend/internal/modules/operationlog"
)

// Round 147: the full demo seed issues one masked MCP read-only token plus an
// operation-log audit sample; cleanup removes the token with zero residue and
// never persists or logs the plaintext secret.
func TestFullDemoSeedRound147MCPToken(t *testing.T) {
	db := openFullDemoTestDB(t)
	if db == nil {
		return
	}
	s := &FullDemoSeeder{DB: db, TenantID: 1, AppEnv: "development"}

	if _, err := s.Seed(context.Background()); err != nil {
		t.Fatal(err)
	}

	var tok mcptoken.Token
	if err := db.First(&tok, "name = ?", demoMCPTokenName).Error; err != nil {
		t.Fatalf("expected seeded MCP demo token: %v", err)
	}
	if tok.TenantID != 1 || tok.Scope != mcptoken.ScopeReadonly {
		t.Fatalf("unexpected token state: %+v", tok)
	}
	if !strings.HasPrefix(tok.Prefix, mcptoken.TokenPrefix) || len(tok.LastFour) != 4 {
		t.Fatalf("expected masked prefix/last-four metadata, got prefix=%q lastFour=%q", tok.Prefix, tok.LastFour)
	}
	if len(tok.TokenHash) != 64 || strings.HasPrefix(tok.TokenHash, mcptoken.TokenPrefix) {
		t.Fatalf("expected SHA-256 hash only, got %q", tok.TokenHash)
	}
	if tok.LastUsedAt == nil {
		t.Fatal("expected demo LastUsedAt sample")
	}
	masked := tok.Masked()
	if strings.Contains(masked, tok.TokenHash) {
		t.Fatalf("masked display must not leak hash: %q", masked)
	}

	// Audit sample: one mcp_token_create operation log, masked token only.
	var logs []operationlog.OperationLog
	if err := db.Where("request_id = ?", demoMCPAuditRequestID).Find(&logs).Error; err != nil {
		t.Fatal(err)
	}
	if len(logs) != 1 {
		t.Fatalf("expected exactly 1 MCP audit sample, got %d", len(logs))
	}
	entry := logs[0]
	if entry.Action != "mcp_token_create" || entry.Resource != "mcp_token" || entry.ResourceID != tok.ID.String() {
		t.Fatalf("unexpected audit sample: %+v", entry)
	}
	if !strings.Contains(entry.Message, masked) {
		t.Fatalf("audit message should carry masked token %q, got %q", masked, entry.Message)
	}
	if strings.Contains(entry.Message, tok.TokenHash) {
		t.Fatalf("audit message must not leak token hash: %q", entry.Message)
	}

	// Idempotent re-seed: still exactly 1 token and 1 audit sample.
	if _, err := s.Seed(context.Background()); err != nil {
		t.Fatal(err)
	}
	var tokens int64
	db.Model(&mcptoken.Token{}).Where("name LIKE ?", "DEMO-%").Count(&tokens)
	if tokens != 1 {
		t.Fatalf("expected 1 demo token after re-seed, got %d", tokens)
	}
	var audits int64
	db.Model(&operationlog.OperationLog{}).Where("request_id = ?", demoMCPAuditRequestID).Count(&audits)
	if audits != 1 {
		t.Fatalf("expected 1 audit sample after re-seed, got %d", audits)
	}

	// Cleanup: zero token residue; VerifyClean reports 0 for mcp_api_tokens.
	if _, err := s.Cleanup(context.Background()); err != nil {
		t.Fatal(err)
	}
	db.Model(&mcptoken.Token{}).Unscoped().Where("name LIKE ?", "DEMO-%").Count(&tokens)
	if tokens != 0 {
		t.Fatalf("expected 0 demo tokens after cleanup, got %d", tokens)
	}
	res, err := s.VerifyClean(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if n, ok := res.Counts["mcp_api_tokens"]; !ok || n != 0 {
		t.Fatalf("expected verify mcp_api_tokens=0, got %d (present=%v)", n, ok)
	}
}
