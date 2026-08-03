package auth

import (
	"bytes"
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"github.com/google/uuid"
	"github.com/trademind-ai/trademind/backend/internal/config"
	"github.com/trademind-ai/trademind/backend/internal/modules/admin"
	"github.com/trademind-ai/trademind/backend/internal/modules/operationlog"
	"github.com/trademind-ai/trademind/backend/internal/pkg/authutil"
	"github.com/trademind-ai/trademind/backend/internal/pkg/model"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

func TestRefreshTokenConcurrentRotation(t *testing.T) {
	testID := uuid.NewString()
	db, err := gorm.Open(sqlite.Open(fmt.Sprintf("file:%s?mode=memory&cache=shared", testID)), &gorm.Config{})
	if err != nil {
		t.Skipf("sqlite unavailable: %v", err)
	}
	if err := db.AutoMigrate(&AuthSession{}, &AuthRefreshToken{}, &AuthLoginAttempt{}, &admin.AdminUser{}); err != nil {
		t.Fatal(err)
	}
	hash, _ := bcrypt.GenerateFromPassword([]byte("test-password-123"), bcrypt.DefaultCost)
	uid := uuid.New()
	email := fmt.Sprintf("test-%s@example.com", testID)
	if err := db.Create(&admin.AdminUser{
		Base:         model.Base{ID: uid},
		Username:     "testuser-" + testID,
		Email:        email,
		PasswordHash: string(hash),
		Role:         "admin",
		Status:       "active",
	}).Error; err != nil {
		t.Fatal(err)
	}
	cfg := &config.Config{
		JWTSecret: "test-jwt-secret-with-enough-length-32",
		Auth: config.AuthConfig{
			SessionMode:           config.AuthSessionModeSecure,
			AccessTokenTTLMinutes: 15,
			RefreshTokenTTLDays:   7,
		},
	}
	svc := &SessionService{Cfg: cfg, DB: db, Admins: &admin.Store{DB: db}}
	res, err := svc.CreateSession(context.Background(), email, "test-password-123", "127.0.0.1", "test")
	if err != nil {
		t.Fatal(err)
	}
	raw := res.RefreshToken
	var okCount int32
	var wg sync.WaitGroup
	for i := 0; i < 20; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, err := svc.RotateRefresh(context.Background(), raw, "127.0.0.1", "test")
			if err == nil {
				atomic.AddInt32(&okCount, 1)
			}
		}()
	}
	wg.Wait()
	if okCount != 1 {
		t.Fatalf("expected exactly 1 successful rotation, got %d", okCount)
	}
	var active int64
	db.Model(&AuthRefreshToken{}).Where("status = ?", RefreshStatusActive).Count(&active)
	if active != 1 {
		t.Fatalf("expected 1 active refresh token, got %d", active)
	}
}

func TestHashTokenStable(t *testing.T) {
	a := authutil.HashToken("abc", "pepper")
	b := authutil.HashToken("abc", "pepper")
	if a != b {
		t.Fatal("hash not stable")
	}
}

func TestLoginGuardLockout(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(fmt.Sprintf("file:%s?mode=memory&cache=shared", uuid.NewString())), &gorm.Config{})
	if err != nil {
		t.Skipf("sqlite unavailable: %v", err)
	}
	if err := db.AutoMigrate(&AuthLoginAttempt{}); err != nil {
		t.Fatal(err)
	}
	cfg := &config.Config{
		Auth: config.AuthConfig{
			LoginMaxAttempts:   3,
			LoginWindowMinutes: 15,
			AccountLockMinutes: 30,
		},
	}
	g := &LoginGuard{Cfg: cfg, DB: db}
	for i := 0; i < 3; i++ {
		_ = g.RecordFailure(context.Background(), "user@example.com", "1.2.3.4")
	}
	if err := g.CheckAllowed(context.Background(), "user@example.com", "1.2.3.4"); err == nil {
		t.Fatal("expected lockout")
	}
	_ = g.ClearFailures(context.Background(), "user@example.com", "1.2.3.4")
	if err := g.CheckAllowed(context.Background(), "user@example.com", "1.2.3.4"); err != nil {
		t.Fatal("expected unlocked after clear")
	}
}

type authQueryBudget struct {
	logger.Interface
	accountLookupCount                  int
	failedAttemptReadCount              int
	failedAttemptWriteCount             int
	operationLogPreviousHashLookupCount int
	operationLogInsertCount             int
}

func (l *authQueryBudget) LogMode(level logger.LogLevel) logger.Interface { return l }

func (l *authQueryBudget) Trace(ctx context.Context, begin time.Time, fc func() (string, int64), err error) {
	sql, _ := fc()
	s := strings.ToLower(sql)
	switch {
	case strings.Contains(s, "select") && containsTable(s, "admin_users"):
		l.accountLookupCount++
	case strings.Contains(s, "select") && containsTable(s, "auth_login_attempts"):
		l.failedAttemptReadCount++
	case strings.Contains(s, "insert into") && strings.Contains(s, "auth_login_attempts"),
		strings.Contains(s, "update") && strings.Contains(s, "auth_login_attempts"):
		l.failedAttemptWriteCount++
	case strings.Contains(s, "select") && containsTable(s, "operation_logs") && strings.Contains(s, "chain_partition"):
		l.operationLogPreviousHashLookupCount++
	case strings.Contains(s, "insert into") && strings.Contains(s, "operation_logs"):
		l.operationLogInsertCount++
	}
}

func containsTable(sql, table string) bool {
	return strings.Contains(sql, "from `"+table+"`") || strings.Contains(sql, `from "`+table+`"`) || strings.Contains(sql, "from "+table)
}

func newAuthHandlerForBudget(t testing.TB) (*Handler, *gorm.DB, *authQueryBudget) {
	t.Helper()
	budget := &authQueryBudget{Interface: logger.Default.LogMode(logger.Silent)}
	db, err := gorm.Open(sqlite.Open(fmt.Sprintf("file:%s?mode=memory&cache=shared", uuid.NewString())), &gorm.Config{Logger: budget})
	if err != nil {
		t.Skipf("sqlite unavailable: %v", err)
	}
	if err := db.AutoMigrate(&AuthSession{}, &AuthRefreshToken{}, &AuthLoginAttempt{}, &admin.AdminUser{}, &operationlog.OperationLog{}); err != nil {
		t.Fatal(err)
	}
	cfg := &config.Config{
		JWTSecret: "test-jwt-secret-with-enough-length-32",
		Auth: config.AuthConfig{
			SessionMode:           config.AuthSessionModeSecure,
			AccessTokenTTLMinutes: 15,
			RefreshTokenTTLDays:   7,
			LoginMaxAttempts:      5,
			LoginWindowMinutes:    15,
			AccountLockMinutes:    30,
		},
	}
	sessions := &SessionService{Cfg: cfg, DB: db, Admins: &admin.Store{DB: db}}
	h := &Handler{
		LoginSvc: &LoginService{Cfg: cfg, Admins: &admin.Store{DB: db}, Sessions: sessions},
		Sessions: sessions,
		Admins:   &admin.Store{DB: db},
		OpLog:    &operationlog.Service{DB: db},
		DB:       db,
		Cfg:      cfg,
	}
	return h, db, budget
}

func seedAuthUser(t testing.TB, db *gorm.DB, email, password string) {
	t.Helper()
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.MinCost)
	if err != nil {
		t.Fatal(err)
	}
	if err := db.Create(&admin.AdminUser{
		Base:         model.Base{ID: uuid.New()},
		Username:     admin.NewInternalUsername(),
		Email:        email,
		PasswordHash: string(hash),
		Role:         "admin",
		Status:       "active",
	}).Error; err != nil {
		t.Fatal(err)
	}
}

func performLogin(h *Handler, account, password string) *httptest.ResponseRecorder {
	gin.SetMode(gin.TestMode)
	body := fmt.Sprintf(`{"account":%q,"password":%q}`, account, password)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	req := httptest.NewRequest("POST", "/api/v1/auth/login", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	req.RemoteAddr = "127.0.0.1:12345"
	c.Request = req
	h.Login(c)
	return w
}

func TestInvalidLoginQueryBudgetAndNoEnumeration(t *testing.T) {
	unknownH, _, unknownBudget := newAuthHandlerForBudget(t)
	unknown := performLogin(unknownH, "missing@example.com", "wrong-password")
	if unknown.Code != 401 {
		t.Fatalf("unknown account status = %d, want 401", unknown.Code)
	}
	if unknownBudget.accountLookupCount != 1 || unknownBudget.operationLogPreviousHashLookupCount != 1 || unknownBudget.operationLogInsertCount != 1 {
		t.Fatalf("unknown budget = %+v", unknownBudget)
	}

	wrongH, wrongDB, wrongBudget := newAuthHandlerForBudget(t)
	seedAuthUser(t, wrongDB, "known@example.com", "correct-password")
	wrong := performLogin(wrongH, "known@example.com", "wrong-password")
	if wrong.Code != 401 {
		t.Fatalf("wrong password status = %d, want 401", wrong.Code)
	}
	if unknown.Body.String() != wrong.Body.String() {
		t.Fatalf("unknown and wrong-password responses differ:\nunknown=%s\nwrong=%s", unknown.Body.String(), wrong.Body.String())
	}
	if wrongBudget.accountLookupCount != 1 || wrongBudget.operationLogPreviousHashLookupCount != 1 || wrongBudget.operationLogInsertCount != 1 {
		t.Fatalf("wrong-password budget = %+v", wrongBudget)
	}

	lockedH, lockedDB, lockedBudget := newAuthHandlerForBudget(t)
	lockUntil := time.Now().UTC().Add(time.Hour)
	if err := lockedDB.Create(&AuthLoginAttempt{
		AccountKey:  "locked@example.com",
		FailedCount: 5,
		LockedUntil: &lockUntil,
	}).Error; err != nil {
		t.Fatal(err)
	}
	locked := performLogin(lockedH, "locked@example.com", "any-password")
	if locked.Code != 401 {
		t.Fatalf("locked account status = %d, want existing 401 contract", locked.Code)
	}
	if lockedBudget.accountLookupCount != 0 || lockedBudget.operationLogPreviousHashLookupCount != 1 || lockedBudget.operationLogInsertCount != 1 {
		t.Fatalf("locked budget = %+v", lockedBudget)
	}
}

func BenchmarkInvalidLoginUnknownAccount(b *testing.B) {
	h, _, _ := newAuthHandlerForBudget(b)
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		performLogin(h, "missing@example.com", "wrong-password")
	}
}

func BenchmarkInvalidLoginWrongPassword(b *testing.B) {
	h, db, _ := newAuthHandlerForBudget(b)
	seedAuthUser(b, db, "bench@example.com", "correct-password")
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		performLogin(h, "bench@example.com", "wrong-password")
	}
}

func BenchmarkInvalidLoginWithAudit(b *testing.B) {
	h, db, _ := newAuthHandlerForBudget(b)
	seedAuthUser(b, db, "audit@example.com", "correct-password")
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		performLogin(h, "audit@example.com", "wrong-password")
	}
}

func TestCreateSessionBindsAccessTokenSessionID(t *testing.T) {
	testID := uuid.NewString()
	db, err := gorm.Open(sqlite.Open(fmt.Sprintf("file:%s?mode=memory&cache=shared", testID)), &gorm.Config{})
	if err != nil {
		t.Skipf("sqlite unavailable: %v", err)
	}
	if err := db.AutoMigrate(&AuthSession{}, &AuthRefreshToken{}, &AuthLoginAttempt{}, &admin.AdminUser{}); err != nil {
		t.Fatal(err)
	}
	hash, _ := bcrypt.GenerateFromPassword([]byte("test-password-123"), bcrypt.DefaultCost)
	uid := uuid.New()
	email := fmt.Sprintf("test-%s@example.com", testID)
	if err := db.Create(&admin.AdminUser{
		Base:         model.Base{ID: uid},
		Username:     "testuser-" + testID,
		Email:        email,
		PasswordHash: string(hash),
		Role:         "admin",
		Status:       "active",
	}).Error; err != nil {
		t.Fatal(err)
	}
	cfg := &config.Config{
		JWTSecret: "test-jwt-secret-with-enough-length-32",
		Auth: config.AuthConfig{
			SessionMode:           config.AuthSessionModeSecure,
			AccessTokenTTLMinutes: 15,
			RefreshTokenTTLDays:   7,
		},
	}
	svc := &SessionService{Cfg: cfg, DB: db, Admins: &admin.Store{DB: db}}
	res, err := svc.CreateSession(context.Background(), email, "test-password-123", "127.0.0.1", "test")
	if err != nil {
		t.Fatal(err)
	}
	if res.SessionID == uuid.Nil {
		t.Fatal("session id should not be nil")
	}
	ks, err := BuildKeySet(cfg)
	if err != nil {
		t.Fatal(err)
	}
	claims, err := ParseAccessToken(cfg, ks, res.AccessToken)
	if err != nil {
		t.Fatal(err)
	}
	if claims.SessionID != res.SessionID.String() {
		t.Fatalf("access token session_id %q should match created session %q", claims.SessionID, res.SessionID)
	}
	var sess AuthSession
	if err := db.First(&sess, "id = ?", res.SessionID.String()).Error; err != nil {
		t.Fatalf("session row not found by id: %v", err)
	}
	var refresh AuthRefreshToken
	if err := db.First(&refresh, "session_id = ?", res.SessionID.String()).Error; err != nil {
		t.Fatalf("refresh token not bound to session: %v", err)
	}
}

func newSessionServiceWithUser(t *testing.T) (*SessionService, *gorm.DB, uuid.UUID, string, *LoginSessionResult) {
	t.Helper()
	testID := uuid.NewString()
	testPassword := "pw-" + testID
	db, err := gorm.Open(sqlite.Open(fmt.Sprintf("file:%s?mode=memory&cache=shared", testID)), &gorm.Config{})
	if err != nil {
		t.Skipf("sqlite unavailable: %v", err)
	}
	if err := db.AutoMigrate(&AuthSession{}, &AuthRefreshToken{}, &AuthLoginAttempt{}, &admin.AdminUser{}); err != nil {
		t.Fatal(err)
	}
	hash, _ := bcrypt.GenerateFromPassword([]byte(testPassword), bcrypt.MinCost)
	uid := uuid.New()
	email := fmt.Sprintf("test-%s@example.com", testID)
	if err := db.Create(&admin.AdminUser{
		Base:         model.Base{ID: uid},
		Username:     "testuser-" + testID,
		Email:        email,
		PasswordHash: string(hash),
		Role:         "admin",
		Status:       "active",
	}).Error; err != nil {
		t.Fatal(err)
	}
	cfg := &config.Config{
		JWTSecret: "test-jwt-secret-with-enough-length-32",
		Auth: config.AuthConfig{
			SessionMode:           config.AuthSessionModeSecure,
			AccessTokenTTLMinutes: 15,
			RefreshTokenTTLDays:   7,
		},
	}
	svc := &SessionService{Cfg: cfg, DB: db, Admins: &admin.Store{DB: db}}
	res, err := svc.CreateSession(context.Background(), email, testPassword, "127.0.0.1", "test")
	if err != nil {
		t.Fatal(err)
	}
	return svc, db, uid, testPassword, res
}

// login snapshots the user's token_version onto the session row.
func TestCreateSessionSnapshotsTokenVersion(t *testing.T) {
	_, db, uid, _, res := newSessionServiceWithUser(t)
	var u admin.AdminUser
	if err := db.First(&u, "id = ?", uid).Error; err != nil {
		t.Fatal(err)
	}
	var sess AuthSession
	if err := db.First(&sess, "id = ?", res.SessionID).Error; err != nil {
		t.Fatal(err)
	}
	if sess.TokenVersion != u.TokenVersion || sess.TokenVersion < 1 {
		t.Fatalf("session token_version = %d, want user token_version %d (>=1)", sess.TokenVersion, u.TokenVersion)
	}
}

// refresh with a stale token_version (e.g. after delete user / password reset /
// role change / tenant disable bumped it) is rejected and the session revoked;
// refresh with the current token_version keeps working.
func TestRotateRefreshRejectsTokenVersionMismatch(t *testing.T) {
	svc, db, uid, password, res := newSessionServiceWithUser(t)

	// normal renewal unaffected before any invalidation
	rotated, err := svc.RotateRefresh(context.Background(), res.RefreshToken, "127.0.0.1", "test")
	if err != nil {
		t.Fatalf("normal refresh must succeed: %v", err)
	}

	// invalidation-style operation bumps token_version without revoking sessions
	if err := db.Model(&admin.AdminUser{}).Where("id = ?", uid).
		UpdateColumn("token_version", gorm.Expr("token_version + 1")).Error; err != nil {
		t.Fatal(err)
	}

	if _, err := svc.RotateRefresh(context.Background(), rotated.RefreshToken, "127.0.0.1", "test"); err == nil || err.Error() != ErrSessionRevoked {
		t.Fatalf("stale token_version refresh: err = %v, want %q", err, ErrSessionRevoked)
	}
	var sess AuthSession
	if err := db.First(&sess, "id = ?", res.SessionID).Error; err != nil {
		t.Fatal(err)
	}
	if sess.Status != SessionStatusRevoked || sess.RevokeReason != "token_version_mismatch" {
		t.Fatalf("session should be revoked with token_version_mismatch, got status=%s reason=%s", sess.Status, sess.RevokeReason)
	}

	// re-login after invalidation renews normally with the new version
	var u admin.AdminUser
	if err := db.First(&u, "id = ?", uid).Error; err != nil {
		t.Fatal(err)
	}
	res2, err := svc.CreateSession(context.Background(), u.Email, password, "127.0.0.1", "test")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := svc.RotateRefresh(context.Background(), res2.RefreshToken, "127.0.0.1", "test"); err != nil {
		t.Fatalf("refresh after re-login must succeed: %v", err)
	}
}

// pre-migration sessions carry token_version 0 and skip the check (no forced logout on upgrade).
func TestRotateRefreshSkipsZeroTokenVersionSession(t *testing.T) {
	svc, db, uid, _, res := newSessionServiceWithUser(t)
	if err := db.Model(&AuthSession{}).Where("id = ?", res.SessionID).
		UpdateColumn("token_version", 0).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Model(&admin.AdminUser{}).Where("id = ?", uid).
		UpdateColumn("token_version", gorm.Expr("token_version + 1")).Error; err != nil {
		t.Fatal(err)
	}
	if _, err := svc.RotateRefresh(context.Background(), res.RefreshToken, "127.0.0.1", "test"); err != nil {
		t.Fatalf("legacy session (token_version=0) refresh must still succeed: %v", err)
	}
}

func TestRefreshLegacyModePrefersBodyOverStaleCookie(t *testing.T) {
	testID := uuid.NewString()
	db, err := gorm.Open(sqlite.Open(fmt.Sprintf("file:%s?mode=memory&cache=shared", testID)), &gorm.Config{})
	if err != nil {
		t.Skipf("sqlite unavailable: %v", err)
	}
	if err := db.AutoMigrate(&AuthSession{}, &AuthRefreshToken{}, &AuthLoginAttempt{}, &admin.AdminUser{}); err != nil {
		t.Fatal(err)
	}
	hash, _ := bcrypt.GenerateFromPassword([]byte("test-password-123"), bcrypt.MinCost)
	uid := uuid.New()
	email := fmt.Sprintf("test-%s@example.com", testID)
	if err := db.Create(&admin.AdminUser{
		Base:         model.Base{ID: uid},
		Username:     "testuser-" + testID,
		Email:        email,
		PasswordHash: string(hash),
		Role:         "admin",
		Status:       "active",
	}).Error; err != nil {
		t.Fatal(err)
	}
	secureCfg := &config.Config{
		JWTSecret: "test-jwt-secret-with-enough-length-32",
		Auth: config.AuthConfig{
			SessionMode:           config.AuthSessionModeSecure,
			AccessTokenTTLMinutes: 15,
			RefreshTokenTTLDays:   7,
		},
	}
	svc := &SessionService{Cfg: secureCfg, DB: db, Admins: &admin.Store{DB: db}}
	res, err := svc.CreateSession(context.Background(), email, "test-password-123", "127.0.0.1", "test")
	if err != nil {
		t.Fatal(err)
	}
	// 先旋转一次，制造已作废的旧令牌（模拟 secure_session 时期遗留的 cookie）
	rotated, err := svc.RotateRefresh(context.Background(), res.RefreshToken, "127.0.0.1", "test")
	if err != nil {
		t.Fatal(err)
	}
	staleCookieToken := res.RefreshToken
	validBodyToken := rotated.RefreshToken

	legacyCfg := &config.Config{
		JWTSecret: "test-jwt-secret-with-enough-length-32",
		Auth: config.AuthConfig{
			SessionMode:           config.AuthSessionModeLegacy,
			AccessTokenTTLMinutes: 15,
			RefreshTokenTTLDays:   7,
		},
	}
	h := &SessionHandler{
		Cfg:      legacyCfg,
		Sessions: &SessionService{Cfg: legacyCfg, DB: db, Admins: &admin.Store{DB: db}},
		DB:       db,
	}
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	req := httptest.NewRequest("POST", "/api/v1/auth/refresh",
		bytes.NewBufferString(fmt.Sprintf(`{"refreshToken":%q}`, validBodyToken)))
	req.Header.Set("Content-Type", "application/json")
	req.AddCookie(&http.Cookie{Name: refreshCookieName, Value: staleCookieToken})
	req.RemoteAddr = "127.0.0.1:12345"
	c.Request = req
	h.Refresh(c)
	if w.Code != 200 {
		t.Fatalf("legacy refresh with stale cookie + valid body = %d, want 200 (body=%s)", w.Code, w.Body.String())
	}
}
