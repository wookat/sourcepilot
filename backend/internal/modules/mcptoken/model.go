package mcptoken

import (
	"time"

	"github.com/google/uuid"
	"github.com/trademind-ai/trademind/backend/internal/pkg/model"
)

// ScopeReadonly is the only scope tokens can carry today: MCP tools are
// strictly read-only and tokens never authorize writes.
const ScopeReadonly = "readonly"

// Token is a tenant-scoped API token for the MCP read-only entry. Only the
// SHA-256 hash of the secret is stored; the plaintext is shown once at
// creation time and never persisted or logged.
type Token struct {
	model.HardDeleteBase
	TenantID int64  `gorm:"not null;index" json:"tenantId"`
	Name     string `gorm:"size:128;not null" json:"name"`
	// Prefix is the non-secret leading part of the token kept for masked display.
	Prefix string `gorm:"size:24;not null" json:"prefix"`
	// LastFour is the trailing 4 characters kept for masked display.
	LastFour string `gorm:"size:8;not null" json:"lastFour"`
	// TokenHash is the hex-encoded SHA-256 of the full plaintext token.
	TokenHash string `gorm:"size:64;not null;uniqueIndex" json:"-"`
	Scope     string `gorm:"size:32;not null;default:readonly" json:"scope"`
	// ExpiresAt, when set, is the instant after which the token stops
	// authenticating. NULL means the token never expires.
	ExpiresAt  *time.Time `gorm:"index" json:"expiresAt,omitempty"`
	CreatedBy  *uuid.UUID `gorm:"type:char(36)" json:"createdBy,omitempty"`
	LastUsedAt *time.Time `json:"lastUsedAt,omitempty"`
	RevokedAt  *time.Time `gorm:"index" json:"revokedAt,omitempty"`
}

// TableName maps Token onto mcp_api_tokens.
func (Token) TableName() string { return "mcp_api_tokens" }

// Masked returns the masked display form, e.g. "sp_mcp_ro_ab12…cd34".
func (t Token) Masked() string {
	return t.Prefix + "…" + t.LastFour
}

// Expired reports whether the token has an expiry in the past.
func (t Token) Expired(now time.Time) bool {
	return t.ExpiresAt != nil && !t.ExpiresAt.After(now)
}
