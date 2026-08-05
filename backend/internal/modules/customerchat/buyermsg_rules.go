package customerchat

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/trademind-ai/trademind/backend/internal/modules/operationlog"
	"github.com/trademind-ai/trademind/backend/internal/pkg/adminperm"
	"gorm.io/datatypes"
)

// ErrBuyerMsgRuleNotFound is returned for missing / cross-tenant rules (404).
var ErrBuyerMsgRuleNotFound = errors.New("buyer message rule not found")

// BuyerMsgRuleBody binds create / update payloads for a node-message rule.
type BuyerMsgRuleBody struct {
	Name       string    `json:"name"`
	Node       string    `json:"node"`
	TemplateID string    `json:"templateId"`
	Enabled    *bool     `json:"enabled"`
	Platforms  *[]string `json:"platforms"`
	ShopIDs    *[]string `json:"shopIds"`
}

// BuyerMsgRuleRow is the API shape for one rule.
type BuyerMsgRuleRow struct {
	ID           uuid.UUID `json:"id"`
	Name         string    `json:"name"`
	Node         string    `json:"node"`
	TemplateID   uuid.UUID `json:"templateId"`
	TemplateName string    `json:"templateName"`
	// TemplateMissing marks rules whose话术模板已被删除：规则不再生成草稿，需重新选择模板
	TemplateMissing bool     `json:"templateMissing"`
	Enabled         bool     `json:"enabled"`
	Platforms       []string `json:"platforms"`
	ShopIDs         []string `json:"shopIds"`
}

func jsonToStrings(raw datatypes.JSON) []string {
	if len(raw) == 0 {
		return []string{}
	}
	var out []string
	if err := json.Unmarshal(raw, &out); err != nil {
		return []string{}
	}
	if out == nil {
		return []string{}
	}
	return out
}

func stringsToJSON(list []string) (datatypes.JSON, error) {
	cleaned := make([]string, 0, len(list))
	for _, v := range list {
		if t := strings.TrimSpace(v); t != "" {
			cleaned = append(cleaned, t)
		}
	}
	b, err := json.Marshal(cleaned)
	if err != nil {
		return nil, err
	}
	return datatypes.JSON(b), nil
}

func (s *Service) buyerMsgRuleRow(row BuyerMessageRule, templateNames map[uuid.UUID]string) BuyerMsgRuleRow {
	name, ok := templateNames[row.TemplateID]
	return BuyerMsgRuleRow{
		ID:              row.ID,
		Name:            row.Name,
		Node:            row.Node,
		TemplateID:      row.TemplateID,
		TemplateName:    name,
		TemplateMissing: !ok,
		Enabled:         row.Enabled,
		Platforms:       jsonToStrings(row.Platforms),
		ShopIDs:         jsonToStrings(row.ShopIDs),
	}
}

func (s *Service) buyerMsgTemplateNames(c *gin.Context, tid int64, ids []uuid.UUID) map[uuid.UUID]string {
	out := map[uuid.UUID]string{}
	if len(ids) == 0 {
		return out
	}
	var rows []CustomerReplyTemplate
	if err := s.DB.WithContext(c.Request.Context()).
		Where("tenant_id = ? AND id IN ?", tid, ids).Find(&rows).Error; err != nil {
		return out
	}
	for _, t := range rows {
		out[t.ID] = t.Name
	}
	return out
}

// ListBuyerMsgRules returns tenant node-message rules.
func (s *Service) ListBuyerMsgRules(c *gin.Context) ([]BuyerMsgRuleRow, error) {
	tid, err := adminperm.TenantIDFromGin(c)
	if err != nil {
		return nil, err
	}
	var rows []BuyerMessageRule
	if err := s.DB.WithContext(c.Request.Context()).
		Where("tenant_id = ?", tid).
		Order("created_at ASC").Find(&rows).Error; err != nil {
		return nil, err
	}
	ids := make([]uuid.UUID, 0, len(rows))
	for _, r := range rows {
		ids = append(ids, r.TemplateID)
	}
	names := s.buyerMsgTemplateNames(c, tid, ids)
	out := make([]BuyerMsgRuleRow, 0, len(rows))
	for _, r := range rows {
		out = append(out, s.buyerMsgRuleRow(r, names))
	}
	return out, nil
}

func (s *Service) validateBuyerMsgTemplate(c *gin.Context, tid int64, templateID uuid.UUID) error {
	var count int64
	if err := s.DB.WithContext(c.Request.Context()).Model(&CustomerReplyTemplate{}).
		Where("tenant_id = ? AND id = ?", tid, templateID).Count(&count).Error; err != nil {
		return err
	}
	if count == 0 {
		return fmt.Errorf("话术模板不存在或不属于当前租户")
	}
	return nil
}

// CreateBuyerMsgRule inserts one tenant-scoped node-message rule.
func (s *Service) CreateBuyerMsgRule(c *gin.Context, body BuyerMsgRuleBody, adminID *uuid.UUID) (*BuyerMsgRuleRow, error) {
	tid, err := adminperm.TenantIDFromGin(c)
	if err != nil {
		return nil, err
	}
	name := strings.TrimSpace(body.Name)
	if name == "" {
		return nil, fmt.Errorf("规则名称不能为空")
	}
	node := strings.TrimSpace(body.Node)
	if !IsValidBuyerMsgNode(node) {
		return nil, fmt.Errorf("订单节点不合法，可选值：%s", strings.Join(BuyerMsgNodes, "/"))
	}
	templateID, err := uuid.Parse(strings.TrimSpace(body.TemplateID))
	if err != nil {
		return nil, fmt.Errorf("请选择话术模板")
	}
	if err := s.validateBuyerMsgTemplate(c, tid, templateID); err != nil {
		return nil, err
	}
	row := &BuyerMessageRule{
		TenantID:   tid,
		Name:       name,
		Node:       node,
		TemplateID: templateID,
		Enabled:    true,
		CreatedBy:  adminID,
	}
	if body.Enabled != nil {
		row.Enabled = *body.Enabled
	}
	if body.Platforms != nil {
		if row.Platforms, err = stringsToJSON(*body.Platforms); err != nil {
			return nil, err
		}
	}
	if body.ShopIDs != nil {
		if err := adminperm.EnsureStoresOperable(c, s.DB, *body.ShopIDs); err != nil {
			return nil, err
		}
		if row.ShopIDs, err = stringsToJSON(*body.ShopIDs); err != nil {
			return nil, err
		}
	}
	if err := s.DB.WithContext(c.Request.Context()).Create(row).Error; err != nil {
		return nil, err
	}
	s.writeBuyerMsgOpLog(c, adminID, "customer.buyer_message_rule.create", row.ID.String(), row.Name)
	names := s.buyerMsgTemplateNames(c, tid, []uuid.UUID{row.TemplateID})
	out := s.buyerMsgRuleRow(*row, names)
	return &out, nil
}

// UpdateBuyerMsgRule patches one tenant-scoped rule (404 for cross-tenant).
func (s *Service) UpdateBuyerMsgRule(c *gin.Context, id uuid.UUID, body BuyerMsgRuleBody, adminID *uuid.UUID) (*BuyerMsgRuleRow, error) {
	tid, err := adminperm.TenantIDFromGin(c)
	if err != nil {
		return nil, err
	}
	var row BuyerMessageRule
	if err := s.DB.WithContext(c.Request.Context()).
		Where("id = ? AND tenant_id = ?", id, tid).First(&row).Error; err != nil {
		return nil, ErrBuyerMsgRuleNotFound
	}
	updates := map[string]any{}
	if n := strings.TrimSpace(body.Name); n != "" {
		updates["name"] = n
	}
	if nd := strings.TrimSpace(body.Node); nd != "" {
		if !IsValidBuyerMsgNode(nd) {
			return nil, fmt.Errorf("订单节点不合法，可选值：%s", strings.Join(BuyerMsgNodes, "/"))
		}
		updates["node"] = nd
	}
	if t := strings.TrimSpace(body.TemplateID); t != "" {
		templateID, err := uuid.Parse(t)
		if err != nil {
			return nil, fmt.Errorf("话术模板 ID 不合法")
		}
		if err := s.validateBuyerMsgTemplate(c, tid, templateID); err != nil {
			return nil, err
		}
		updates["template_id"] = templateID
	}
	if body.Enabled != nil {
		updates["enabled"] = *body.Enabled
	}
	if body.Platforms != nil {
		v, err := stringsToJSON(*body.Platforms)
		if err != nil {
			return nil, err
		}
		updates["platforms"] = v
	}
	if body.ShopIDs != nil {
		if err := adminperm.EnsureStoresOperable(c, s.DB, *body.ShopIDs); err != nil {
			return nil, err
		}
		v, err := stringsToJSON(*body.ShopIDs)
		if err != nil {
			return nil, err
		}
		updates["shop_ids"] = v
	}
	if len(updates) > 0 {
		if err := s.DB.WithContext(c.Request.Context()).Model(&row).Updates(updates).Error; err != nil {
			return nil, err
		}
	}
	if err := s.DB.WithContext(c.Request.Context()).
		Where("id = ? AND tenant_id = ?", id, tid).First(&row).Error; err != nil {
		return nil, ErrBuyerMsgRuleNotFound
	}
	s.writeBuyerMsgOpLog(c, adminID, "customer.buyer_message_rule.update", row.ID.String(), row.Name)
	names := s.buyerMsgTemplateNames(c, tid, []uuid.UUID{row.TemplateID})
	out := s.buyerMsgRuleRow(row, names)
	return &out, nil
}

// DeleteBuyerMsgRule soft-deletes one tenant-scoped rule (404 for cross-tenant).
func (s *Service) DeleteBuyerMsgRule(c *gin.Context, id uuid.UUID, adminID *uuid.UUID) error {
	tid, err := adminperm.TenantIDFromGin(c)
	if err != nil {
		return err
	}
	var row BuyerMessageRule
	if err := s.DB.WithContext(c.Request.Context()).
		Where("id = ? AND tenant_id = ?", id, tid).First(&row).Error; err != nil {
		return ErrBuyerMsgRuleNotFound
	}
	if err := s.DB.WithContext(c.Request.Context()).Delete(&row).Error; err != nil {
		return err
	}
	s.writeBuyerMsgOpLog(c, adminID, "customer.buyer_message_rule.delete", row.ID.String(), row.Name)
	return nil
}

func (s *Service) writeBuyerMsgOpLog(c *gin.Context, adminID *uuid.UUID, action, targetID, targetName string) {
	if s.OpLog == nil {
		return
	}
	_ = s.OpLog.Write(c, operationlog.WriteOpts{
		AdminUserID: adminID,
		Action:      action,
		Resource:    "buyer_message",
		ResourceID:  targetID,
		Status:      "success",
		Message:     fmt.Sprintf("id=%s name=%s", targetID, targetName),
	})
}
