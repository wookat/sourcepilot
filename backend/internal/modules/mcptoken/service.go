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
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/trademind-ai/trademind/backend/internal/modules/auth"
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

// createLocks serializes Create per tenant within this process so the
// count→insert cap check cannot race between concurrent requests. On
// PostgreSQL a transaction-scoped advisory lock extends the same guarantee
// across replicas.
var createLocks sync.Map

func lockTenantCreate(tenantID int64) func() {
	actual, _ := createLocks.LoadOrStore(tenantID, &sync.Mutex{})
	mu := actual.(*sync.Mutex)
	mu.Lock()
	return mu.Unlock
}

func isPostgres(db *gorm.DB) bool {
	return db != nil && db.Dialector != nil && db.Dialector.Name() == "postgres"
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

// ErrInvalidPurpose indicates an unknown token purpose.
var ErrInvalidPurpose = errors.New("mcptoken: invalid purpose (want mcp/openapi/both)")

// Write-scope tokens must expire: default 30 days when unspecified, never
// more than 90 days, never non-expiring. Read-only tokens keep the legacy
// optional expiry so their semantics do not change.
const (
	WriteTokenDefaultExpiryDays = 30
	WriteTokenMaxExpiryDays     = 90
)

// ErrWriteExpiryTooLong rejects write tokens whose requested lifetime
// exceeds WriteTokenMaxExpiryDays (non-expiring write tokens are forbidden).
var ErrWriteExpiryTooLong = errors.New("mcptoken: write:ops token expiry must be 1-90 days (never non-expiring)")

// ErrWritePurposeOpenAPI rejects write scopes on openapi-only tokens: the
// whitelisted write tools exist only on the MCP entry, so an openapi-only
// write grant would be dead surface waiting to widen.
var ErrWritePurposeOpenAPI = errors.New("mcptoken: write:ops requires purpose mcp or both")

// Create issues a new readonly token for the tenant. The plaintext is
// returned once; only its SHA-256 hash is persisted. expiresAt is optional:
// nil issues a non-expiring token, a non-nil value must lie in the future.
// purpose selects the entry the token may call (empty defaults to mcp).
func (s *Service) Create(ctx context.Context, tenantID int64, name string, purpose string, expiresAt *time.Time, createdBy *uuid.UUID) (*CreateResult, error) {
	return s.CreateScoped(ctx, tenantID, name, purpose, nil, expiresAt, createdBy)
}

// CreateScoped issues a token with an explicit scope set. scopes may combine
// readonly and write:ops (empty defaults to readonly; unknown members are
// rejected). Write scope is only grantable here — there is no upgrade path
// for existing tokens — and forces an expiry (default 30 days, max 90).
// Caller-side authorization (admin only for write scope) is enforced at the
// handler; this service enforces the shape invariants.
func (s *Service) CreateScoped(ctx context.Context, tenantID int64, name string, purpose string, scopes []string, expiresAt *time.Time, createdBy *uuid.UUID) (*CreateResult, error) {
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
	purpose = strings.TrimSpace(purpose)
	if purpose == "" {
		purpose = PurposeMCP
	}
	if !ValidPurpose(purpose) {
		return nil, ErrInvalidPurpose
	}
	scope, err := NormalizeScopes(scopes)
	if err != nil {
		return nil, err
	}
	now := time.Now().UTC()
	if expiresAt != nil {
		utc := expiresAt.UTC()
		if !utc.After(now) {
			return nil, ErrInvalidExpiry
		}
		expiresAt = &utc
	}
	if strings.Contains(scope, ScopeWriteOps) {
		if purpose == PurposeOpenAPI {
			return nil, ErrWritePurposeOpenAPI
		}
		if expiresAt == nil {
			def := now.Add(WriteTokenDefaultExpiryDays * 24 * time.Hour)
			expiresAt = &def
		} else if expiresAt.After(now.Add(WriteTokenMaxExpiryDays * 24 * time.Hour)) {
			return nil, ErrWriteExpiryTooLong
		}
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
		Scope:     scope,
		Purpose:   purpose,
		ExpiresAt: expiresAt,
		CreatedBy: createdBy,
	}
	unlock := lockTenantCreate(tenantID)
	defer unlock()
	err = s.DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if isPostgres(tx) {
			// Transaction-scoped advisory lock serializes the cap check across
			// replicas; released automatically at commit/rollback.
			if lerr := tx.Exec("SELECT pg_advisory_xact_lock(hashtext(?)::bigint)",
				fmt.Sprintf("mcptoken_create:%d", tenantID)).Error; lerr != nil {
				return fmt.Errorf("mcptoken: lock: %w", lerr)
			}
		}
		var active int64
		if cerr := tenantquery.ScopeTenant(tx.Model(&Token{}), tenantID).
			Where("revoked_at IS NULL").Count(&active).Error; cerr != nil {
			return fmt.Errorf("mcptoken: count: %w", cerr)
		}
		if active >= MaxActiveTokensPerTenant {
			return ErrTooManyTokens
		}
		if cerr := tx.Create(&row).Error; cerr != nil {
			return fmt.Errorf("mcptoken: create: %w", cerr)
		}
		return nil
	})
	if err != nil {
		return nil, err
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

// Authenticate resolves an active token row from a presented plaintext for
// the MCP entry. It fails closed on malformed, unknown, or revoked tokens.
func (s *Service) Authenticate(ctx context.Context, plain string) (*Token, error) {
	return s.AuthenticateFor(ctx, plain, PurposeMCP)
}

// AuthenticateFor resolves an active token row from a presented plaintext for
// one entry (PurposeMCP or PurposeOpenAPI). Tokens issued for the other entry
// are rejected exactly like unknown tokens so the surfaces stay disjoint.
func (s *Service) AuthenticateFor(ctx context.Context, plain string, entry string) (*Token, error) {
	if s == nil || s.DB == nil {
		return nil, ErrInvalidToken
	}
	plain = strings.TrimSpace(plain)
	if !strings.HasPrefix(plain, TokenPrefix) || len(plain) != len(TokenPrefix)+secretHexLen {
		return nil, ErrInvalidToken
	}
	hash := HashToken(plain)
	q := s.DB.WithContext(ctx).Model(&Token{}).
		Where("token_hash = ? AND revoked_at IS NULL", hash).
		Where("expires_at IS NULL OR expires_at > ?", time.Now().UTC())
	switch entry {
	case PurposeMCP:
		// Empty purpose covers rows written before the column existed.
		q = q.Where("purpose IN ?", []string{PurposeMCP, PurposeBoth, ""})
	case PurposeOpenAPI:
		q = q.Where("purpose IN ?", []string{PurposeOpenAPI, PurposeBoth})
	default:
		return nil, ErrInvalidToken
	}
	var row Token
	err := q.First(&row).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrInvalidToken
		}
		return nil, fmt.Errorf("mcptoken: auth: %w", err)
	}
	if subtle.ConstantTimeCompare([]byte(row.TokenHash), []byte(hash)) != 1 || row.TenantID < 0 || row.Expired(time.Now().UTC()) {
		return nil, ErrInvalidToken
	}
	// Scope-set gate: a token whose scope column parses to no known member
	// authorizes nothing (malformed / future scopes fail closed). The MCP
	// entry accepts readonly (query tools) and write:ops (whitelisted write
	// tools); the Open API entry stays strictly readonly — write scope never
	// widens it.
	switch entry {
	case PurposeMCP:
		if !row.HasScope(ScopeReadonly) && !row.HasScope(ScopeWriteOps) {
			return nil, ErrInvalidToken
		}
	case PurposeOpenAPI:
		if !row.HasScope(ScopeReadonly) {
			return nil, ErrInvalidToken
		}
	}
	// A tenant disabled by a platform administrator loses the token entries too,
	// otherwise a terminated tenant keeps reading its data through MCP / Open API
	// after the admin console has locked it out. An unavailable tenant state
	// fails closed, matching the JWT path.
	if err := auth.EnsureTenantActive(ctx, s.DB, row.TenantID); err != nil {
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
