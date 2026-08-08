// Package mcpwrite implements the governed MCP write pipeline (R179 W1 基建):
// three default-off gates (env / tenant / token scope), a mandatory
// dry-run → one-time confirmation token → execute state machine, fail-closed
// per-call audit and per-token / per-tenant write quotas. Message-line /
// external-platform sends are permanently outside this surface (绝不自动外发).
package mcpwrite

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/trademind-ai/trademind/backend/internal/pkg/id"
	"gorm.io/gorm"
)

// ConfirmationTTL is the lifetime of one confirmation token: long enough for
// a human to read the dry-run preview and confirm, short enough that a
// leaked token is useless minutes later.
const ConfirmationTTL = 5 * time.Minute

// confirmPrefix marks confirmation token plaintexts (never stored).
const confirmPrefix = "sp_mcp_confirm_"

// Confirmation is one issued dry-run confirmation. Only the SHA-256 of the
// plaintext is stored. It binds tenant + caller token + tool + params hash,
// is single-use (ConsumedAt set atomically) and expires after
// ConfirmationTTL.
type Confirmation struct {
	ID         uuid.UUID `gorm:"type:char(36);primaryKey" json:"id"`
	TenantID   int64     `gorm:"not null;index" json:"tenantId"`
	TokenID    uuid.UUID `gorm:"type:char(36);not null;index" json:"tokenId"`
	Tool       string    `gorm:"size:64;not null" json:"tool"`
	ParamsHash string    `gorm:"size:64;not null" json:"paramsHash"`
	// ConfirmHash is the hex SHA-256 of the plaintext confirmation token.
	ConfirmHash string     `gorm:"size:64;not null;uniqueIndex" json:"-"`
	ExpiresAt   time.Time  `gorm:"not null;index" json:"expiresAt"`
	ConsumedAt  *time.Time `json:"consumedAt,omitempty"`
	// ExecutedAt is set when the confirmed execute committed successfully, so
	// a replayed execute can answer already_executed instead of re-mutating.
	ExecutedAt *time.Time `json:"executedAt,omitempty"`
	CreatedAt  time.Time  `json:"createdAt"`
}

// TableName keeps a stable table name for migrations.
func (Confirmation) TableName() string { return "mcp_write_confirmations" }

// BeforeCreate assigns a UUID when id is zero.
func (c *Confirmation) BeforeCreate(tx *gorm.DB) error {
	id.Ensure(&c.ID)
	return nil
}

// Confirmation consumption rejections. All map to a refused execute; the
// caller must run a fresh dry_run.
var (
	ErrConfirmationRequired = errors.New("mcp write: execute 需要 confirmationToken（先以 mode=dry_run 调用获取）")
	ErrConfirmationInvalid  = errors.New("mcp write: confirmationToken 无效或与本次调用（租户/token/工具/参数）不匹配，请重新 dry_run")
	ErrConfirmationExpired  = errors.New("mcp write: confirmationToken 已过期，请重新 dry_run")
	ErrConfirmationConsumed = errors.New("mcp write: confirmationToken 已被使用，请重新 dry_run")
)

// hashConfirm returns the hex SHA-256 of a confirmation plaintext.
func hashConfirm(plain string) string {
	sum := sha256.Sum256([]byte(plain))
	return hex.EncodeToString(sum[:])
}

// issueConfirmation persists a new confirmation on the given handle and
// returns the plaintext exactly once.
func issueConfirmation(db *gorm.DB, tenantID int64, tokenID uuid.UUID, tool, paramsHash string) (string, *Confirmation, error) {
	buf := make([]byte, 32)
	if _, err := rand.Read(buf); err != nil {
		return "", nil, fmt.Errorf("mcpwrite: rand: %w", err)
	}
	plain := confirmPrefix + hex.EncodeToString(buf)
	row := Confirmation{
		TenantID:    tenantID,
		TokenID:     tokenID,
		Tool:        tool,
		ParamsHash:  paramsHash,
		ConfirmHash: hashConfirm(plain),
		ExpiresAt:   time.Now().UTC().Add(ConfirmationTTL),
	}
	if err := db.Create(&row).Error; err != nil {
		return "", nil, fmt.Errorf("mcpwrite: issue confirmation: %w", err)
	}
	return plain, &row, nil
}

// consumeOutcome distinguishes the states a consume attempt can land in.
type consumeOutcome int

const (
	consumeOK consumeOutcome = iota
	consumeAlreadyExecuted
)

// consumeConfirmation atomically consumes one confirmation, enforcing every
// binding in the UPDATE's WHERE clause: hash, tenant, caller token, tool,
// params hash, unconsumed, unexpired. A mismatch on any axis leaves the row
// untouched and is then classified for a precise (but non-oracular) error.
func consumeConfirmation(ctx context.Context, db *gorm.DB, tenantID int64, tokenID uuid.UUID, tool, paramsHash, plain string) (consumeOutcome, *Confirmation, error) {
	now := time.Now().UTC()
	h := hashConfirm(plain)
	res := db.WithContext(ctx).Model(&Confirmation{}).
		Where("confirm_hash = ? AND tenant_id = ? AND token_id = ? AND tool = ? AND params_hash = ?",
			h, tenantID, tokenID, tool, paramsHash).
		Where("consumed_at IS NULL AND expires_at > ?", now).
		Update("consumed_at", now)
	if res.Error != nil {
		return 0, nil, fmt.Errorf("mcpwrite: consume confirmation: %w", res.Error)
	}
	if res.RowsAffected == 1 {
		var row Confirmation
		if err := db.WithContext(ctx).First(&row, "confirm_hash = ?", h).Error; err != nil {
			return 0, nil, fmt.Errorf("mcpwrite: load confirmation: %w", err)
		}
		return consumeOK, &row, nil
	}
	// Nothing consumed: classify without leaking other tenants' state.
	var row Confirmation
	err := db.WithContext(ctx).
		First(&row, "confirm_hash = ? AND tenant_id = ? AND token_id = ?", h, tenantID, tokenID).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			// Unknown token, cross-tenant or different caller: identical error.
			return 0, nil, ErrConfirmationInvalid
		}
		return 0, nil, fmt.Errorf("mcpwrite: classify confirmation: %w", err)
	}
	switch {
	case row.Tool != tool || row.ParamsHash != paramsHash:
		return 0, nil, ErrConfirmationInvalid
	case row.ConsumedAt != nil && row.ExecutedAt != nil:
		return consumeAlreadyExecuted, &row, nil
	case row.ConsumedAt != nil:
		return 0, nil, ErrConfirmationConsumed
	case !row.ExpiresAt.After(now):
		return 0, nil, ErrConfirmationExpired
	default:
		return 0, nil, ErrConfirmationInvalid
	}
}
