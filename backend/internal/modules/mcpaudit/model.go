// Package mcpaudit records one audit row per MCP tool call: tenant, token,
// tool name, outcome and timing. For read tools, query parameters and query
// results are never stored. For whitelisted write tools, only allowlisted
// key=value argument/result summaries are stored (business keys like order
// no / tag name — never secrets or free text) so every write stays traceable.
package mcpaudit

import (
	"time"

	"github.com/google/uuid"
	"github.com/trademind-ai/trademind/backend/internal/pkg/id"
	"gorm.io/gorm"
)

// Statuses of one audit row. success/error describe completed tool calls;
// auth_failed / rate_limited record entry-level rejections (401/429) so
// brute-force attempts and throttling events stay visible in the audit table.
// Rows for unauthenticated callers carry tenant 0 (platform scope).
const (
	StatusSuccess     = "success"
	StatusError       = "error"
	StatusAuthFailed  = "auth_failed"
	StatusRateLimited = "rate_limited"
)

// Modes of one write-tool audit row. Read tools leave Mode empty; write
// tools record dry_run / execute so the paired rows for one confirmed write
// stay linkable via ConfirmHash.
const (
	ModeDryRun  = "dry_run"
	ModeExecute = "execute"
)

// ToolCallLog is one immutable MCP tool-call audit row (no soft delete).
type ToolCallLog struct {
	ID       uuid.UUID `gorm:"type:char(36);primaryKey" json:"id"`
	TenantID int64     `gorm:"not null;index" json:"tenantId"`
	TokenID  uuid.UUID `gorm:"type:char(36);index" json:"tokenId"`
	// TokenName / TokenMasked snapshot the token identity at call time so the
	// log stays readable after the token is revoked or renamed.
	TokenName   string `gorm:"size:128" json:"tokenName"`
	TokenMasked string `gorm:"size:40" json:"tokenMasked"`
	Tool        string `gorm:"size:64;index;not null" json:"tool"`
	Status      string `gorm:"size:16;index;not null" json:"status"`
	// Mode is empty for read tools; dry_run / execute for write tools.
	Mode string `gorm:"size:16;index;not null;default:''" json:"mode,omitempty"`
	// ParamsSummary / ResultSummary hold allowlisted key=value pairs for
	// write tools only (e.g. "orderNo=X tag=Y" / "applied=1"). Never secrets.
	ParamsSummary string `gorm:"size:512;not null;default:''" json:"paramsSummary,omitempty"`
	ResultSummary string `gorm:"size:512;not null;default:''" json:"resultSummary,omitempty"`
	// ConfirmHash is the SHA-256 of the confirmation token binding the
	// dry_run row to its execute row. Empty for read tools.
	ConfirmHash string    `gorm:"size:64;index;not null;default:''" json:"confirmHash,omitempty"`
	DurationMs  int64     `gorm:"not null;default:0" json:"durationMs"`
	CreatedAt   time.Time `gorm:"index" json:"createdAt"`
}

// TableName keeps a stable table name for migrations.
func (ToolCallLog) TableName() string { return "mcp_tool_call_logs" }

// BeforeCreate assigns a UUID when id is zero.
func (l *ToolCallLog) BeforeCreate(tx *gorm.DB) error {
	id.Ensure(&l.ID)
	return nil
}
