package auth

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"github.com/google/uuid"
	"github.com/trademind-ai/trademind/backend/internal/config"
	"github.com/trademind-ai/trademind/backend/internal/modules/admin"
	"github.com/trademind-ai/trademind/backend/internal/modules/operationlog"
	"github.com/trademind-ai/trademind/backend/internal/pkg/model"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

func newLoginAuditTestHandler(t *testing.T, tenantID int64) (*Handler, *gorm.DB, string) {
	t.Helper()
	testID := uuid.NewString()
	db, err := gorm.Open(sqlite.Open(fmt.Sprintf("file:%s?mode=memory&cache=shared", testID)), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	if err != nil {
		t.Skipf("sqlite unavailable: %v", err)
	}
	if err := db.AutoMigrate(&admin.AdminUser{}, &operationlog.OperationLog{}); err != nil {
		t.Fatal(err)
	}
	// Trusted tenant-state lookups fail closed when the table is missing.
	if err := db.Exec(`CREATE TABLE IF NOT EXISTS tenants (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		name TEXT NOT NULL,
		status TEXT NOT NULL DEFAULT 'active',
		created_by TEXT,
		created_at DATETIME,
		updated_at DATETIME,
		deleted_at DATETIME
	)`).Error; err != nil {
		t.Fatal(err)
	}
	hash, _ := bcrypt.GenerateFromPassword([]byte("test-password-123"), bcrypt.DefaultCost)
	email := fmt.Sprintf("tenant-audit-%s@example.com", testID)
	if err := db.Create(&admin.AdminUser{
		Base:         model.Base{ID: uuid.New()},
		TenantID:     tenantID,
		Username:     "tenant-audit-" + testID,
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
			SessionMode:           config.AuthSessionModeLegacy,
			AccessTokenTTLMinutes: 15,
			RefreshTokenTTLDays:   7,
		},
	}
	admins := &admin.Store{DB: db}
	h := &Handler{
		LoginSvc: &LoginService{Cfg: cfg, Admins: admins},
		Admins:   admins,
		OpLog:    &operationlog.Service{DB: db},
		DB:       db,
		Cfg:      cfg,
	}
	return h, db, email
}

func doLogin(t *testing.T, h *Handler, account, password string) {
	t.Helper()
	gin.SetMode(gin.TestMode)
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	body, _ := json.Marshal(map[string]string{"account": account, "password": password})
	c.Request = httptest.NewRequest("POST", "/api/v1/auth/login", bytes.NewReader(body))
	c.Request.Header.Set("Content-Type", "application/json")
	h.Login(c)
}

func lastLoginLog(t *testing.T, db *gorm.DB) *operationlog.OperationLog {
	t.Helper()
	var row operationlog.OperationLog
	if err := db.Where("action = ? AND resource = ?", "login", "auth").
		Order("created_at DESC, id DESC").First(&row).Error; err != nil {
		t.Fatalf("load login log: %v", err)
	}
	return &row
}

func TestLoginSuccessAuditUsesUserTenant(t *testing.T) {
	h, db, email := newLoginAuditTestHandler(t, 7)
	doLogin(t, h, email, "test-password-123")
	row := lastLoginLog(t, db)
	if row.Status != "success" {
		t.Fatalf("got status %q, want success", row.Status)
	}
	if row.TenantID != 7 {
		t.Fatalf("got tenant_id %d, want 7", row.TenantID)
	}
}

func TestLoginFailureKnownAccountAuditUsesUserTenant(t *testing.T) {
	h, db, email := newLoginAuditTestHandler(t, 7)
	doLogin(t, h, email, "wrong-password")
	row := lastLoginLog(t, db)
	if row.Status != "failed" {
		t.Fatalf("got status %q, want failed", row.Status)
	}
	if row.TenantID != 7 {
		t.Fatalf("got tenant_id %d, want 7", row.TenantID)
	}
}

func TestLoginFailureUnknownAccountAuditStaysTenantZero(t *testing.T) {
	h, db, _ := newLoginAuditTestHandler(t, 7)
	doLogin(t, h, "nobody@example.com", "whatever")
	row := lastLoginLog(t, db)
	if row.Status != "failed" {
		t.Fatalf("got status %q, want failed", row.Status)
	}
	if row.TenantID != 0 {
		t.Fatalf("got tenant_id %d, want 0", row.TenantID)
	}
}
