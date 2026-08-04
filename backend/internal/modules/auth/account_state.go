package auth

import (
	"context"
	"errors"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// accountState is the trusted account snapshot used by request authentication.
type accountState struct {
	Status       string
	TokenVersion int
}

// authStateCacheTTL bounds how long a last-known-good auth snapshot may stand
// in for the database during a transient outage. A revoked account can keep
// its token for at most this window when the database is unreachable.
const authStateCacheTTL = 30 * time.Second

type cachedAccountState struct {
	state accountState
	at    time.Time
}

var accountStateCache sync.Map // uuid.UUID -> cachedAccountState

// EnsureAccountActive validates the account behind an access token on every
// request: the row must still exist, must not be soft deleted, must be active
// and its token_version must match the version captured in the token.
//
// Password resets, role/status changes and account deletion all bump
// token_version, so this check is what makes those operations revoke access
// tokens immediately instead of waiting for the token TTL to expire. Missing
// rows fail closed. Transient database errors fall back to the last snapshot
// read within authStateCacheTTL; with no fresh snapshot the request fails
// closed with ErrAuthStateUnavailable instead of silently passing.
func EnsureAccountActive(ctx context.Context, db *gorm.DB, userID uuid.UUID, tokenVersion int) error {
	_, err := EnsureAccountActiveDetailed(ctx, db, userID, tokenVersion)
	return err
}

// EnsureAccountActiveDetailed reports, in addition to the validation result,
// whether the decision was bridged from the last snapshot because the
// database was unreachable.
func EnsureAccountActiveDetailed(ctx context.Context, db *gorm.DB, userID uuid.UUID, tokenVersion int) (bridged bool, err error) {
	if db == nil || userID == uuid.Nil {
		return false, nil
	}
	var rows []accountState
	if err := db.WithContext(ctx).Table("admin_users").
		Select("status", "token_version").
		Where("id = ? AND deleted_at IS NULL", userID).
		Limit(1).Scan(&rows).Error; err != nil {
		if cached, ok := accountStateCache.Load(userID); ok {
			c := cached.(cachedAccountState)
			if time.Since(c.at) <= authStateCacheTTL {
				return true, evaluateAccountState(c.state, tokenVersion)
			}
		}
		return true, errors.New(ErrAuthStateUnavailable)
	}
	if len(rows) == 0 {
		accountStateCache.Delete(userID)
		return false, errors.New(ErrUserDisabled)
	}
	accountStateCache.Store(userID, cachedAccountState{state: rows[0], at: time.Now()})
	return false, evaluateAccountState(rows[0], tokenVersion)
}

func evaluateAccountState(st accountState, tokenVersion int) error {
	if !strings.EqualFold(strings.TrimSpace(st.Status), "active") {
		return errors.New(ErrUserDisabled)
	}
	if tokenVersion > 0 && st.TokenVersion > tokenVersion {
		return errors.New(ErrSessionRevoked)
	}
	return nil
}
