package mcpserver_test

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"github.com/google/uuid"
	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/trademind-ai/trademind/backend/internal/modules/mcpaudit"
	"github.com/trademind-ai/trademind/backend/internal/modules/mcpserver"
	"github.com/trademind-ai/trademind/backend/internal/modules/mcptoken"
	"github.com/trademind-ai/trademind/backend/internal/modules/mcpwrite"
	"github.com/trademind-ai/trademind/backend/internal/modules/order"
	"github.com/trademind-ai/trademind/backend/internal/modules/settings"
	"gorm.io/gorm"
)

// R179 W1: governed MCP write surface tests — three default-off gates,
// dry-run → one-time confirmation → execute, fail-closed audit, quotas,
// tenant isolation and idempotent order tagging.

func openWriteTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	dsn := fmt.Sprintf("file:mcpwrite_%s?mode=memory&cache=shared&_pragma=busy_timeout(5000)", uuid.New().String())
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Skipf("sqlite unavailable: %v", err)
	}
	sqlDB, _ := db.DB()
	sqlDB.SetMaxOpenConns(1)
	if err := db.AutoMigrate(
		&mcptoken.Token{}, &mcpaudit.ToolCallLog{}, &mcpwrite.Confirmation{},
		&order.Order{}, &order.OrderTag{}, &order.OrderTagLink{}, &settings.Setting{},
	); err != nil {
		t.Fatal(err)
	}
	if err := db.Exec(`CREATE TABLE tenants (id integer primary key, status text, deleted_at datetime)`).Error; err != nil {
		t.Fatal(err)
	}
	return db
}

func newWriteTestServer(t *testing.T, db *gorm.DB, writeEnabled bool) (*httptest.Server, *mcptoken.Service, *mcpaudit.Service) {
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
		WriteEnabled: writeEnabled,
		Orders:       &order.Service{DB: db},
		Settings:     &settings.Service{DB: db},
	}))
	srv := httptest.NewServer(r)
	t.Cleanup(srv.Close)
	return srv, tokens, audits
}

func enableTenantWrite(t *testing.T, db *gorm.DB, tenantID int64) {
	t.Helper()
	if err := db.Create(&settings.Setting{
		TenantID:  tenantID,
		GroupKey:  mcpwrite.SettingsGroupMCP,
		ItemKey:   mcpwrite.SettingsKeyWriteEnable,
		ItemValue: "true",
	}).Error; err != nil {
		t.Fatal(err)
	}
}

func seedTagFixtures(t *testing.T, db *gorm.DB) {
	t.Helper()
	orders := []order.Order{
		{TenantID: 1, OrderNo: "T1-W1", Platform: "douyin", Status: "pending", Currency: "CNY"},
		{TenantID: 2, OrderNo: "T2-W1", Platform: "shopee", Status: "pending", Currency: "USD"},
	}
	for i := range orders {
		if err := db.Create(&orders[i]).Error; err != nil {
			t.Fatal(err)
		}
	}
	tags := []order.OrderTag{
		{TenantID: 1, Name: "加急", Color: "red"},
		{TenantID: 2, Name: "加急", Color: "red"},
	}
	for i := range tags {
		if err := db.Create(&tags[i]).Error; err != nil {
			t.Fatal(err)
		}
	}
}

func newWriteToken(t *testing.T, tokens *mcptoken.Service, tenantID int64) string {
	t.Helper()
	res, err := tokens.CreateScoped(context.Background(), tenantID, "writer", "",
		[]string{mcptoken.ScopeReadonly, mcptoken.ScopeWriteOps}, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	return res.Plaintext
}

func callTagTool(t *testing.T, sess *mcp.ClientSession, tool string, args map[string]any) (*mcp.CallToolResult, *mcpwrite.Result) {
	t.Helper()
	out, err := sess.CallTool(context.Background(), &mcp.CallToolParams{Name: tool, Arguments: args})
	if err != nil {
		t.Fatalf("call %s: %v", tool, err)
	}
	if out.IsError {
		return out, nil
	}
	var res mcpwrite.Result
	raw, err := json.Marshal(out.StructuredContent)
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(raw, &res); err != nil {
		t.Fatal(err)
	}
	return out, &res
}

func toolErrorText(t *testing.T, out *mcp.CallToolResult) string {
	t.Helper()
	if !out.IsError {
		t.Fatal("expected tool error")
	}
	var sb strings.Builder
	for _, c := range out.Content {
		if tc, ok := c.(*mcp.TextContent); ok {
			sb.WriteString(tc.Text)
		}
	}
	return sb.String()
}

// Gate 1 (env, default off): write tools are not exposed at all and readonly
// surface is unchanged.
func TestWriteToolsHiddenWhenEnvGateClosed(t *testing.T) {
	db := openWriteTestDB(t)
	seedTagFixtures(t, db)
	enableTenantWrite(t, db, 1)
	srv, tokens, _ := newWriteTestServer(t, db, false)
	sess := connect(t, srv.URL, newWriteToken(t, tokens, 1))
	toolsRes, err := sess.ListTools(context.Background(), nil)
	if err != nil {
		t.Fatal(err)
	}
	for _, tool := range toolsRes.Tools {
		if tool.Name == "orders_add_tag" || tool.Name == "orders_remove_tag" {
			t.Fatalf("write tool %s exposed with MCP_WRITE_ENABLED=false", tool.Name)
		}
	}
	if _, err := sess.CallTool(context.Background(), &mcp.CallToolParams{
		Name: "orders_add_tag", Arguments: map[string]any{"orderNo": "T1-W1", "tagName": "加急", "mode": "dry_run"},
	}); err == nil {
		t.Fatal("write tool callable with env gate closed")
	}
}

// Gate 2 (tenant, default off): tool visible for write:ops token but every
// call is rejected until the tenant opts in.
func TestTenantGateClosedRejects(t *testing.T) {
	db := openWriteTestDB(t)
	seedTagFixtures(t, db)
	srv, tokens, _ := newWriteTestServer(t, db, true)
	sess := connect(t, srv.URL, newWriteToken(t, tokens, 1))
	out, _ := callTagTool(t, sess, "orders_add_tag", map[string]any{"orderNo": "T1-W1", "tagName": "加急", "mode": "dry_run"})
	if msg := toolErrorText(t, out); !strings.Contains(msg, "租户级开关") {
		t.Fatalf("tenant-gate message = %q", msg)
	}
}

// Gate 3 (scope): readonly tokens never see the write tools; write-only
// tokens never see the read tools.
func TestScopeGateSeparatesSurfaces(t *testing.T) {
	db := openWriteTestDB(t)
	seedTagFixtures(t, db)
	enableTenantWrite(t, db, 1)
	srv, tokens, _ := newWriteTestServer(t, db, true)

	ro, err := tokens.Create(context.Background(), 1, "ro", "", nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	roSess := connect(t, srv.URL, ro.Plaintext)
	roTools, err := roSess.ListTools(context.Background(), nil)
	if err != nil {
		t.Fatal(err)
	}
	for _, tool := range roTools.Tools {
		if strings.HasPrefix(tool.Name, "orders_add") || strings.HasPrefix(tool.Name, "orders_remove") {
			t.Fatalf("readonly token sees write tool %s", tool.Name)
		}
	}
	if _, err := roSess.CallTool(context.Background(), &mcp.CallToolParams{
		Name: "orders_add_tag", Arguments: map[string]any{"orderNo": "T1-W1", "tagName": "加急", "mode": "dry_run"},
	}); err == nil {
		t.Fatal("readonly token could call a write tool")
	}

	wo, err := tokens.CreateScoped(context.Background(), 1, "wo", "", []string{mcptoken.ScopeWriteOps}, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	woSess := connect(t, srv.URL, wo.Plaintext)
	woTools, err := woSess.ListTools(context.Background(), nil)
	if err != nil {
		t.Fatal(err)
	}
	for _, tool := range woTools.Tools {
		if !strings.HasPrefix(tool.Name, "orders_add_tag") && !strings.HasPrefix(tool.Name, "orders_remove_tag") {
			t.Fatalf("write-only token sees read tool %s", tool.Name)
		}
	}
	if _, err := woSess.CallTool(context.Background(), &mcp.CallToolParams{
		Name: "orders_query", Arguments: map[string]any{},
	}); err == nil {
		t.Fatal("write-only token could call a read tool")
	}
}

// Happy path: dry_run returns preview + confirmation; execute applies the
// tag; a second add round-trip is an idempotent no-op; remove detaches.
func TestDryRunConfirmExecuteIdempotent(t *testing.T) {
	db := openWriteTestDB(t)
	seedTagFixtures(t, db)
	enableTenantWrite(t, db, 1)
	srv, tokens, _ := newWriteTestServer(t, db, true)
	sess := connect(t, srv.URL, newWriteToken(t, tokens, 1))
	args := map[string]any{"orderNo": "T1-W1", "tagName": "加急"}

	_, dry := callTagTool(t, sess, "orders_add_tag", map[string]any{"orderNo": "T1-W1", "tagName": "加急", "mode": "dry_run"})
	if dry == nil || dry.ConfirmationToken == "" || dry.Mode != "dry_run" {
		t.Fatalf("dry run result = %+v", dry)
	}
	preview, _ := json.Marshal(dry.Preview)
	if !strings.Contains(string(preview), `"change":"add"`) {
		t.Fatalf("preview = %s", preview)
	}

	_, exec1 := callTagTool(t, sess, "orders_add_tag", map[string]any{
		"orderNo": args["orderNo"], "tagName": args["tagName"], "mode": "execute", "confirmationToken": dry.ConfirmationToken,
	})
	if exec1 == nil || exec1.Mode != "execute" {
		t.Fatalf("execute result = %+v", exec1)
	}
	res1, _ := json.Marshal(exec1.Result)
	if !strings.Contains(string(res1), `"applied":1`) {
		t.Fatalf("execute result = %s", res1)
	}

	// Idempotent second add: fresh dry_run previews "none", execute applies 0.
	_, dry2 := callTagTool(t, sess, "orders_add_tag", map[string]any{"orderNo": "T1-W1", "tagName": "加急", "mode": "dry_run"})
	p2, _ := json.Marshal(dry2.Preview)
	if !strings.Contains(string(p2), `"change":"none"`) {
		t.Fatalf("second preview = %s", p2)
	}
	_, exec2 := callTagTool(t, sess, "orders_add_tag", map[string]any{
		"orderNo": "T1-W1", "tagName": "加急", "mode": "execute", "confirmationToken": dry2.ConfirmationToken,
	})
	r2, _ := json.Marshal(exec2.Result)
	if strings.Contains(string(r2), `"applied":1`) {
		t.Fatalf("idempotent re-add applied a link: %s", r2)
	}

	// Remove.
	_, dry3 := callTagTool(t, sess, "orders_remove_tag", map[string]any{"orderNo": "T1-W1", "tagName": "加急", "mode": "dry_run"})
	_, exec3 := callTagTool(t, sess, "orders_remove_tag", map[string]any{
		"orderNo": "T1-W1", "tagName": "加急", "mode": "execute", "confirmationToken": dry3.ConfirmationToken,
	})
	r3, _ := json.Marshal(exec3.Result)
	if !strings.Contains(string(r3), `"removed":1`) {
		t.Fatalf("remove result = %s", r3)
	}
	var links int64
	if err := db.Model(&order.OrderTagLink{}).Count(&links).Error; err != nil {
		t.Fatal(err)
	}
	if links != 0 {
		t.Fatalf("links = %d after remove", links)
	}
}

// Execute without / with a bad confirmation, reuse after execute, expiry,
// caller binding and parameter drift are all rejected.
func TestConfirmationHardening(t *testing.T) {
	db := openWriteTestDB(t)
	seedTagFixtures(t, db)
	enableTenantWrite(t, db, 1)
	srv, tokens, _ := newWriteTestServer(t, db, true)
	sess := connect(t, srv.URL, newWriteToken(t, tokens, 1))

	// Missing confirmation.
	out, _ := callTagTool(t, sess, "orders_add_tag", map[string]any{"orderNo": "T1-W1", "tagName": "加急", "mode": "execute"})
	if msg := toolErrorText(t, out); !strings.Contains(msg, "confirmationToken") {
		t.Fatalf("missing-confirmation message = %q", msg)
	}
	// Unknown confirmation.
	out, _ = callTagTool(t, sess, "orders_add_tag", map[string]any{
		"orderNo": "T1-W1", "tagName": "加急", "mode": "execute", "confirmationToken": "sp_mcp_confirm_bogus",
	})
	if msg := toolErrorText(t, out); !strings.Contains(msg, "无效") {
		t.Fatalf("bogus-confirmation message = %q", msg)
	}

	// Parameter drift: confirmation for T1-W1 used with a different tag op.
	_, dry := callTagTool(t, sess, "orders_add_tag", map[string]any{"orderNo": "T1-W1", "tagName": "加急", "mode": "dry_run"})
	out, _ = callTagTool(t, sess, "orders_remove_tag", map[string]any{
		"orderNo": "T1-W1", "tagName": "加急", "mode": "execute", "confirmationToken": dry.ConfirmationToken,
	})
	if msg := toolErrorText(t, out); !strings.Contains(msg, "无效") {
		t.Fatalf("drift message = %q", msg)
	}

	// Caller binding: another write token of the same tenant cannot spend it.
	sess2 := connect(t, srv.URL, newWriteToken(t, tokens, 1))
	out, _ = callTagTool(t, sess2, "orders_add_tag", map[string]any{
		"orderNo": "T1-W1", "tagName": "加急", "mode": "execute", "confirmationToken": dry.ConfirmationToken,
	})
	if msg := toolErrorText(t, out); !strings.Contains(msg, "无效") {
		t.Fatalf("cross-caller message = %q", msg)
	}

	// Expiry.
	if err := db.Model(&mcpwrite.Confirmation{}).Where("consumed_at IS NULL").
		Update("expires_at", time.Now().UTC().Add(-time.Minute)).Error; err != nil {
		t.Fatal(err)
	}
	out, _ = callTagTool(t, sess, "orders_add_tag", map[string]any{
		"orderNo": "T1-W1", "tagName": "加急", "mode": "execute", "confirmationToken": dry.ConfirmationToken,
	})
	if msg := toolErrorText(t, out); !strings.Contains(msg, "过期") {
		t.Fatalf("expiry message = %q", msg)
	}

	// Reuse after successful execute answers already_executed (no re-mutation).
	_, dry2 := callTagTool(t, sess, "orders_add_tag", map[string]any{"orderNo": "T1-W1", "tagName": "加急", "mode": "dry_run"})
	callTagTool(t, sess, "orders_add_tag", map[string]any{
		"orderNo": "T1-W1", "tagName": "加急", "mode": "execute", "confirmationToken": dry2.ConfirmationToken,
	})
	_, replay := callTagTool(t, sess, "orders_add_tag", map[string]any{
		"orderNo": "T1-W1", "tagName": "加急", "mode": "execute", "confirmationToken": dry2.ConfirmationToken,
	})
	if replay == nil || !replay.AlreadyExecuted {
		t.Fatalf("replay result = %+v", replay)
	}
	var links int64
	if err := db.Model(&order.OrderTagLink{}).Count(&links).Error; err != nil {
		t.Fatal(err)
	}
	if links != 1 {
		t.Fatalf("links = %d, want 1 (replay must not re-mutate)", links)
	}
}

// Cross-tenant targets resolve to not-found (404 semantics, no oracle).
func TestCrossTenantTargetNotFound(t *testing.T) {
	db := openWriteTestDB(t)
	seedTagFixtures(t, db)
	enableTenantWrite(t, db, 1)
	srv, tokens, _ := newWriteTestServer(t, db, true)
	sess := connect(t, srv.URL, newWriteToken(t, tokens, 1))
	out, _ := callTagTool(t, sess, "orders_add_tag", map[string]any{"orderNo": "T2-W1", "tagName": "加急", "mode": "dry_run"})
	if msg := toolErrorText(t, out); !strings.Contains(msg, "订单不存在") {
		t.Fatalf("cross-tenant message = %q", msg)
	}
}

// Fail-closed audit: dry_run and execute leave paired rows; a broken audit
// table rejects the execute and rolls the mutation back.
func TestWriteAuditFailClosed(t *testing.T) {
	db := openWriteTestDB(t)
	seedTagFixtures(t, db)
	enableTenantWrite(t, db, 1)
	srv, tokens, _ := newWriteTestServer(t, db, true)
	sess := connect(t, srv.URL, newWriteToken(t, tokens, 1))

	_, dry := callTagTool(t, sess, "orders_add_tag", map[string]any{"orderNo": "T1-W1", "tagName": "加急", "mode": "dry_run"})
	var dryRow mcpaudit.ToolCallLog
	if err := db.First(&dryRow, "tool = ? AND mode = ?", "orders_add_tag", "dry_run").Error; err != nil {
		t.Fatalf("dry_run audit row missing: %v", err)
	}
	if dryRow.ConfirmHash == "" || !strings.Contains(dryRow.ParamsSummary, "orderNo=T1-W1") {
		t.Fatalf("dry_run audit row = %+v", dryRow)
	}
	if strings.Contains(dryRow.ParamsSummary, dry.ConfirmationToken) || strings.Contains(dryRow.ResultSummary, dry.ConfirmationToken) {
		t.Fatal("confirmation plaintext leaked into audit row")
	}

	// Break the audit table: execute must fail and the tag must not stick.
	if err := db.Migrator().DropTable(&mcpaudit.ToolCallLog{}); err != nil {
		t.Fatal(err)
	}
	out, _ := callTagTool(t, sess, "orders_add_tag", map[string]any{
		"orderNo": "T1-W1", "tagName": "加急", "mode": "execute", "confirmationToken": dry.ConfirmationToken,
	})
	if !out.IsError {
		t.Fatal("execute succeeded with audit unavailable")
	}
	var links int64
	if err := db.Model(&order.OrderTagLink{}).Count(&links).Error; err != nil {
		t.Fatal(err)
	}
	if links != 0 {
		t.Fatalf("links = %d, want 0 (mutation must roll back with audit)", links)
	}
}

// Quotas: an exhausted per-token hourly budget rejects new writes; the
// per-tenant daily budget is enforced independently.
func TestWriteQuotasFailClosed(t *testing.T) {
	db := openWriteTestDB(t)
	seedTagFixtures(t, db)
	enableTenantWrite(t, db, 1)
	srv, tokens, audits := newWriteTestServer(t, db, true)
	plain := newWriteToken(t, tokens, 1)
	tok, err := tokens.Authenticate(context.Background(), plain)
	if err != nil {
		t.Fatal(err)
	}
	sess := connect(t, srv.URL, plain)

	// Saturate the per-token hourly budget with synthetic execute rows.
	for i := 0; i < mcpwrite.PerTokenHourlyLimit; i++ {
		if err := audits.Write(context.Background(), mcpaudit.WriteOpts{
			TenantID: 1, TokenID: tok.ID, Tool: "orders_add_tag",
			Status: mcpaudit.StatusSuccess, Mode: mcpaudit.ModeExecute,
		}); err != nil {
			t.Fatal(err)
		}
	}
	out, _ := callTagTool(t, sess, "orders_add_tag", map[string]any{"orderNo": "T1-W1", "tagName": "加急", "mode": "dry_run"})
	if msg := toolErrorText(t, out); !strings.Contains(msg, "上限") {
		t.Fatalf("token quota message = %q", msg)
	}

	// A different token of the same tenant still works (per-token budget)...
	sess2 := connect(t, srv.URL, newWriteToken(t, tokens, 1))
	_, dry := callTagTool(t, sess2, "orders_add_tag", map[string]any{"orderNo": "T1-W1", "tagName": "加急", "mode": "dry_run"})
	if dry == nil || dry.ConfirmationToken == "" {
		t.Fatalf("second token dry_run = %+v", dry)
	}
	// ...until the tenant daily budget is saturated too.
	for i := int64(0); i < mcpwrite.PerTenantDailyLimit; i++ {
		if err := audits.Write(context.Background(), mcpaudit.WriteOpts{
			TenantID: 1, TokenID: uuid.New(), Tool: "orders_add_tag",
			Status: mcpaudit.StatusSuccess, Mode: mcpaudit.ModeExecute,
		}); err != nil {
			t.Fatal(err)
		}
	}
	out, _ = callTagTool(t, sess2, "orders_add_tag", map[string]any{"orderNo": "T1-W1", "tagName": "加急", "mode": "dry_run"})
	if msg := toolErrorText(t, out); !strings.Contains(msg, "租户今日") {
		t.Fatalf("tenant quota message = %q", msg)
	}
}

// The MCP surface exposes no message-line / external-send tool under any
// gate combination (绝不自动外发 red line).
func TestNoExternalSendToolExposed(t *testing.T) {
	db := openWriteTestDB(t)
	enableTenantWrite(t, db, 1)
	srv, tokens, _ := newWriteTestServer(t, db, true)
	sess := connect(t, srv.URL, newWriteToken(t, tokens, 1))
	toolsRes, err := sess.ListTools(context.Background(), nil)
	if err != nil {
		t.Fatal(err)
	}
	allowed := map[string]bool{
		"orders_query": true, "inventory_query": true, "report_summary": true,
		"exceptions_pending": true, "orders_add_tag": true, "orders_remove_tag": true,
	}
	for _, tool := range toolsRes.Tools {
		if !allowed[tool.Name] {
			t.Fatalf("unexpected tool exposed: %s", tool.Name)
		}
		lower := strings.ToLower(tool.Name)
		for _, banned := range []string{"message", "send", "reply", "chat", "publish"} {
			if strings.Contains(lower, banned) {
				t.Fatalf("message/external tool exposed: %s", tool.Name)
			}
		}
	}
}
