package carrier

import (
	"context"
	"errors"
	"fmt"
	"regexp"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/trademind-ai/trademind/backend/internal/modules/operationlog"
	"github.com/trademind-ai/trademind/backend/internal/pkg/adminperm"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// ErrNotFound is returned when a carrier is absent in the tenant scope.
var ErrNotFound = errors.New("carrier not found")

// Service owns carrier CRUD with tenant isolation.
type Service struct {
	DB    *gorm.DB
	OpLog *operationlog.Service
}

var validCode = regexp.MustCompile(`^[a-z0-9_-]{1,64}$`)

// EnsurePresets idempotently seeds built-in carriers for one tenant.
func EnsurePresets(ctx context.Context, db *gorm.DB, tenantID int64) error {
	if db == nil {
		return fmt.Errorf("carrier: no db")
	}
	rows := make([]Carrier, 0, len(Presets()))
	for _, p := range Presets() {
		rows = append(rows, Carrier{
			TenantID:            tenantID,
			Code:                p.Code,
			Name:                p.Name,
			Enabled:             true,
			IsPreset:            true,
			TrackingURLTemplate: p.TrackingURLTemplate,
			SortOrder:           p.SortOrder,
		})
	}
	return db.WithContext(ctx).
		Clauses(clause.OnConflict{Columns: []clause.Column{{Name: "tenant_id"}, {Name: "code"}}, DoNothing: true}).
		Create(&rows).Error
}

// ListQuery filters GET /carriers.
type ListQuery struct {
	// EnabledOnly returns only enabled carriers (shipping selectors).
	EnabledOnly bool
	Keyword     string
}

// List returns tenant carriers ordered by sort_order, seeding presets first.
func (s *Service) List(c *gin.Context, q ListQuery) ([]Carrier, error) {
	if s == nil || s.DB == nil {
		return nil, fmt.Errorf("carrier: no db")
	}
	tid, err := adminperm.TenantIDFromGin(c)
	if err != nil {
		return nil, err
	}
	if err := EnsurePresets(c.Request.Context(), s.DB, tid); err != nil {
		return nil, err
	}
	tx := s.DB.WithContext(c.Request.Context()).Where("tenant_id = ?", tid)
	if q.EnabledOnly {
		tx = tx.Where("enabled = ?", true)
	}
	if kw := strings.TrimSpace(q.Keyword); kw != "" {
		like := "%" + strings.ToLower(kw) + "%"
		tx = tx.Where("LOWER(code) LIKE ? OR LOWER(name) LIKE ?", like, like)
	}
	var rows []Carrier
	if err := tx.Order("sort_order ASC, code ASC").Find(&rows).Error; err != nil {
		return nil, err
	}
	return rows, nil
}

// CreateBody POST /carriers.
type CreateBody struct {
	Code                string `json:"code"`
	Name                string `json:"name"`
	TrackingURLTemplate string `json:"trackingUrlTemplate"`
	SortOrder           int    `json:"sortOrder"`
}

// Create adds a custom carrier in the current tenant.
func (s *Service) Create(c *gin.Context, body CreateBody, adminID *uuid.UUID) (*Carrier, error) {
	if s == nil || s.DB == nil {
		return nil, fmt.Errorf("carrier: no db")
	}
	tid, err := adminperm.TenantIDFromGin(c)
	if err != nil {
		return nil, err
	}
	code := strings.ToLower(strings.TrimSpace(body.Code))
	if !validCode.MatchString(code) {
		return nil, fmt.Errorf("编码仅允许小写字母、数字、下划线或中划线（1-64 位）")
	}
	name := strings.TrimSpace(body.Name)
	if name == "" {
		return nil, fmt.Errorf("名称不能为空")
	}
	var exists int64
	if err := s.DB.WithContext(c.Request.Context()).Model(&Carrier{}).
		Where("tenant_id = ? AND code = ?", tid, code).Count(&exists).Error; err != nil {
		return nil, err
	}
	if exists > 0 {
		return nil, fmt.Errorf("编码已存在：%s", code)
	}
	row := Carrier{
		TenantID:            tid,
		Code:                code,
		Name:                name,
		Enabled:             true,
		TrackingURLTemplate: strings.TrimSpace(body.TrackingURLTemplate),
		SortOrder:           body.SortOrder,
	}
	if err := s.DB.WithContext(c.Request.Context()).Create(&row).Error; err != nil {
		return nil, err
	}
	s.log(c, adminID, "carrier.create", row.ID.String(), fmt.Sprintf("code=%s name=%s", row.Code, row.Name))
	return &row, nil
}

// UpdateBody PUT /carriers/:id (partial semantics).
type UpdateBody struct {
	Name                *string `json:"name"`
	Enabled             *bool   `json:"enabled"`
	TrackingURLTemplate *string `json:"trackingUrlTemplate"`
	SortOrder           *int    `json:"sortOrder"`
}

// Update renames / toggles a carrier in the current tenant.
func (s *Service) Update(c *gin.Context, id uuid.UUID, body UpdateBody, adminID *uuid.UUID) (*Carrier, error) {
	row, err := s.findScoped(c, id)
	if err != nil {
		return nil, err
	}
	if body.Name != nil {
		name := strings.TrimSpace(*body.Name)
		if name == "" {
			return nil, fmt.Errorf("名称不能为空")
		}
		row.Name = name
	}
	if body.Enabled != nil {
		row.Enabled = *body.Enabled
	}
	if body.TrackingURLTemplate != nil {
		row.TrackingURLTemplate = strings.TrimSpace(*body.TrackingURLTemplate)
	}
	if body.SortOrder != nil {
		row.SortOrder = *body.SortOrder
	}
	if err := s.DB.WithContext(c.Request.Context()).Save(row).Error; err != nil {
		return nil, err
	}
	s.log(c, adminID, "carrier.update", row.ID.String(), fmt.Sprintf("code=%s enabled=%v", row.Code, row.Enabled))
	return row, nil
}

// Delete removes a custom carrier; presets can only be disabled.
func (s *Service) Delete(c *gin.Context, id uuid.UUID, adminID *uuid.UUID) error {
	row, err := s.findScoped(c, id)
	if err != nil {
		return err
	}
	if row.IsPreset {
		return fmt.Errorf("预置物流商不可删除，可停用")
	}
	if err := s.DB.WithContext(c.Request.Context()).
		Where("id = ? AND tenant_id = ?", row.ID, row.TenantID).Delete(&Carrier{}).Error; err != nil {
		return err
	}
	s.log(c, adminID, "carrier.delete", row.ID.String(), fmt.Sprintf("code=%s name=%s", row.Code, row.Name))
	return nil
}

func (s *Service) findScoped(c *gin.Context, id uuid.UUID) (*Carrier, error) {
	if s == nil || s.DB == nil {
		return nil, fmt.Errorf("carrier: no db")
	}
	tid, err := adminperm.TenantIDFromGin(c)
	if err != nil {
		return nil, err
	}
	var row Carrier
	if err := s.DB.WithContext(c.Request.Context()).
		First(&row, "id = ? AND tenant_id = ?", id, tid).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	return &row, nil
}

// ResolveEnabled finds an enabled carrier by code (exact) or name (exact) in
// the current tenant, seeding presets first so fresh tenants resolve too.
func (s *Service) ResolveEnabled(c *gin.Context, key string) (*Carrier, error) {
	if s == nil || s.DB == nil {
		return nil, fmt.Errorf("carrier: no db")
	}
	k := strings.TrimSpace(key)
	if k == "" {
		return nil, ErrNotFound
	}
	tid, err := adminperm.TenantIDFromGin(c)
	if err != nil {
		return nil, err
	}
	return s.ResolveEnabledTenant(c.Request.Context(), tid, k)
}

// ResolveEnabledTenant is ResolveEnabled for non-HTTP callers (background /
// automation flows) that already carry a tenant id.
func (s *Service) ResolveEnabledTenant(ctx context.Context, tenantID int64, key string) (*Carrier, error) {
	if s == nil || s.DB == nil {
		return nil, fmt.Errorf("carrier: no db")
	}
	k := strings.TrimSpace(key)
	if k == "" {
		return nil, ErrNotFound
	}
	if err := EnsurePresets(ctx, s.DB, tenantID); err != nil {
		return nil, err
	}
	var row Carrier
	err := s.DB.WithContext(ctx).
		Where("tenant_id = ? AND enabled = ? AND (code = ? OR name = ?)", tenantID, true, strings.ToLower(k), k).
		Order("is_preset DESC, sort_order ASC").
		First(&row).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		// Loose fallback: pasted names are often shorter than the official
		// one (顺丰 → 顺丰速运), so try a name-prefix match before giving up.
		err = s.DB.WithContext(ctx).
			Where("tenant_id = ? AND enabled = ? AND name LIKE ?", tenantID, true, k+"%").
			Order("is_preset DESC, sort_order ASC").
			First(&row).Error
	}
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	return &row, nil
}

// TrackingURLFor renders the carrier's tracking link for one waybill.
func (r *Carrier) TrackingURLFor(trackingNo string) string {
	tn := strings.TrimSpace(trackingNo)
	if r == nil || tn == "" || strings.TrimSpace(r.TrackingURLTemplate) == "" {
		return ""
	}
	return strings.ReplaceAll(r.TrackingURLTemplate, "{trackingNo}", tn)
}

func (s *Service) log(c *gin.Context, adminID *uuid.UUID, action, resourceID, msg string) {
	if s == nil || s.OpLog == nil {
		return
	}
	_ = s.OpLog.Write(c, operationlog.WriteOpts{
		AdminUserID: adminID,
		Action:      action,
		Resource:    "carrier",
		ResourceID:  resourceID,
		Status:      "success",
		Message:     msg,
	})
}
