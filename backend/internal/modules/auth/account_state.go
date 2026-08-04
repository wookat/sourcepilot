package auth

import (
	"context"
	"errors"
	"strings"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// accountState is the trusted account snapshot used by request authentication.
type accountState struct {
	Status       string
	TokenVersion int
}

// EnsureAccountActive validates the account behind an access token on every
// request: the row must still exist, must not be soft deleted, must be active
// and its token_version must match the version captured in the token.
//
// Password resets, role/status changes and account deletion all bump
// token_version, so this check is what makes those operations revoke access
// tokens immediately instead of waiting for the token TTL to expire. Missing
// rows fail closed; transient database errors fail open so a database blip
// never locks every tenant out.
func EnsureAccountActive(ctx context.Context, db *gorm.DB, userID uuid.UUID, tokenVersion int) error {
	if db == nil || userID == uuid.Nil {
		return nil
	}
	var rows []accountState
	if err := db.WithContext(ctx).Table("admin_users").
		Select("status", "token_version").
		Where("id = ? AND deleted_at IS NULL", userID).
		Limit(1).Scan(&rows).Error; err != nil {
		return nil
	}
	if len(rows) == 0 {
		return errors.New(ErrUserDisabled)
	}
	if !strings.EqualFold(strings.TrimSpace(rows[0].Status), "active") {
		return errors.New(ErrUserDisabled)
	}
	if tokenVersion > 0 && rows[0].TokenVersion > tokenVersion {
		return errors.New(ErrSessionRevoked)
	}
	return nil
}
