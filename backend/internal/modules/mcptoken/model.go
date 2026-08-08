package mcptoken

import (
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/trademind-ai/trademind/backend/internal/pkg/model"
)

// Scopes are the privilege axis of a token. Scope is stored as a
// comma-joined set (e.g. "readonly" or "readonly,write:ops"); membership is
// checked per entry / per tool. write:ops never implies read.
const (
	// ScopeReadonly authorizes the read-only query tools (MCP) and the Open
	// API entry. Pre-existing tokens all carry exactly this scope.
	ScopeReadonly = "readonly"
	// ScopeWriteOps authorizes the whitelisted MCP write tools (W1 订单打标
	// 起步). Only grantable at creation time, admin-only, forced expiry.
	ScopeWriteOps = "write:ops"
)

// knownScopes lists every scope value the system understands. Unknown
// members are ignored at authentication time (they grant nothing) and are
// rejected at creation time.
var knownScopes = map[string]bool{ScopeReadonly: true, ScopeWriteOps: true}

// ParseScopes splits a stored scope column into its known members. Unknown
// members grant nothing; an empty result means the token authorizes no entry
// (fail closed).
func ParseScopes(raw string) []string {
	seen := map[string]bool{}
	var out []string
	for _, part := range strings.Split(raw, ",") {
		p := strings.TrimSpace(part)
		if p == "" || !knownScopes[p] || seen[p] {
			continue
		}
		seen[p] = true
		out = append(out, p)
	}
	return out
}

// NormalizeScopes validates a requested scope set for token creation:
// unknown members are rejected (fail closed at grant time), duplicates
// collapse, and an empty set defaults to readonly. The result is the
// canonical stored form (sorted, comma-joined).
func NormalizeScopes(in []string) (string, error) {
	if len(in) == 0 {
		return ScopeReadonly, nil
	}
	seen := map[string]bool{}
	var out []string
	for _, raw := range in {
		p := strings.TrimSpace(raw)
		if p == "" {
			continue
		}
		if !knownScopes[p] {
			return "", fmt.Errorf("%w: %s", ErrInvalidScope, p)
		}
		if !seen[p] {
			seen[p] = true
			out = append(out, p)
		}
	}
	if len(out) == 0 {
		return ScopeReadonly, nil
	}
	sort.Strings(out)
	return strings.Join(out, ","), nil
}

// ErrInvalidScope indicates an unknown scope member in a creation request.
var ErrInvalidScope = errors.New("mcptoken: invalid scope (want readonly / write:ops)")

// Purposes bind a token to one or both read-only entries. Scope stays the
// single privilege axis (readonly); purpose is a surface selector so a token
// issued for one entry cannot be replayed against the other.
const (
	// PurposeMCP restricts the token to the MCP entry (POST /api/mcp).
	PurposeMCP = "mcp"
	// PurposeOpenAPI restricts the token to the Open API entry (/api/open/v1/*).
	PurposeOpenAPI = "openapi"
	// PurposeBoth authorizes both read-only entries.
	PurposeBoth = "both"
)

// ValidPurpose reports whether p is a known token purpose.
func ValidPurpose(p string) bool {
	return p == PurposeMCP || p == PurposeOpenAPI || p == PurposeBoth
}

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
	// Purpose selects which read-only entry accepts the token: mcp, openapi or
	// both. Pre-existing tokens default to mcp so their surface never widens.
	Purpose string `gorm:"size:16;not null;default:mcp" json:"purpose"`
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

// Scopes returns the token's known scope members. An empty or unparseable
// scope column yields an empty set, which authorizes nothing.
func (t Token) Scopes() []string { return ParseScopes(t.Scope) }

// HasScope reports whether the token carries one scope member.
func (t Token) HasScope(scope string) bool {
	for _, s := range t.Scopes() {
		if s == scope {
			return true
		}
	}
	return false
}

// AllowsMCP reports whether the token may call the MCP entry. An empty
// purpose is treated as mcp for rows written before the column existed.
func (t Token) AllowsMCP() bool {
	return t.Purpose == PurposeMCP || t.Purpose == PurposeBoth || t.Purpose == ""
}

// AllowsOpenAPI reports whether the token may call the Open API entry.
func (t Token) AllowsOpenAPI() bool {
	return t.Purpose == PurposeOpenAPI || t.Purpose == PurposeBoth
}
