package mcpserver_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/trademind-ai/trademind/backend/internal/modules/mcpaudit"
	"github.com/trademind-ai/trademind/backend/internal/modules/mcpserver"
	"github.com/trademind-ai/trademind/backend/internal/modules/mcptoken"
	"gorm.io/gorm"
)

func newAuditedServer(t *testing.T, db *gorm.DB) (*httptest.Server, *mcptoken.Service, *mcpaudit.Service) {
	t.Helper()
	if err := db.AutoMigrate(&mcpaudit.ToolCallLog{}); err != nil {
		t.Fatal(err)
	}
	gin.SetMode(gin.TestMode)
	tokens := &mcptoken.Service{DB: db}
	audits := &mcpaudit.Service{DB: db}
	r := gin.New()
	r.POST("/api/mcp", mcpserver.GinHandler(&mcpserver.Deps{
		DB:        db,
		Tokens:    tokens,
		Audits:    audits,
		RateRPS:   100,
		RateBurst: 100,
		Version:   "test",
	}))
	srv := httptest.NewServer(r)
	t.Cleanup(srv.Close)
	return srv, tokens, audits
}

func TestToolCallsAreAudited(t *testing.T) {
	db := openTestDB(t)
	seedOrders(t, db)
	srv, tokens, audits := newAuditedServer(t, db)
	res, err := tokens.Create(context.Background(), 1, "audited", "", nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	sess := connect(t, srv.URL, res.Plaintext)
	if _, err := sess.CallTool(context.Background(), &mcp.CallToolParams{
		Name:      "orders_query",
		Arguments: map[string]any{},
	}); err != nil {
		t.Fatal(err)
	}

	logs, err := audits.List(context.Background(), 1, mcpaudit.ListFilter{})
	if err != nil {
		t.Fatal(err)
	}
	if logs.Total != 1 {
		t.Fatalf("expected 1 audit row, got %d", logs.Total)
	}
	row := logs.Items[0]
	if row.Tool != "orders_query" || row.Status != mcpaudit.StatusSuccess {
		t.Fatalf("unexpected audit row: %+v", row)
	}
	if row.TenantID != 1 || row.TokenID != res.Token.ID {
		t.Fatalf("audit row not bound to tenant/token: %+v", row)
	}
	if strings.Contains(row.TokenMasked, res.Plaintext) || row.TokenMasked == "" {
		t.Fatalf("token must be masked in audit: %q", row.TokenMasked)
	}
}

func TestFailedToolCallAuditedAsError(t *testing.T) {
	db := openTestDB(t)
	srv, tokens, audits := newAuditedServer(t, db)
	res, err := tokens.Create(context.Background(), 1, "audited-err", "", nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	sess := connect(t, srv.URL, res.Plaintext)
	// Invalid argument type: the SDK surfaces a tool error result.
	out, err := sess.CallTool(context.Background(), &mcp.CallToolParams{
		Name:      "orders_query",
		Arguments: map[string]any{"pageSize": "not-a-number"},
	})
	if err == nil && (out == nil || !out.IsError) {
		t.Skip("tool accepted invalid arguments; cannot exercise error path")
	}

	logs, err := audits.List(context.Background(), 1, mcpaudit.ListFilter{Status: mcpaudit.StatusError})
	if err != nil {
		t.Fatal(err)
	}
	if logs.Total != 1 {
		t.Fatalf("expected 1 error audit row, got %d", logs.Total)
	}
}

func TestExpiredTokenRejectedAtEntry(t *testing.T) {
	db := openTestDB(t)
	srv, tokens, _ := newAuditedServer(t, db)
	future := time.Now().UTC().Add(time.Hour)
	res, err := tokens.Create(context.Background(), 1, "expiring", "", &future, nil)
	if err != nil {
		t.Fatal(err)
	}
	// Move expiry into the past.
	expired := time.Now().UTC().Add(-time.Minute)
	if err := db.Model(&mcptoken.Token{}).Where("id = ?", res.Token.ID).
		Update("expires_at", expired).Error; err != nil {
		t.Fatal(err)
	}
	req, _ := http.NewRequest(http.MethodPost, srv.URL+"/api/mcp", strings.NewReader("{}"))
	req.Header.Set("Authorization", "Bearer "+res.Plaintext)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("expired token: status %d, want 401", resp.StatusCode)
	}
}
