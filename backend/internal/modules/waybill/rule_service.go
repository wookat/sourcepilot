package waybill

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/trademind-ai/trademind/backend/internal/pkg/adminperm"
	"gorm.io/datatypes"
	"gorm.io/gorm"
)

// ListRules returns tenant shipping rules ordered by priority.
func (s *Service) ListRules(c *gin.Context) ([]ShippingRule, error) {
	if s == nil || s.DB == nil {
		return nil, fmt.Errorf("waybill: no db")
	}
	tid, err := adminperm.TenantIDFromGin(c)
	if err != nil {
		return nil, err
	}
	var rows []ShippingRule
	if err := s.DB.WithContext(c.Request.Context()).
		Where("tenant_id = ?", tid).
		Order("priority ASC, created_at ASC").Find(&rows).Error; err != nil {
		return nil, err
	}
	return rows, nil
}

// RuleBody is the create / update payload for a shipping rule.
type RuleBody struct {
	Name        string    `json:"name"`
	Priority    *int      `json:"priority"`
	Enabled     *bool     `json:"enabled"`
	Provinces   *[]string `json:"provinces"`
	Platforms   *[]string `json:"platforms"`
	MinWeightKg *float64  `json:"minWeightKg"`
	MaxWeightKg *float64  `json:"maxWeightKg"`
	MinAmount   *float64  `json:"minAmount"`
	MaxAmount   *float64  `json:"maxAmount"`
	CarrierCode string    `json:"carrierCode"`
}

func normalizeStrings(in []string) []string {
	out := make([]string, 0, len(in))
	for _, v := range in {
		if t := strings.TrimSpace(v); t != "" {
			out = append(out, t)
		}
	}
	return out
}

func mustJSONList(in []string) datatypes.JSON {
	b, _ := json.Marshal(in)
	return datatypes.JSON(b)
}

func (s *Service) validateRuleCarrier(c *gin.Context, code string) (string, error) {
	code = strings.ToLower(strings.TrimSpace(code))
	if code == "" {
		return "", fmt.Errorf("请选择推荐物流商")
	}
	if s.Carriers != nil {
		if _, err := s.Carriers.ResolveEnabled(c, code); err != nil {
			return "", fmt.Errorf("物流商编码无效或已停用：%s", code)
		}
	}
	return code, nil
}

func validateRange(minV, maxV *float64, label string) error {
	if minV != nil && *minV < 0 {
		return fmt.Errorf("%s下限不能为负数", label)
	}
	if maxV != nil && *maxV < 0 {
		return fmt.Errorf("%s上限不能为负数", label)
	}
	if minV != nil && maxV != nil && *minV > *maxV {
		return fmt.Errorf("%s下限不能大于上限", label)
	}
	return nil
}

// CreateRule adds a shipping rule in the current tenant.
func (s *Service) CreateRule(c *gin.Context, body RuleBody, adminID *uuid.UUID) (*ShippingRule, error) {
	if s == nil || s.DB == nil {
		return nil, fmt.Errorf("waybill: no db")
	}
	tid, err := adminperm.TenantIDFromGin(c)
	if err != nil {
		return nil, err
	}
	name := strings.TrimSpace(body.Name)
	if name == "" {
		return nil, fmt.Errorf("规则名称不能为空")
	}
	code, err := s.validateRuleCarrier(c, body.CarrierCode)
	if err != nil {
		return nil, err
	}
	if err := validateRange(body.MinWeightKg, body.MaxWeightKg, "重量"); err != nil {
		return nil, err
	}
	if err := validateRange(body.MinAmount, body.MaxAmount, "金额"); err != nil {
		return nil, err
	}
	row := ShippingRule{
		TenantID:    tid,
		Name:        name,
		Priority:    intOr(body.Priority, 0),
		Enabled:     boolOr(body.Enabled, true),
		MinWeightKg: body.MinWeightKg,
		MaxWeightKg: body.MaxWeightKg,
		MinAmount:   body.MinAmount,
		MaxAmount:   body.MaxAmount,
		CarrierCode: code,
	}
	if body.Provinces != nil {
		row.Provinces = mustJSONList(normalizeStrings(*body.Provinces))
	}
	if body.Platforms != nil {
		row.Platforms = mustJSONList(normalizeStrings(*body.Platforms))
	}
	wantEnabled := row.Enabled
	if err := s.DB.WithContext(c.Request.Context()).Create(&row).Error; err != nil {
		return nil, err
	}
	if !wantEnabled {
		// The gorm default:true tag overrides a zero-value bool on insert.
		if err := s.DB.WithContext(c.Request.Context()).Model(&ShippingRule{}).
			Where("id = ?", row.ID).Update("enabled", false).Error; err != nil {
			return nil, err
		}
		row.Enabled = false
	}
	s.log(c, adminID, "shipping_rule.create", row.ID.String(), fmt.Sprintf("name=%s carrier=%s", row.Name, row.CarrierCode))
	return &row, nil
}

// UpdateRule edits a shipping rule in the current tenant.
func (s *Service) UpdateRule(c *gin.Context, id uuid.UUID, body RuleBody, adminID *uuid.UUID) (*ShippingRule, error) {
	row, err := s.findRuleScoped(c, id)
	if err != nil {
		return nil, err
	}
	if strings.TrimSpace(body.Name) != "" {
		row.Name = strings.TrimSpace(body.Name)
	}
	if body.Priority != nil {
		row.Priority = *body.Priority
	}
	if body.Enabled != nil {
		row.Enabled = *body.Enabled
	}
	if body.Provinces != nil {
		row.Provinces = mustJSONList(normalizeStrings(*body.Provinces))
	}
	if body.Platforms != nil {
		row.Platforms = mustJSONList(normalizeStrings(*body.Platforms))
	}
	if body.MinWeightKg != nil {
		row.MinWeightKg = body.MinWeightKg
	}
	if body.MaxWeightKg != nil {
		row.MaxWeightKg = body.MaxWeightKg
	}
	if body.MinAmount != nil {
		row.MinAmount = body.MinAmount
	}
	if body.MaxAmount != nil {
		row.MaxAmount = body.MaxAmount
	}
	if err := validateRange(row.MinWeightKg, row.MaxWeightKg, "重量"); err != nil {
		return nil, err
	}
	if err := validateRange(row.MinAmount, row.MaxAmount, "金额"); err != nil {
		return nil, err
	}
	if strings.TrimSpace(body.CarrierCode) != "" {
		code, err := s.validateRuleCarrier(c, body.CarrierCode)
		if err != nil {
			return nil, err
		}
		row.CarrierCode = code
	}
	if err := s.DB.WithContext(c.Request.Context()).Save(row).Error; err != nil {
		return nil, err
	}
	s.log(c, adminID, "shipping_rule.update", row.ID.String(), fmt.Sprintf("name=%s enabled=%v", row.Name, row.Enabled))
	return row, nil
}

// DeleteRule removes a shipping rule in the current tenant.
func (s *Service) DeleteRule(c *gin.Context, id uuid.UUID, adminID *uuid.UUID) error {
	row, err := s.findRuleScoped(c, id)
	if err != nil {
		return err
	}
	if err := s.DB.WithContext(c.Request.Context()).
		Where("id = ? AND tenant_id = ?", row.ID, row.TenantID).Delete(&ShippingRule{}).Error; err != nil {
		return err
	}
	s.log(c, adminID, "shipping_rule.delete", row.ID.String(), fmt.Sprintf("name=%s", row.Name))
	return nil
}

func (s *Service) findRuleScoped(c *gin.Context, id uuid.UUID) (*ShippingRule, error) {
	if s == nil || s.DB == nil {
		return nil, fmt.Errorf("waybill: no db")
	}
	tid, err := adminperm.TenantIDFromGin(c)
	if err != nil {
		return nil, err
	}
	var row ShippingRule
	if err := s.DB.WithContext(c.Request.Context()).
		First(&row, "id = ? AND tenant_id = ?", id, tid).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrRuleNotFound
		}
		return nil, err
	}
	return &row, nil
}
