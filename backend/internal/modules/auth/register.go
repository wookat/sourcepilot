package auth

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/trademind-ai/trademind/backend/internal/modules/admin"
	"github.com/trademind-ai/trademind/backend/internal/modules/operationlog"
	"github.com/trademind-ai/trademind/backend/internal/pkg/model"
	"github.com/trademind-ai/trademind/backend/internal/pkg/response"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"
)

// registrationTenant mirrors the tenants table without a cross-module import
// (auth reads tenants by table name; see tenant_state.go).
type registrationTenant struct {
	ID        int64  `gorm:"primaryKey;autoIncrement"`
	Name      string `gorm:"size:128;not null"`
	Status    string `gorm:"size:16;not null"`
	CreatedAt time.Time
	UpdatedAt time.Time
}

func (registrationTenant) TableName() string { return "tenants" }

// registrationTenantName derives a unique tenant name from the owner email,
// keeping within the 128-char column while avoiding soft-delete name clashes.
func registrationTenantName(email string) string {
	suffix := make([]byte, 4)
	_, _ = rand.Read(suffix)
	if len(email) > 100 {
		email = email[:100]
	}
	return email + "-" + hex.EncodeToString(suffix)
}

// createRegistrationUser provisions a fresh tenant for a self-registered
// account and creates its admin user inside that tenant atomically, so a new
// registration never lands in the platform tenant (tenant 0).
func createRegistrationUser(ctx context.Context, db *gorm.DB, u *admin.AdminUser) error {
	return db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		t := registrationTenant{Name: registrationTenantName(u.Email), Status: "active"}
		if err := tx.Create(&t).Error; err != nil {
			return err
		}
		u.TenantID = t.ID
		return tx.Create(u).Error
	})
}

type registerBody struct {
	Email           string `json:"email" binding:"required,email,max=128"`
	Code            string `json:"code" binding:"required,len=6"`
	Password        string `json:"password" binding:"required,min=6,max=128"`
	ConfirmPassword string `json:"confirmPassword" binding:"required,eqfield=Password"`
}

func (h *Handler) Register(c *gin.Context) {
	if h.Redis == nil {
		response.Fail(c, 503, response.CodeInternalError, "redis unavailable")
		return
	}
	var body registerBody
	if err := c.ShouldBindJSON(&body); err != nil {
		response.Fail(c, 400, response.CodeBadRequest, "invalid body")
		return
	}
	emailAddr := strings.ToLower(strings.TrimSpace(body.Email))

	// Verify code
	codeKey := fmt.Sprintf("email_code:register:%s", emailAddr)
	storedCode, err := h.Redis.Get(c.Request.Context(), codeKey).Result()
	if err != nil || storedCode != body.Code {
		if h.OpLog != nil {
			_ = h.OpLog.Write(c, operationlog.WriteOpts{
				Username: emailAddr,
				Action:   "register",
				Resource: "auth",
				Status:   "failed",
				Message:  "invalid verification code",
			})
		}
		response.Fail(c, 400, response.CodeBadRequest, "invalid verification code")
		return
	}

	// Hash password
	hash, err := bcrypt.GenerateFromPassword([]byte(body.Password), bcrypt.DefaultCost)
	if err != nil {
		response.Fail(c, 500, response.CodeInternalError, "password error")
		return
	}

	// Create user in its own fresh tenant
	u := admin.AdminUser{
		Base:         model.Base{},
		Username:     admin.NewInternalUsername(),
		Email:        emailAddr,
		DisplayName:  emailAddr,
		PasswordHash: string(hash),
		Role:         "admin", // TODO(RBAC): first version all admin scope
		Status:       "active",
	}

	if err := createRegistrationUser(c.Request.Context(), h.Admins.DB, &u); err != nil {
		if h.OpLog != nil {
			_ = h.OpLog.Write(c, operationlog.WriteOpts{
				Username: emailAddr,
				Action:   "register",
				Resource: "auth",
				Status:   "failed",
				Message:  "email already registered",
			})
		}
		response.Fail(c, 400, response.CodeBadRequest, "email already registered")
		return
	}

	// Delete code after successful registration
	h.Redis.Del(c.Request.Context(), codeKey)

	// Log success
	if h.OpLog != nil {
		uid := u.ID
		_ = h.OpLog.Write(c, operationlog.WriteOpts{
			AdminUserID: &uid,
			Username:    u.Email,
			Action:      "register",
			Resource:    "auth",
			Status:      "success",
		})
	}

	// Auto login
	res, err := h.LoginSvc.Login(c.Request.Context(), emailAddr, body.Password, c.ClientIP(), c.Request.UserAgent())
	if err != nil {
		response.Fail(c, 500, response.CodeInternalError, "auto login failed")
		return
	}

	response.OK(c, gin.H{
		"token":     res.Token,
		"expiresAt": res.ExpiresAt,
		"user":      res.User,
	})
}
