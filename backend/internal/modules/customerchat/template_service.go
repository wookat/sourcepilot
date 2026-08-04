package customerchat

import (
	"fmt"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/trademind-ai/trademind/backend/internal/modules/operationlog"
	"github.com/trademind-ai/trademind/backend/internal/pkg/adminperm"
	"gorm.io/gorm"
)

const templateContentMaxLen = 4000

// TemplateListQuery filters GET /customer/reply-templates.
type TemplateListQuery struct {
	GroupKey string
	Keyword  string
	Enabled  *bool
}

// ListTemplates returns tenant-scoped templates ordered by group, sort order, creation time.
func (s *Service) ListTemplates(c *gin.Context, q TemplateListQuery) ([]TemplateRow, error) {
	tid, err := adminperm.TenantIDFromGin(c)
	if err != nil {
		return nil, err
	}
	db := s.DB.WithContext(c.Request.Context()).Model(&CustomerReplyTemplate{}).
		Where("tenant_id = ?", tid)
	if g := strings.TrimSpace(q.GroupKey); g != "" {
		db = db.Where("group_key = ?", g)
	}
	if kw := strings.TrimSpace(q.Keyword); kw != "" {
		like := "%" + kw + "%"
		db = db.Where("name LIKE ? OR content LIKE ?", like, like)
	}
	if q.Enabled != nil {
		db = db.Where("enabled = ?", *q.Enabled)
	}
	var rows []CustomerReplyTemplate
	if err := db.Order("group_key ASC, sort_order ASC, created_at ASC").Find(&rows).Error; err != nil {
		return nil, err
	}
	out := make([]TemplateRow, 0, len(rows))
	for _, r := range rows {
		out = append(out, toTemplateRow(r))
	}
	return out, nil
}

// TemplateUpsertBody binds create/update payloads.
type TemplateUpsertBody struct {
	GroupKey  string `json:"groupKey"`
	Name      string `json:"name"`
	Content   string `json:"content"`
	SortOrder *int   `json:"sortOrder"`
	Enabled   *bool  `json:"enabled"`
}

func validateTemplateBody(groupKey, name, content string) error {
	if !IsValidTemplateGroup(groupKey) {
		return fmt.Errorf("分组不合法，可选值：%s", strings.Join(TemplateGroups, "/"))
	}
	if name == "" {
		return fmt.Errorf("模板名称不能为空")
	}
	if content == "" {
		return fmt.Errorf("模板内容不能为空")
	}
	if len(content) > templateContentMaxLen {
		return fmt.Errorf("模板内容过长（最多 %d 字符）", templateContentMaxLen)
	}
	return nil
}

// CreateTemplate inserts one tenant-scoped template.
func (s *Service) CreateTemplate(c *gin.Context, body TemplateUpsertBody, adminID *uuid.UUID) (*TemplateRow, error) {
	tid, err := adminperm.TenantIDFromGin(c)
	if err != nil {
		return nil, err
	}
	groupKey := strings.TrimSpace(body.GroupKey)
	name := strings.TrimSpace(body.Name)
	content := strings.TrimSpace(body.Content)
	if err := validateTemplateBody(groupKey, name, content); err != nil {
		return nil, err
	}
	row := &CustomerReplyTemplate{
		TenantID:  tid,
		GroupKey:  groupKey,
		Name:      name,
		Content:   content,
		Enabled:   true,
		CreatedBy: adminID,
	}
	if body.SortOrder != nil {
		row.SortOrder = *body.SortOrder
	} else {
		var maxOrder int
		s.DB.WithContext(c.Request.Context()).Model(&CustomerReplyTemplate{}).
			Where("tenant_id = ? AND group_key = ?", tid, groupKey).
			Select("COALESCE(MAX(sort_order), 0)").Scan(&maxOrder)
		row.SortOrder = maxOrder + 1
	}
	if body.Enabled != nil {
		row.Enabled = *body.Enabled
	}
	if err := s.DB.WithContext(c.Request.Context()).Create(row).Error; err != nil {
		return nil, err
	}
	s.writeTemplateOpLog(c, adminID, "customer.reply_template.create", row.ID.String(), row.Name)
	out := toTemplateRow(*row)
	return &out, nil
}

// UpdateTemplate patches one tenant-scoped template.
func (s *Service) UpdateTemplate(c *gin.Context, id uuid.UUID, body TemplateUpsertBody, adminID *uuid.UUID) (*TemplateRow, error) {
	tid, err := adminperm.TenantIDFromGin(c)
	if err != nil {
		return nil, err
	}
	var row CustomerReplyTemplate
	if err := s.DB.WithContext(c.Request.Context()).
		Where("id = ? AND tenant_id = ?", id, tid).First(&row).Error; err != nil {
		return nil, err
	}
	updates := map[string]any{}
	groupKey := row.GroupKey
	if g := strings.TrimSpace(body.GroupKey); g != "" {
		groupKey = g
		updates["group_key"] = g
	}
	name := row.Name
	if n := strings.TrimSpace(body.Name); n != "" {
		name = n
		updates["name"] = n
	}
	content := row.Content
	if ct := strings.TrimSpace(body.Content); ct != "" {
		content = ct
		updates["content"] = ct
	}
	if err := validateTemplateBody(groupKey, name, content); err != nil {
		return nil, err
	}
	if body.SortOrder != nil {
		updates["sort_order"] = *body.SortOrder
	}
	if body.Enabled != nil {
		updates["enabled"] = *body.Enabled
	}
	if len(updates) > 0 {
		if err := s.DB.WithContext(c.Request.Context()).Model(&row).Updates(updates).Error; err != nil {
			return nil, err
		}
	}
	if err := s.DB.WithContext(c.Request.Context()).
		Where("id = ? AND tenant_id = ?", id, tid).First(&row).Error; err != nil {
		return nil, err
	}
	s.writeTemplateOpLog(c, adminID, "customer.reply_template.update", row.ID.String(), row.Name)
	out := toTemplateRow(row)
	return &out, nil
}

// DeleteTemplate soft-deletes one tenant-scoped template.
func (s *Service) DeleteTemplate(c *gin.Context, id uuid.UUID, adminID *uuid.UUID) error {
	tid, err := adminperm.TenantIDFromGin(c)
	if err != nil {
		return err
	}
	var row CustomerReplyTemplate
	if err := s.DB.WithContext(c.Request.Context()).
		Where("id = ? AND tenant_id = ?", id, tid).First(&row).Error; err != nil {
		return err
	}
	if err := s.DB.WithContext(c.Request.Context()).Delete(&row).Error; err != nil {
		return err
	}
	s.writeTemplateOpLog(c, adminID, "customer.reply_template.delete", row.ID.String(), row.Name)
	return nil
}

// ReorderTemplatesBody binds POST /customer/reply-templates/reorder.
type ReorderTemplatesBody struct {
	GroupKey string   `json:"groupKey"`
	IDs      []string `json:"ids"`
}

// ReorderTemplates rewrites sort_order for the given ids (1-based, in order)
// within one group of the caller's tenant.
func (s *Service) ReorderTemplates(c *gin.Context, body ReorderTemplatesBody) error {
	tid, err := adminperm.TenantIDFromGin(c)
	if err != nil {
		return err
	}
	groupKey := strings.TrimSpace(body.GroupKey)
	if !IsValidTemplateGroup(groupKey) {
		return fmt.Errorf("分组不合法，可选值：%s", strings.Join(TemplateGroups, "/"))
	}
	if len(body.IDs) == 0 {
		return fmt.Errorf("ids 不能为空")
	}
	ids := make([]uuid.UUID, 0, len(body.IDs))
	for _, raw := range body.IDs {
		u, err := uuid.Parse(strings.TrimSpace(raw))
		if err != nil {
			return fmt.Errorf("非法模板 ID: %s", raw)
		}
		ids = append(ids, u)
	}
	return s.DB.WithContext(c.Request.Context()).Transaction(func(tx *gorm.DB) error {
		var count int64
		if err := tx.Model(&CustomerReplyTemplate{}).
			Where("tenant_id = ? AND group_key = ? AND id IN ?", tid, groupKey, ids).
			Count(&count).Error; err != nil {
			return err
		}
		if count != int64(len(ids)) {
			return fmt.Errorf("部分模板不存在或不属于该分组")
		}
		for i, id := range ids {
			if err := tx.Model(&CustomerReplyTemplate{}).
				Where("id = ? AND tenant_id = ?", id, tid).
				Update("sort_order", i+1).Error; err != nil {
				return err
			}
		}
		return nil
	})
}

func (s *Service) writeTemplateOpLog(c *gin.Context, adminID *uuid.UUID, action, targetID, targetName string) {
	if s.OpLog == nil {
		return
	}
	_ = s.OpLog.Write(c, operationlog.WriteOpts{
		AdminUserID: adminID,
		Action:      action,
		Resource:    "customer_reply_template",
		ResourceID:  targetID,
		Status:      "success",
		Message:     fmt.Sprintf("templateId=%s name=%s", targetID, targetName),
	})
}
