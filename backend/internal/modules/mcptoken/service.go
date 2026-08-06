package mcptoken

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/trademind-ai/trademind/backend/internal/pkg/tenantquery"
	"gorm.io/gorm"
)

// TokenPrefix is the fixed leading marker of every MCP read-only token.
const TokenPrefix = "sp_mcp_ro_"

// secretHexLen is the number of random hex chars after the prefix (32 bytes).
const secretHexLen = 64

// ErrNotFound indicates the token does not exist within the tenant scope.
var ErrNotFound = errors.New("mcp token not found")

// ErrInvalidToken indicates the presented plaintext token is unknown, revoked,
// expired or carries a scope the read-only entry does not accept.
var ErrInvalidToken = errors.New("invalid mcp token")

// ErrInvalidExpiry indicates a requested expiry that is not in the future.
var ErrInvalidExpiry = errors.New("mcptoken: expiry must be in the future")

// MaxActiveTokensPerTenant caps live tokens per tenant. Each token owns its own
// rate-limit bucket, so an unbounded token count would multiply the request
// budget one tenant can consume.
const MaxActiveTokensPerTenant = 20

// ErrTooManyTokens indicates the tenant already holds MaxActiveTokensPerTenant
// active tokens; one must be revoked before issuing another.
var ErrTooManyTokens = errors.New("mcptoken: active token limit reached (revoke an unused token first)")

// Service manages tenant-scoped MCP read-only API tokens.
type Service struct {
	DB *gorm.DB
}

// HashToken returns the hex-encoded SHA-256 of a plaintext token.
func HashToken(plain string) string {
	sum := sha256.Sum256([]byte(plain))
	return hex.EncodeToString(sum[:])
}

// CreateResult carries the one-time plaintext next to the stored row.
type CreateResult struct {
	Token Token
	// Plaintext is returned exactly once at creation and never stored.
	Plaintext string
}

// Create issues a new readonly token for the tenant. The plaintext is
// returned once; only its SHA-256 hash is persisted. expiresAt is optional:
// nil issues a non-expiring token, a non-nil value must lie in the future.
func (s *Service) Create(ctx context.Context, tenantID int64, name string, expiresAt *time.Time, createdBy *uuid.UUID) (*CreateResult, error) {
	if s == nil || s.DB == nil {
		return nil, fmt.Errorf("mcptoken: no db")
	}
	if tenantID < 0 {
		return nil, fmt.Errorf("mcptoken: invalid tenant")
	}
	name = strings.TrimSpace(name)
	if name == "" {
		return nil, fmt.Errorf("mcptoken: name required")
	}
	if len([]rune(name)) > 64 {
		return nil, fmt.Errorf("mcptoken: name too long (max 64)")
	}
	if expiresAt != nil {
		utc := expiresAt.UTC()
		if !utc.After(time.Now().UTC()) {
			return nil, ErrInvalidExpiry
		}
		expiresAt = &utc
	}
	var active int64
	if err := tenantquery.ScopeTenant(s.DB.WithContext(ctx).Model(&Token{}), tenantID).
		Where("revoked_at IS NULL").Count(&active).Error; err != nil {
		return nil, fmt.Errorf("mcptoken: count: %w", err)
	}
	if active >= MaxActiveTokensPerTenant {
		return nil, ErrTooManyTokens
	}
	buf := make([]byte, secretHexLen/2)
	if _, err := rand.Read(buf); err != nil {
		return nil, fmt.Errorf("mcptoken: rand: %w", err)
	}
	plain := TokenPrefix + hex.EncodeToString(buf)
	row := Token{
		TenantID:  tenantID,
		Name:      name,
		Prefix:    plain[:len(TokenPrefix)+4],
		LastFour:  plain[len(plain)-4:],
		TokenHash: HashToken(plain),
		Scope:     ScopeReadonly,
		ExpiresAt: expiresAt,
		CreatedBy: createdBy,
	}
	if err := s.DB.WithContext(ctx).Create(&row).Error; err != nil {
		return nil, fmt.Errorf("mcptoken: create: %w", err)
	}
	return &CreateResult{Token: row, Plaintext: plain}, nil
}

// List returns all tokens of the tenant (masked; hash never leaves the service).
func (s *Service) List(ctx context.Context, tenantID int64) ([]Token, error) {
	if s == nil || s.DB == nil {
		return nil, fmt.Errorf("mcptoken: no db")
	}
	var rows []Token
	q := tenantquery.ScopeTenant(s.DB.WithContext(ctx).Model(&Token{}), tenantID)
	if err := q.Order("created_at DESC").Find(&rows).Error; err != nil {
		return nil, fmt.Errorf("mcptoken: list: %w", err)
	}
	return rows, nil
}

// Revoke marks a token revoked within the tenant scope. Idempotent.
func (s *Service) Revoke(ctx context.Context, tenantID int64, id uuid.UUID) (*Token, error) {
	if s == nil || s.DB == nil {
		return nil, fmt.Errorf("mcptoken: no db")
	}
	var row Token
	q := tenantquery.ScopeTenant(s.DB.WithContext(ctx).Model(&Token{}), tenantID)
	if err := q.Where("id = ?", id).First(&row).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("mcptoken: load: %w", err)
	}
	if row.RevokedAt == nil {
		now := time.Now().UTC()
		if err := s.DB.WithContext(ctx).Model(&Token{}).
			Where("id = ? AND revoked_at IS NULL", row.ID).
			Update("revoked_at", now).Error; err != nil {
			return nil, fmt.Errorf("mcptoken: revoke: %w", err)
		}
		row.RevokedAt = &now
	}
	return &row, nil
}

// Authenticate resolves an active token row from a presented plaintext.
// It fails closed on malformed, unknown, or revoked tokens.
func (s *Service) Authenticate(ctx context.Context, plain string) (*Token, error) {
	if s == nil || s.DB == nil {
		return nil, ErrInvalidToken
	}
	plain = strings.TrimSpace(plain)
	if !strings.HasPrefix(plain, TokenPrefix) || len(plain) != len(TokenPrefix)+secretHexLen {
		return nil, ErrInvalidToken
	}
	hash := HashToken(plain)
	var row Token
	err := s.DB.WithContext(ctx).Model(&Token{}).
		Where("token_hash = ? AND revoked_at IS NULL AND scope = ?", hash, ScopeReadonly).
		Where("expires_at IS NULL OR expires_at > ?", time.Now().UTC()).
		First(&row).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrInvalidToken
		}
		return nil, fmt.Errorf("mcptoken: auth: %w", err)
	}
	if subtle.ConstantTimeCompare([]byte(row.TokenHash), []byte(hash)) != 1 || row.TenantID < 0 || row.Expired(time.Now().UTC()) {
		return nil, ErrInvalidToken
	}
	return &row, nil
}

// TouchLastUsed records usage time (best effort, throttled to 1/min per token).
func (s *Service) TouchLastUsed(ctx context.Context, id uuid.UUID) {
	if s == nil || s.DB == nil || id == uuid.Nil {
		return
	}
	now := time.Now().UTC()
	cutoff := now.Add(-time.Minute)
	_ = s.DB.WithContext(ctx).Model(&Token{}).
		Where("id = ? AND (last_used_at IS NULL OR last_used_at < ?)", id, cutoff).
		Update("last_used_at", now).Error
}
