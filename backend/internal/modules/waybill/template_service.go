package waybill

import (
	"errors"
	"fmt"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/trademind-ai/trademind/backend/internal/modules/carrier"
	"github.com/trademind-ai/trademind/backend/internal/modules/operationlog"
	"github.com/trademind-ai/trademind/backend/internal/pkg/adminperm"
	"gorm.io/gorm"
)

// ErrTemplateNotFound is returned when a template is absent in the tenant scope.
var ErrTemplateNotFound = errors.New("waybill template not found")

// ErrRuleNotFound is returned when a shipping rule is absent in the tenant scope.
var ErrRuleNotFound = errors.New("shipping rule not found")

// Service owns waybill template + shipping rule CRUD with tenant isolation.
type Service struct {
	DB       *gorm.DB
	OpLog    *operationlog.Service
	Carriers *carrier.Service
}

func validSize(code string) bool {
	for _, s := range ValidSizes() {
		if s == code {
			return true
		}
	}
	return false
}

// ListTemplates returns tenant templates ordered by sort_order, seeding
// presets first so fresh tenants always have the three built-in sizes.
func (s *Service) ListTemplates(c *gin.Context) ([]Template, error) {
	if s == nil || s.DB == nil {
		return nil, fmt.Errorf("waybill: no db")
	}
	tid, err := adminperm.TenantIDFromGin(c)
	if err != nil {
		return nil, err
	}
	if err := EnsureTemplatePresets(c.Request.Context(), s.DB, tid); err != nil {
		return nil, err
	}
	var rows []Template
	if err := s.DB.WithContext(c.Request.Context()).
		Where("tenant_id = ?", tid).
		Order("sort_order ASC, created_at ASC").Find(&rows).Error; err != nil {
		return nil, err
	}
	return rows, nil
}

// GetTemplate returns one tenant template by id.
func (s *Service) GetTemplate(c *gin.Context, id uuid.UUID) (*Template, error) {
	return s.findTemplateScoped(c, id)
}

// TemplateBody is the create / update payload for a template.
type TemplateBody struct {
	Name            string  `json:"name"`
	SizeCode        string  `json:"sizeCode"`
	ShowRecipient   *bool   `json:"showRecipient"`
	ShowSender      *bool   `json:"showSender"`
	ShowItems       *bool   `json:"showItems"`
	ShowRemark      *bool   `json:"showRemark"`
	ShowCarrierLogo *bool   `json:"showCarrierLogo"`
	HeaderText      *string `json:"headerText"`
	FooterText      *string `json:"footerText"`
	IsDefault       *bool   `json:"isDefault"`
	SortOrder       *int    `json:"sortOrder"`
}

// CreateTemplate adds a custom template in the current tenant.
func (s *Service) CreateTemplate(c *gin.Context, body TemplateBody, adminID *uuid.UUID) (*Template, error) {
	if s == nil || s.DB == nil {
		return nil, fmt.Errorf("waybill: no db")
	}
	tid, err := adminperm.TenantIDFromGin(c)
	if err != nil {
		return nil, err
	}
	name := strings.TrimSpace(body.Name)
	if name == "" {
		return nil, fmt.Errorf("模板名称不能为空")
	}
	if !validSize(body.SizeCode) {
		return nil, fmt.Errorf("纸张规格无效，可选：100x180 / 100x150 / a4_list")
	}
	row := Template{
		TenantID:        tid,
		Name:            name,
		SizeCode:        body.SizeCode,
		ShowRecipient:   boolOr(body.ShowRecipient, true),
		ShowSender:      boolOr(body.ShowSender, true),
		ShowItems:       boolOr(body.ShowItems, true),
		ShowRemark:      boolOr(body.ShowRemark, true),
		ShowCarrierLogo: boolOr(body.ShowCarrierLogo, false),
		HeaderText:      strOr(body.HeaderText),
		FooterText:      strOr(body.FooterText),
		SortOrder:       intOr(body.SortOrder, 0),
	}
	err = s.DB.WithContext(c.Request.Context()).Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(&row).Error; err != nil {
			return err
		}
		if body.IsDefault != nil && *body.IsDefault {
			return s.setDefaultTx(tx, tid, row.ID)
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	if body.IsDefault != nil && *body.IsDefault {
		row.IsDefault = true
	}
	s.log(c, adminID, "waybill_template.create", row.ID.String(), fmt.Sprintf("name=%s size=%s", row.Name, row.SizeCode))
	return &row, nil
}

// UpdateTemplate edits a template in the current tenant.
func (s *Service) UpdateTemplate(c *gin.Context, id uuid.UUID, body TemplateBody, adminID *uuid.UUID) (*Template, error) {
	row, err := s.findTemplateScoped(c, id)
	if err != nil {
		return nil, err
	}
	if strings.TrimSpace(body.Name) != "" {
		row.Name = strings.TrimSpace(body.Name)
	}
	if body.SizeCode != "" {
		if !validSize(body.SizeCode) {
			return nil, fmt.Errorf("纸张规格无效，可选：100x180 / 100x150 / a4_list")
		}
		row.SizeCode = body.SizeCode
	}
	if body.ShowRecipient != nil {
		row.ShowRecipient = *body.ShowRecipient
	}
	if body.ShowSender != nil {
		row.ShowSender = *body.ShowSender
	}
	if body.ShowItems != nil {
		row.ShowItems = *body.ShowItems
	}
	if body.ShowRemark != nil {
		row.ShowRemark = *body.ShowRemark
	}
	if body.ShowCarrierLogo != nil {
		row.ShowCarrierLogo = *body.ShowCarrierLogo
	}
	if body.HeaderText != nil {
		row.HeaderText = strings.TrimSpace(*body.HeaderText)
	}
	if body.FooterText != nil {
		row.FooterText = strings.TrimSpace(*body.FooterText)
	}
	if body.SortOrder != nil {
		row.SortOrder = *body.SortOrder
	}
	err = s.DB.WithContext(c.Request.Context()).Transaction(func(tx *gorm.DB) error {
		if err := tx.Save(row).Error; err != nil {
			return err
		}
		if body.IsDefault != nil && *body.IsDefault && !row.IsDefault {
			if err := s.setDefaultTx(tx, row.TenantID, row.ID); err != nil {
				return err
			}
			row.IsDefault = true
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	s.log(c, adminID, "waybill_template.update", row.ID.String(), fmt.Sprintf("name=%s size=%s default=%v", row.Name, row.SizeCode, row.IsDefault))
	return row, nil
}

// setDefaultTx makes one template the tenant default (single default).
func (s *Service) setDefaultTx(tx *gorm.DB, tenantID int64, id uuid.UUID) error {
	if err := tx.Model(&Template{}).
		Where("tenant_id = ? AND is_default = ?", tenantID, true).
		Update("is_default", false).Error; err != nil {
		return err
	}
	return tx.Model(&Template{}).Where("id = ?", id).Update("is_default", true).Error
}

// DeleteTemplate removes a custom template; presets cannot be deleted.
func (s *Service) DeleteTemplate(c *gin.Context, id uuid.UUID, adminID *uuid.UUID) error {
	row, err := s.findTemplateScoped(c, id)
	if err != nil {
		return err
	}
	if row.IsPreset {
		return fmt.Errorf("预置模板不可删除，可修改字段配置")
	}
	if err := s.DB.WithContext(c.Request.Context()).
		Where("id = ? AND tenant_id = ?", row.ID, row.TenantID).Delete(&Template{}).Error; err != nil {
		return err
	}
	s.log(c, adminID, "waybill_template.delete", row.ID.String(), fmt.Sprintf("name=%s", row.Name))
	return nil
}

func (s *Service) findTemplateScoped(c *gin.Context, id uuid.UUID) (*Template, error) {
	if s == nil || s.DB == nil {
		return nil, fmt.Errorf("waybill: no db")
	}
	tid, err := adminperm.TenantIDFromGin(c)
	if err != nil {
		return nil, err
	}
	var row Template
	if err := s.DB.WithContext(c.Request.Context()).
		First(&row, "id = ? AND tenant_id = ?", id, tid).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrTemplateNotFound
		}
		return nil, err
	}
	return &row, nil
}

// DefaultTemplate returns the tenant default template (seeding presets first),
// falling back to the first template by sort order.
func (s *Service) DefaultTemplate(c *gin.Context) (*Template, error) {
	if s == nil || s.DB == nil {
		return nil, fmt.Errorf("waybill: no db")
	}
	tid, err := adminperm.TenantIDFromGin(c)
	if err != nil {
		return nil, err
	}
	if err := EnsureTemplatePresets(c.Request.Context(), s.DB, tid); err != nil {
		return nil, err
	}
	var row Template
	err = s.DB.WithContext(c.Request.Context()).
		Where("tenant_id = ? AND is_default = ?", tid, true).First(&row).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		err = s.DB.WithContext(c.Request.Context()).
			Where("tenant_id = ?", tid).
			Order("sort_order ASC, created_at ASC").First(&row).Error
	}
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, ErrTemplateNotFound
	}
	if err != nil {
		return nil, err
	}
	return &row, nil
}

func boolOr(p *bool, def bool) bool {
	if p != nil {
		return *p
	}
	return def
}

func intOr(p *int, def int) int {
	if p != nil {
		return *p
	}
	return def
}

func strOr(p *string) string {
	if p != nil {
		return strings.TrimSpace(*p)
	}
	return ""
}

func (s *Service) log(c *gin.Context, adminID *uuid.UUID, action, resourceID, msg string) {
	if s == nil || s.OpLog == nil {
		return
	}
	_ = s.OpLog.Write(c, operationlog.WriteOpts{
		AdminUserID: adminID,
		Action:      action,
		Resource:    "waybill",
		ResourceID:  resourceID,
		Status:      "success",
		Message:     msg,
	})
}
