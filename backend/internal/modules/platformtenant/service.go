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
	ErrDuplicateTenantName    = errors.New("租户名称已存在")
	ErrDuplicateAdminEmail    = errors.New("管理员邮箱已被使用")
	ErrTenantNotFound         = errors.New("租户不存在")
	ErrPlatformTenantReadOnly = errors.New("平台租户不可停用或改名")
)

// Service manages platform tenants.
type Service struct {
	DB    *gorm.DB
	OpLog *operationlog.Service
	// PurgeSync runs purge tasks synchronously instead of in a background
	// goroutine (tests only).
	PurgeSync bool
}

// TenantRow is one tenant in the platform tenant list.
type TenantRow struct {
	ID         int64  `json:"id"`
	Name       string `json:"name"`
	Status     string `json:"status"`
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
	platformRow := TenantRow{
		ID:         PlatformTenantID,
		Name:       "平台租户（默认）",
		Status:     StatusActive,
		AdminCount: platformCount,
	}
	// 平台租户 0 是隐式租户，无 tenants 表行；用最早的平台管理员创建时间作为创建时间。
	var firstAdmin admin.AdminUser
	if err := tx.Where("tenant_id = ?", PlatformTenantID).Order("created_at ASC").
		Select("created_at").Limit(1).Find(&firstAdmin).Error; err != nil {
		return nil, err
	}
	if !firstAdmin.CreatedAt.IsZero() {
		platformRow.CreatedAt = firstAdmin.CreatedAt.UTC().Format(time.RFC3339)
	}
	out = append(out, platformRow)
	for i := range rows {
		cnt, err := userCountByTenant(tx, rows[i].ID)
		if err != nil {
			return nil, err
		}
		out = append(out, TenantRow{
			ID:         rows[i].ID,
			Name:       rows[i].Name,
			Status:     normalizeStatus(rows[i].Status),
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
			Status:     normalizeStatus(t.Status),
			AdminCount: 1,
			CreatedAt:  t.CreatedAt.UTC().Format(time.RFC3339),
		},
		AdminID:    u.ID.String(),
		AdminEmail: email,
	}, nil
}

func normalizeStatus(s string) string {
	if strings.EqualFold(strings.TrimSpace(s), StatusDisabled) {
		return StatusDisabled
	}
	return StatusActive
}

func (s *Service) writeOpLog(c *gin.Context, actorID *uuid.UUID, action string, tenantID int64, message string) {
	if s.OpLog == nil {
		return
	}
	_ = s.OpLog.Write(c, operationlog.WriteOpts{
		AdminUserID: actorID,
		Action:      action,
		Resource:    "tenant",
		ResourceID:  fmt.Sprintf("%d", tenantID),
		Status:      "success",
		Message:     message,
	})
}

func (s *Service) findTenant(c *gin.Context, id int64) (*Tenant, error) {
	if s == nil || s.DB == nil {
		return nil, fmt.Errorf("platformtenant: no db")
	}
	if id == PlatformTenantID {
		return nil, ErrPlatformTenantReadOnly
	}
	var t Tenant
	if err := s.DB.WithContext(c.Request.Context()).First(&t, "id = ?", id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrTenantNotFound
		}
		return nil, err
	}
	return &t, nil
}

// Rename changes a tenant's display name. The platform tenant cannot be renamed.
func (s *Service) Rename(c *gin.Context, id int64, newName string, actorID *uuid.UUID) (*TenantRow, error) {
	name := strings.TrimSpace(newName)
	if name == "" || len(name) > 128 {
		return nil, fmt.Errorf("租户名称必填（不超过 128 字符）")
	}
	t, err := s.findTenant(c, id)
	if err != nil {
		return nil, err
	}
	ctx := c.Request.Context()
	var cnt int64
	if err := s.DB.WithContext(ctx).Model(&Tenant{}).
		Where("LOWER(name) = ? AND id <> ?", strings.ToLower(name), t.ID).Count(&cnt).Error; err != nil {
		return nil, err
	}
	if cnt > 0 {
		return nil, ErrDuplicateTenantName
	}
	oldName := t.Name
	if err := s.DB.WithContext(ctx).Model(t).Update("name", name).Error; err != nil {
		return nil, err
	}
	s.writeOpLog(c, actorID, "tenant.rename", t.ID, fmt.Sprintf("tenantId=%d oldName=%s newName=%s", t.ID, oldName, name))
	cntUsers, err := userCountByTenant(s.DB.WithContext(ctx), t.ID)
	if err != nil {
		return nil, err
	}
	return &TenantRow{
		ID:         t.ID,
		Name:       name,
		Status:     normalizeStatus(t.Status),
		AdminCount: cntUsers,
		CreatedAt:  t.CreatedAt.UTC().Format(time.RFC3339),
	}, nil
}

// SetStatus enables or disables a tenant. Disabling rejects every login of
// the tenant's accounts and invalidates existing sessions on their next
// request. The platform tenant (tenant 0) cannot be disabled.
func (s *Service) SetStatus(c *gin.Context, id int64, status string, actorID *uuid.UUID) (*TenantRow, error) {
	if status != StatusActive && status != StatusDisabled {
		return nil, fmt.Errorf("无效的租户状态")
	}
	t, err := s.findTenant(c, id)
	if err != nil {
		return nil, err
	}
	ctx := c.Request.Context()
	if normalizeStatus(t.Status) != status {
		if err := s.DB.WithContext(ctx).Model(t).Update("status", status).Error; err != nil {
			return nil, err
		}
	}
	action := "tenant.enable"
	if status == StatusDisabled {
		action = "tenant.disable"
	}
	s.writeOpLog(c, actorID, action, t.ID, fmt.Sprintf("tenantId=%d name=%s status=%s", t.ID, t.Name, status))
	cntUsers, err := userCountByTenant(s.DB.WithContext(ctx), t.ID)
	if err != nil {
		return nil, err
	}
	return &TenantRow{
		ID:         t.ID,
		Name:       t.Name,
		Status:     status,
		AdminCount: cntUsers,
		CreatedAt:  t.CreatedAt.UTC().Format(time.RFC3339),
	}, nil
}
