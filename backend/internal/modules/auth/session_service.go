package auth

import (
	"context"
	"crypto/subtle"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/trademind-ai/trademind/backend/internal/config"
	"github.com/trademind-ai/trademind/backend/internal/modules/admin"
	"github.com/trademind-ai/trademind/backend/internal/pkg/authutil"
	"github.com/trademind-ai/trademind/backend/internal/pkg/id"
	"github.com/trademind-ai/trademind/backend/internal/pkg/metrics"
	"github.com/trademind-ai/trademind/backend/internal/pkg/p7diag"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// SessionService manages sessions and refresh token rotation.
type SessionService struct {
	Cfg     *config.Config
	DB      *gorm.DB
	Admins  *admin.Store
	Metrics *metrics.Catalog
}

// LoginSessionResult is returned after successful credential verification.
type LoginSessionResult struct {
	AccessToken  string
	AccessExp    time.Time
	RefreshToken string
	SessionID    uuid.UUID
	TenantID     int64
	User         userView
}

// CreateSession authenticates and creates session + refresh token family.
func (s *SessionService) CreateSession(ctx context.Context, account, password, ip, userAgent string) (*LoginSessionResult, error) {
	if s == nil || s.Admins == nil || s.Cfg == nil || s.DB == nil {
		return nil, fmt.Errorf("auth: misconfigured")
	}
	guard := &LoginGuard{Cfg: s.Cfg, DB: s.DB}
	stageStart := time.Now()
	if err := guard.CheckAllowed(ctx, account, ip); err != nil {
		p7diag.ObserveStage(p7diag.RouteAuthInvalidLogin, "rate_limit_check", authOutcome(err), stageStart)
		p7diag.ObserveStage(p7diag.RouteAuthInvalidLogin, "lockout_evaluate", authOutcome(err), stageStart)
		if err.Error() == ErrAccountTemporarilyLocked {
			p7diag.Path(p7diag.RouteAuthInvalidLogin, "locked_account")
			p7diag.Path(p7diag.RouteAuthInvalidLogin, p7diag.PathLockedAccount)
			p7diag.Count(p7diag.RouteAuthInvalidLogin, "lockedAccountQueryCount", 1)
		} else if err.Error() == ErrTooManyAttempts {
			p7diag.Path(p7diag.RouteAuthInvalidLogin, "rate_limited")
		}
		return nil, err
	}
	p7diag.ObserveStage(p7diag.RouteAuthInvalidLogin, "rate_limit_check", p7diag.OutcomeSuccess, stageStart)
	p7diag.ObserveStage(p7diag.RouteAuthInvalidLogin, "lockout_evaluate", p7diag.OutcomeSuccess, stageStart)

	stageStart = time.Now()
	var u *admin.AdminUser
	timing, err := p7diag.TimedGorm(s.DB, func() error {
		var lookupErr error
		u, lookupErr = s.Admins.ByLoginAccount(ctx, account)
		return lookupErr
	})
	outcome := authOutcome(err)
	p7diag.ObserveStage(p7diag.RouteAuthInvalidLogin, "account_lookup", outcome, stageStart)
	p7diag.ObserveDBOperation(p7diag.RouteAuthInvalidLogin, "account_lookup", outcome, stageStart)
	p7diag.ObserveSQL(p7diag.RouteAuthInvalidLogin, "auth", "auth.account_lookup", "select", "admin_users", outcome, false, timing)
	p7diag.Count(p7diag.RouteAuthInvalidLogin, "accountLookupCount", 1)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			p7diag.Path(p7diag.RouteAuthInvalidLogin, "account_missing")
			p7diag.Path(p7diag.RouteAuthInvalidLogin, p7diag.PathUnknownAccount)
			p7diag.Count(p7diag.RouteAuthInvalidLogin, "unknownAccountQueryCount", 1)
			p7diag.ObservePasswordVerify(p7diag.PathUnknownAccount, p7diag.PasswordAlgoBcrypt, bcrypt.DefaultCost, 0, time.Now())
			_ = guard.RecordFailure(ctx, account, ip)
			return nil, errors.New(ErrInvalidCredentials)
		}
		return nil, err
	}
	stageStart = time.Now()
	cost := bcrypt.DefaultCost
	if c, cerr := bcrypt.Cost([]byte(u.PasswordHash)); cerr == nil {
		cost = c
	}
	if err := admin.CheckPassword(u.PasswordHash, password); err != nil {
		p7diag.ObserveStage(p7diag.RouteAuthInvalidLogin, "password_verify", p7diag.OutcomeExpectedRejection, stageStart)
		p7diag.ObservePasswordVerify(p7diag.PathKnownWrongPassword, p7diag.PasswordAlgoBcrypt, cost, 1, stageStart)
		p7diag.Count(p7diag.RouteAuthInvalidLogin, "passwordVerifyCount", 1)
		p7diag.Count(p7diag.RouteAuthInvalidLogin, "wrongPasswordQueryCount", 1)
		p7diag.Path(p7diag.RouteAuthInvalidLogin, "wrong_password")
		p7diag.Path(p7diag.RouteAuthInvalidLogin, p7diag.PathKnownWrongPassword)
		_ = guard.RecordFailure(ctx, account, ip)
		return nil, withAuditTenant(u.TenantID, errors.New(ErrInvalidCredentials))
	}
	p7diag.ObserveStage(p7diag.RouteAuthInvalidLogin, "password_verify", p7diag.OutcomeSuccess, stageStart)
	p7diag.ObservePasswordVerify(p7diag.PathSuccessVerify, p7diag.PasswordAlgoBcrypt, cost, 1, stageStart)
	p7diag.Count(p7diag.RouteAuthInvalidLogin, "passwordVerifyCount", 1)
	if st := strings.TrimSpace(strings.ToLower(u.Status)); st == "disabled" || st == "inactive" {
		return nil, withAuditTenant(u.TenantID, errors.New(ErrUserDisabled))
	}
	if tenantDisabled(ctx, s.DB, u.TenantID) {
		return nil, withAuditTenant(u.TenantID, errors.New(ErrTenantDisabled))
	}
	if u.MustChangePassword {
		return nil, withAuditTenant(u.TenantID, errors.New(ErrPasswordChangeRequired))
	}
	_ = guard.ClearFailures(ctx, account, ip)

	now := time.Now().UTC()
	session := &AuthSession{
		TenantID:         u.TenantID,
		UserID:           u.ID,
		Status:           SessionStatusActive,
		BrowserSummary:   authutil.SummarizeUserAgent(userAgent),
		DeviceSummary:    "web",
		IPHash:           authutil.HashIP(ip),
		UserAgentSummary: authutil.SummarizeUserAgent(userAgent),
		LastActivityAt:   now,
		TokenVersion:     u.TokenVersion,
	}
	// 先分配会话 ID，保证访问令牌 session_id 声明与落库会话一致（会话吊销依赖该绑定）
	id.Ensure(&session.ID)
	familyID := uuid.New()
	refreshRaw, err := authutil.NewOpaqueToken(32)
	if err != nil {
		return nil, err
	}
	refreshTTL := s.Cfg.RefreshTokenTTL()
	refresh := &AuthRefreshToken{
		TenantID:         u.TenantID,
		UserID:           u.ID,
		SessionID:        session.ID,
		TokenFamilyID:    familyID,
		TokenHash:        authutil.HashToken(refreshRaw, s.Cfg.JWTSecret),
		Status:           RefreshStatusActive,
		ExpiresAt:        now.Add(refreshTTL),
		IPHash:           authutil.HashIP(ip),
		UserAgentSummary: authutil.SummarizeUserAgent(userAgent),
	}

	ks, err := BuildKeySet(s.Cfg)
	if err != nil {
		return nil, err
	}
	access, exp, err := MintAccessToken(s.Cfg, ks, MintAccessInput{
		UserID:       u.ID,
		Username:     u.LoginLabel(),
		TenantID:     u.TenantID,
		SessionID:    session.ID,
		TokenVersion: u.TokenVersion,
	})
	if err != nil {
		return nil, err
	}

	err = s.DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(session).Error; err != nil {
			return err
		}
		refresh.SessionID = session.ID
		return tx.Create(refresh).Error
	})
	if err != nil {
		return nil, err
	}
	s.ObserveAuth("session_created", "success", "created", "password")

	label := u.LoginLabel()
	dn := u.DisplayName
	if dn == "" {
		dn = label
	}
	return &LoginSessionResult{
		AccessToken:  access,
		AccessExp:    exp,
		RefreshToken: refreshRaw,
		SessionID:    session.ID,
		TenantID:     u.TenantID,
		User: userView{
			ID:          u.ID.String(),
			Username:    label,
			Email:       u.Email,
			Phone:       u.Phone,
			DisplayName: dn,
		},
	}, nil
}

// RefreshResult holds new tokens after rotation.
type RefreshResult struct {
	AccessToken  string
	AccessExp    time.Time
	RefreshToken string
}

// RotateRefresh exchanges a refresh token for a new pair (rotation + reuse detection).
func (s *SessionService) RotateRefresh(ctx context.Context, refreshRaw, ip, userAgent string) (*RefreshResult, error) {
	if s == nil || s.Cfg == nil || s.DB == nil {
		return nil, fmt.Errorf("auth: misconfigured")
	}
	refreshRaw = strings.TrimSpace(refreshRaw)
	if refreshRaw == "" {
		return nil, errors.New(ErrRefreshTokenRevoked)
	}
	hash := authutil.HashToken(refreshRaw, s.Cfg.JWTSecret)
	now := time.Now().UTC()

	var result *RefreshResult
	staleVersionSession := uuid.Nil
	err := s.DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var row AuthRefreshToken
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
			Where("token_hash = ?", hash).First(&row).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return errors.New(ErrRefreshTokenRevoked)
			}
			return err
		}

		switch row.Status {
		case RefreshStatusActive:
			// proceed
		case RefreshStatusRotated:
			_ = s.revokeTokenFamilyTx(tx, row.TokenFamilyID, RefreshStatusCompromised, "reuse_detected")
			return errors.New(ErrRefreshTokenReused)
		case RefreshStatusRevoked, RefreshStatusCompromised, RefreshStatusReuseDetected:
			return errors.New(ErrRefreshTokenRevoked)
		default:
			return errors.New(ErrRefreshTokenRevoked)
		}
		if now.After(row.ExpiresAt) {
			_ = tx.Model(&row).Updates(map[string]any{"status": RefreshStatusExpired}).Error
			return errors.New(ErrRefreshTokenExpired)
		}

		var sess AuthSession
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
			First(&sess, "id = ?", row.SessionID).Error; err != nil {
			return errors.New(ErrSessionRevoked)
		}
		if sess.Status != SessionStatusActive {
			return errors.New(ErrSessionRevoked)
		}

		var u admin.AdminUser
		if err := tx.Select("id", "tenant_id", "role", "status", "token_version", "email", "phone", "display_name").
			First(&u, "id = ?", row.UserID).Error; err != nil {
			return errors.New(ErrSessionRevoked)
		}
		if st := strings.TrimSpace(strings.ToLower(u.Status)); st == "disabled" || st == "inactive" {
			_ = s.revokeSessionTx(tx, sess.ID, "user_disabled")
			return errors.New(ErrUserDisabled)
		}
		if tenantDisabled(ctx, tx, u.TenantID) {
			_ = s.revokeSessionTx(tx, sess.ID, "tenant_disabled")
			return errors.New(ErrTenantDisabled)
		}
		// 与访问令牌 ValidateSessionAccess 同口径：token_version 不匹配的会话拒绝续期，
		// 使删用户/改密码/改角色/租户停用等失效类操作不依赖逐处吊销也能兜底
		if sess.TokenVersion > 0 && u.TokenVersion > 0 && sess.TokenVersion != u.TokenVersion {
			staleVersionSession = sess.ID
			return errors.New(ErrSessionRevoked)
		}

		newRaw, err := authutil.NewOpaqueToken(32)
		if err != nil {
			return err
		}
		newID := uuid.New()
		newRow := &AuthRefreshToken{
			ID:               newID,
			TenantID:         row.TenantID,
			UserID:           row.UserID,
			SessionID:        row.SessionID,
			TokenFamilyID:    row.TokenFamilyID,
			TokenHash:        authutil.HashToken(newRaw, s.Cfg.JWTSecret),
			ParentTokenID:    &row.ID,
			Status:           RefreshStatusActive,
			ExpiresAt:        now.Add(s.Cfg.RefreshTokenTTL()),
			IPHash:           authutil.HashIP(ip),
			UserAgentSummary: authutil.SummarizeUserAgent(userAgent),
		}
		lastUsed := now
		if err := tx.Model(&row).Updates(map[string]any{
			"status":               RefreshStatusRotated,
			"replaced_by_token_id": newID,
			"last_used_at":         &lastUsed,
		}).Error; err != nil {
			return err
		}
		if err := tx.Create(newRow).Error; err != nil {
			return err
		}
		if err := tx.Model(&sess).Updates(map[string]any{
			"last_activity_at":   now,
			"ip_hash":            authutil.HashIP(ip),
			"user_agent_summary": authutil.SummarizeUserAgent(userAgent),
		}).Error; err != nil {
			return err
		}

		ks, err := BuildKeySet(s.Cfg)
		if err != nil {
			return err
		}
		access, exp, err := MintAccessToken(s.Cfg, ks, MintAccessInput{
			UserID:       u.ID,
			Username:     u.LoginLabel(),
			TenantID:     u.TenantID,
			SessionID:    sess.ID,
			TokenVersion: u.TokenVersion,
		})
		if err != nil {
			return err
		}
		result = &RefreshResult{
			AccessToken:  access,
			AccessExp:    exp,
			RefreshToken: newRaw,
		}
		return nil
	})
	if err != nil {
		// 回滚后再持久化吊销，避免失效会话反复尝试续期
		if staleVersionSession != uuid.Nil {
			_ = s.DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
				return s.revokeSessionTx(tx, staleVersionSession, "token_version_mismatch")
			})
		}
		reason := classifyAuthReason(err)
		s.ObserveAuth("refresh", "failure", reason, "refresh_token")
		if err.Error() == ErrRefreshTokenReused {
			s.ObserveAuth("refresh_reuse", "failure", "reuse_detected", "refresh_token")
		}
		return nil, err
	}
	s.ObserveAuth("refresh", "success", "success", "refresh_token")
	return result, nil
}

func (s *SessionService) revokeTokenFamilyTx(tx *gorm.DB, familyID uuid.UUID, status, reason string) error {
	now := time.Now().UTC()
	return tx.Model(&AuthRefreshToken{}).
		Where("token_family_id = ? AND status IN ?", familyID, []string{RefreshStatusActive, RefreshStatusRotated}).
		Updates(map[string]any{
			"status":        status,
			"revoked_at":    &now,
			"revoke_reason": reason,
		}).Error
}

func (s *SessionService) revokeSessionTx(tx *gorm.DB, sessionID uuid.UUID, reason string) error {
	now := time.Now().UTC()
	if err := tx.Model(&AuthSession{}).Where("id = ?", sessionID).Updates(map[string]any{
		"status":        SessionStatusRevoked,
		"revoked_at":    &now,
		"revoke_reason": reason,
	}).Error; err != nil {
		return err
	}
	return tx.Model(&AuthRefreshToken{}).
		Where("session_id = ? AND status = ?", sessionID, RefreshStatusActive).
		Updates(map[string]any{
			"status":        RefreshStatusRevoked,
			"revoked_at":    &now,
			"revoke_reason": reason,
		}).Error
}

// RevokeSession revokes one session and its refresh tokens.
func (s *SessionService) RevokeSession(ctx context.Context, sessionID, userID uuid.UUID, reason string) error {
	if s == nil || s.DB == nil {
		return fmt.Errorf("auth: misconfigured")
	}
	err := s.DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var sess AuthSession
		if err := tx.First(&sess, "id = ?", sessionID).Error; err != nil {
			return err
		}
		if sess.UserID != userID {
			return gorm.ErrRecordNotFound
		}
		return s.revokeSessionTx(tx, sessionID, reason)
	})
	if err == nil {
		s.ObserveAuth("session_revoked", "success", safeAuthReason(reason), "password")
	}
	return err
}

// RevokeOtherSessions revokes all sessions except current.
func (s *SessionService) RevokeOtherSessions(ctx context.Context, userID, currentSessionID uuid.UUID) (int64, error) {
	if s == nil || s.DB == nil {
		return 0, fmt.Errorf("auth: misconfigured")
	}
	var count int64
	err := s.DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var sessions []AuthSession
		if err := tx.Where("user_id = ? AND status = ? AND id <> ?", userID, SessionStatusActive, currentSessionID).
			Find(&sessions).Error; err != nil {
			return err
		}
		for _, sess := range sessions {
			if err := s.revokeSessionTx(tx, sess.ID, "revoke_others"); err != nil {
				return err
			}
			count++
		}
		return nil
	})
	return count, err
}

// RevokeAllUserSessions revokes every session for a user (admin action).
func (s *SessionService) RevokeAllUserSessions(ctx context.Context, userID uuid.UUID, reason string) (int64, error) {
	if s == nil || s.DB == nil {
		return 0, fmt.Errorf("auth: misconfigured")
	}
	var count int64
	err := s.DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var sessions []AuthSession
		if err := tx.Where("user_id = ? AND status = ?", userID, SessionStatusActive).Find(&sessions).Error; err != nil {
			return err
		}
		for _, sess := range sessions {
			if err := s.revokeSessionTx(tx, sess.ID, reason); err != nil {
				return err
			}
			count++
		}
		return nil
	})
	return count, err
}

// ListSessions returns active/recent sessions for a user.
func (s *SessionService) ListSessions(ctx context.Context, userID uuid.UUID) ([]AuthSession, error) {
	if s == nil || s.DB == nil {
		return nil, fmt.Errorf("auth: misconfigured")
	}
	var rows []AuthSession
	err := s.DB.WithContext(ctx).
		Where("user_id = ?", userID).
		Order("last_activity_at DESC").
		Limit(50).
		Find(&rows).Error
	return rows, err
}

// sessionAccessSnapshot is the last-known-good result of a full session
// validation, usable for at most authStateCacheTTL when the database is
// unreachable.
type sessionAccessSnapshot struct {
	userID       uuid.UUID
	tokenVersion int
	at           time.Time
}

var sessionStateCache sync.Map // uuid.UUID -> sessionAccessSnapshot

// ValidateSessionAccess checks session is active for access token binding.
// A missing or revoked session fails closed with ErrSessionRevoked. Transient
// database errors fall back to the last successful validation within
// authStateCacheTTL; with no fresh snapshot the request fails closed with
// ErrAuthStateUnavailable instead of masquerading as a revoked session.
func (s *SessionService) ValidateSessionAccess(ctx context.Context, sessionID uuid.UUID, userID uuid.UUID, tokenVersion int) error {
	if sessionID == uuid.Nil {
		return nil
	}
	if s == nil || s.DB == nil {
		return fmt.Errorf("auth: misconfigured")
	}
	var sess AuthSession
	if err := s.DB.WithContext(ctx).Select("id", "status", "user_id").First(&sess, "id = ?", sessionID).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			sessionStateCache.Delete(sessionID)
			return errors.New(ErrSessionRevoked)
		}
		return validateSessionFromSnapshot(sessionID, userID, tokenVersion)
	}
	if sess.Status != SessionStatusActive || sess.UserID != userID {
		sessionStateCache.Delete(sessionID)
		return errors.New(ErrSessionRevoked)
	}
	var u admin.AdminUser
	if err := s.DB.WithContext(ctx).Select("token_version", "status", "tenant_id").First(&u, "id = ?", userID).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			sessionStateCache.Delete(sessionID)
			return errors.New(ErrSessionRevoked)
		}
		return validateSessionFromSnapshot(sessionID, userID, tokenVersion)
	}
	if st := strings.TrimSpace(strings.ToLower(u.Status)); st == "disabled" || st == "inactive" {
		sessionStateCache.Delete(sessionID)
		return errors.New(ErrUserDisabled)
	}
	disabled, err := tenantState(ctx, s.DB, u.TenantID)
	if err != nil {
		return validateSessionFromSnapshot(sessionID, userID, tokenVersion)
	}
	if disabled {
		sessionStateCache.Delete(sessionID)
		return errors.New(ErrTenantDisabled)
	}
	if tokenVersion > 0 && u.TokenVersion > 0 && tokenVersion != u.TokenVersion {
		sessionStateCache.Delete(sessionID)
		return errors.New(ErrSessionRevoked)
	}
	sessionStateCache.Store(sessionID, sessionAccessSnapshot{userID: userID, tokenVersion: u.TokenVersion, at: time.Now()})
	return nil
}

func validateSessionFromSnapshot(sessionID uuid.UUID, userID uuid.UUID, tokenVersion int) error {
	cached, ok := sessionStateCache.Load(sessionID)
	if !ok {
		return errors.New(ErrAuthStateUnavailable)
	}
	snap := cached.(sessionAccessSnapshot)
	if time.Since(snap.at) > authStateCacheTTL {
		return errors.New(ErrAuthStateUnavailable)
	}
	if snap.userID != userID {
		return errors.New(ErrSessionRevoked)
	}
	if tokenVersion > 0 && snap.tokenVersion > 0 && tokenVersion != snap.tokenVersion {
		return errors.New(ErrSessionRevoked)
	}
	return nil
}

// RevokeByRefreshToken revokes session linked to refresh token (logout).
func (s *SessionService) RevokeByRefreshToken(ctx context.Context, refreshRaw string) error {
	if s == nil || s.DB == nil || s.Cfg == nil {
		return fmt.Errorf("auth: misconfigured")
	}
	hash := authutil.HashToken(strings.TrimSpace(refreshRaw), s.Cfg.JWTSecret)
	var row AuthRefreshToken
	if err := s.DB.WithContext(ctx).Where("token_hash = ?", hash).First(&row).Error; err != nil {
		return nil
	}
	return s.DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		return s.revokeSessionTx(tx, row.SessionID, "logout")
	})
}

// ConstantTimeCompare wraps subtle.ConstantTimeCompare for tests.
func ConstantTimeCompare(a, b string) bool {
	return subtle.ConstantTimeCompare([]byte(a), []byte(b)) == 1
}
