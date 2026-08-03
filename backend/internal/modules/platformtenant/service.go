package platformtenant

import (
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/trademind-ai/trademind/backend/internal/modules/admin"
	"github.com/trademind-ai/trademind/backend/internal/modules/operationlog"
	"github.com/trademind-ai/trademind/backend/internal/pkg/adminperm"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"
)

var (
	ErrDuplicateTenantName = errors.New("租户名称已存在")
	ErrDuplicateAdminEmail = errors.New("管理员邮箱已被使用")
)

// Service manages platform tenants.
type Service struct {
	DB    *gorm.DB
	OpLog *operationlog.Service
}

// TenantRow is one tenant in the platform tenant list.
type TenantRow struct {
	ID         int64  `json:"id"`
	Name       string `json:"name"`
	AdminCount int64  `json:"adminCount"`
	CreatedAt  string `json:"createdAt"`
}

// CreateBody creates a tenant together with its initial admin account.
type CreateBody struct {
	Name          string `json:"name"`
	AdminEmail    string `json:"adminEmail"`
	AdminPassword string `json:"adminPassword"`
}

// CreateResult echoes the created tenant and its initial admin.
type CreateResult struct {
	Tenant     TenantRow `json:"tenant"`
	AdminID    string    `json:"adminId"`
	AdminEmail string    `json:"adminEmail"`
}

func userCountByTenant(tx *gorm.DB, tenantID int64) (int64, error) {
	var cnt int64
	err := tx.Model(&admin.AdminUser{}).Where("tenant_id = ?", tenantID).Count(&cnt).Error
	return cnt, err
}

// List returns all tenants, including the implicit platform tenant 0.
func (s *Service) List(c *gin.Context) ([]TenantRow, error) {
	if s == nil || s.DB == nil {
		return nil, fmt.Errorf("platformtenant: no db")
	}
	tx := s.DB.WithContext(c.Request.Context())
	var rows []Tenant
	if err := tx.Order("id ASC").Find(&rows).Error; err != nil {
		return nil, err
	}
	out := make([]TenantRow, 0, len(rows)+1)
	platformCount, err := userCountByTenant(tx, PlatformTenantID)
	if err != nil {
		return nil, err
	}
	out = append(out, TenantRow{
		ID:         PlatformTenantID,
		Name:       "平台租户（默认）",
		AdminCount: platformCount,
	})
	for i := range rows {
		cnt, err := userCountByTenant(tx, rows[i].ID)
		if err != nil {
			return nil, err
		}
		out = append(out, TenantRow{
			ID:         rows[i].ID,
			Name:       rows[i].Name,
			AdminCount: cnt,
			CreatedAt:  rows[i].CreatedAt.UTC().Format(time.RFC3339),
		})
	}
	return out, nil
}

// Create provisions a tenant and its initial admin account in one transaction.
func (s *Service) Create(c *gin.Context, body CreateBody, actorID *uuid.UUID) (*CreateResult, error) {
	if s == nil || s.DB == nil {
		return nil, fmt.Errorf("platformtenant: no db")
	}
	name := strings.TrimSpace(body.Name)
	if name == "" || len(name) > 128 {
		return nil, fmt.Errorf("租户名称必填（不超过 128 字符）")
	}
	email := strings.ToLower(strings.TrimSpace(body.AdminEmail))
	if email == "" || !strings.Contains(email, "@") || len(email) > 128 {
		return nil, fmt.Errorf("管理员邮箱格式无效")
	}
	pw := strings.TrimSpace(body.AdminPassword)
	if len(pw) < 6 || len(pw) > 128 {
		return nil, fmt.Errorf("管理员密码至少 6 位")
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(pw), bcrypt.DefaultCost)
	if err != nil {
		return nil, err
	}
	ctx := c.Request.Context()
	var cnt int64
	if err := s.DB.WithContext(ctx).Model(&Tenant{}).Where("LOWER(name) = ?", strings.ToLower(name)).Count(&cnt).Error; err != nil {
		return nil, err
	}
	if cnt > 0 {
		return nil, ErrDuplicateTenantName
	}
	if err := s.DB.WithContext(ctx).Model(&admin.AdminUser{}).Where("LOWER(TRIM(email)) = ?", email).Count(&cnt).Error; err != nil {
		return nil, err
	}
	if cnt > 0 {
		return nil, ErrDuplicateAdminEmail
	}
	t := &Tenant{Name: name, CreatedBy: actorID}
	u := &admin.AdminUser{
		Username:     admin.NewInternalUsername(),
		Email:        email,
		DisplayName:  email,
		PasswordHash: string(hash),
		Role:         adminperm.RoleAdmin,
		Status:       admin.StatusActive,
	}
	err = s.DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(t).Error; err != nil {
			return err
		}
		u.TenantID = t.ID
		return tx.Create(u).Error
	})
	if err != nil {
		return nil, err
	}
	if s.OpLog != nil {
		_ = s.OpLog.Write(c, operationlog.WriteOpts{
			AdminUserID: actorID,
			Action:      "tenant.create",
			Resource:    "tenant",
			ResourceID:  fmt.Sprintf("%d", t.ID),
			Status:      "success",
			Message:     fmt.Sprintf("tenantId=%d name=%s adminEmail=%s", t.ID, name, email),
		})
	}
	return &CreateResult{
		Tenant: TenantRow{
			ID:         t.ID,
			Name:       t.Name,
			AdminCount: 1,
			CreatedAt:  t.CreatedAt.UTC().Format(time.RFC3339),
		},
		AdminID:    u.ID.String(),
		AdminEmail: email,
	}, nil
}
