// Package mcpaudit records one audit row per MCP tool call: tenant, token,
// tool name, outcome and timing. Query parameters and query results are never
// stored, so the log carries no business data.
package mcpaudit

import (
	"time"

	"github.com/google/uuid"
	"github.com/trademind-ai/trademind/backend/internal/pkg/id"
	"gorm.io/gorm"
)

// Statuses of one tool call.
const (
	StatusSuccess = "success"
	StatusError   = "error"
)

// ToolCallLog is one immutable MCP tool-call audit row (no soft delete).
type ToolCallLog struct {
	ID       uuid.UUID `gorm:"type:char(36);primaryKey" json:"id"`
	TenantID int64     `gorm:"not null;index" json:"tenantId"`
	TokenID  uuid.UUID `gorm:"type:char(36);index" json:"tokenId"`
	// TokenName / TokenMasked snapshot the token identity at call time so the
	// log stays readable after the token is revoked or renamed.
	TokenName   string    `gorm:"size:128" json:"tokenName"`
	TokenMasked string    `gorm:"size:40" json:"tokenMasked"`
	Tool        string    `gorm:"size:64;index;not null" json:"tool"`
	Status      string    `gorm:"size:16;index;not null" json:"status"`
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
