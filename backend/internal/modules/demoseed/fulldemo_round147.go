package demoseed

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"time"

	"gorm.io/gorm"

	"github.com/trademind-ai/trademind/backend/internal/modules/mcptoken"
	"github.com/trademind-ai/trademind/backend/internal/modules/operationlog"
)

// demoMCPTokenName is the DEMO- prefixed display name of the seeded MCP
// read-only token, so Cleanup / VerifyClean can target it by prefix without
// ever touching real tokens.
const demoMCPTokenName = "DEMO-MCP 演示只读 token"

// demoMCPAuditRequestID marks the seeded MCP audit sample so repeated seed
// runs stay idempotent (operation logs are append-only and never cleaned).
const demoMCPAuditRequestID = "DEMO-MCP-TOKEN-AUDIT"

// seedRound147MCPToken issues one DEMO MCP read-only token sample plus a
// matching operation-log audit row, so the MCP token management page and the
// operation-log audit trail are demonstrable out of the box. The plaintext is
// generated, hashed and immediately discarded — only the masked prefix / last
// four and the SHA-256 hash are persisted, same as the real create flow.
//
// The audit row lives in the append-only operation-log hash chain, so Cleanup
// intentionally keeps it (hard-deleting a chained row would break audit
// integrity verification); the token row itself is cleaned with zero residue.
func (s *FullDemoSeeder) seedRound147MCPToken(tx *gorm.DB, res *FullDemoResult, now time.Time) error {
	if !tx.Migrator().HasTable("mcp_api_tokens") {
		return nil
	}
	count := func(table string, n int64) { res.Counts[table] += n }

	buf := make([]byte, 32)
	if _, err := rand.Read(buf); err != nil {
		return fmt.Errorf("demoseed: mcp token rand: %w", err)
	}
	plain := mcptoken.TokenPrefix + hex.EncodeToString(buf)
	lastUsed := now.Add(-90 * time.Minute)
	row := mcptoken.Token{
		TenantID:   s.TenantID,
		Name:       demoMCPTokenName,
		Prefix:     plain[:len(mcptoken.TokenPrefix)+4],
		LastFour:   plain[len(plain)-4:],
		TokenHash:  mcptoken.HashToken(plain),
		Scope:      mcptoken.ScopeReadonly,
		LastUsedAt: &lastUsed,
	}
	if err := tx.Create(&row).Error; err != nil {
		return fmt.Errorf("demoseed: mcp token: %w", err)
	}
	count("mcp_api_tokens", 1)

	if !tx.Migrator().HasTable("operation_logs") {
		return nil
	}
	var existing int64
	if err := tx.Model(&operationlog.OperationLog{}).
		Where("tenant_id = ? AND request_id = ?", s.TenantID, demoMCPAuditRequestID).
		Count(&existing).Error; err != nil {
		return fmt.Errorf("demoseed: mcp audit lookup: %w", err)
	}
	if existing > 0 {
		return nil
	}
	opLog := &operationlog.Service{DB: tx}
	if err := opLog.WriteBackground(context.Background(), operationlog.WriteOpts{
		TenantID:   s.TenantID,
		Username:   "demo_admin@trademind.local",
		AdminRole:  "admin",
		Action:     "mcp_token_create",
		Resource:   "mcp_token",
		ResourceID: row.ID.String(),
		RequestID:  demoMCPAuditRequestID,
		Status:     "success",
		Message:    "DEMO- 演示：创建 MCP 只读 token：" + demoMCPTokenName + "（" + row.Masked() + "）",
	}); err != nil {
		return fmt.Errorf("demoseed: mcp audit sample: %w", err)
	}
	return nil
}
