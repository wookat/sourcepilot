package order

import (
	"errors"
	"fmt"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/trademind-ai/trademind/backend/internal/modules/operationlog"
	"github.com/trademind-ai/trademind/backend/internal/pkg/adminperm"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// ErrOrderTagNotFound is returned for missing / cross-tenant tags (404).
var ErrOrderTagNotFound = errors.New("order tag not found")

// orderTagBatchLimit caps one批量打标 request.
const orderTagBatchLimit = 200

// validOrderTagColors are the antd tag color tokens the admin UI offers.
var validOrderTagColors = []string{
	"blue", "green", "red", "orange", "gold", "purple", "cyan", "magenta", "geekblue", "volcano", "lime", "default",
}

// OrderTagBody is the create / update payload for a tag.
type OrderTagBody struct {
	Name  string `json:"name"`
	Color string `json:"color"`
}

func validateOrderTagBody(row *OrderTag, body OrderTagBody, isCreate bool) error {
	if name := strings.TrimSpace(body.Name); name != "" {
		if len([]rune(name)) > 32 {
			return fmt.Errorf("标签名称最长 32 个字符")
		}
		row.Name = name
	}
	if isCreate && strings.TrimSpace(row.Name) == "" {
		return fmt.Errorf("标签名称不能为空")
	}
	if color := strings.TrimSpace(body.Color); color != "" {
		if !listContains(validOrderTagColors, color) {
			return fmt.Errorf("无效的标签颜色：%s", color)
		}
		row.Color = color
	}
	if row.Color == "" {
		row.Color = "blue"
	}
	return nil
}

// ListOrderTags returns tenant tags ordered by name.
func (s *Service) ListOrderTags(c *gin.Context) ([]OrderTag, error) {
	if s == nil || s.DB == nil {
		return nil, fmt.Errorf("order: no db")
	}
	tid, err := adminperm.TenantIDFromGin(c)
	if err != nil {
		return nil, err
	}
	var rows []OrderTag
	if err := s.DB.WithContext(c.Request.Context()).
		Where("tenant_id = ?", tid).
		Order("name ASC, created_at ASC").Find(&rows).Error; err != nil {
		return nil, err
	}
	if rows == nil {
		rows = []OrderTag{}
	}
	return rows, nil
}

// CreateOrderTag adds a tag in the current tenant (名称租户内唯一).
func (s *Service) CreateOrderTag(c *gin.Context, body OrderTagBody, adminID *uuid.UUID) (*OrderTag, error) {
	if s == nil || s.DB == nil {
		return nil, fmt.Errorf("order: no db")
	}
	tid, err := adminperm.TenantIDFromGin(c)
	if err != nil {
		return nil, err
	}
	row := OrderTag{TenantID: tid}
	if err := validateOrderTagBody(&row, body, true); err != nil {
		return nil, err
	}
	if err := s.ensureTagNameFree(c, tid, row.Name, uuid.Nil); err != nil {
		return nil, err
	}
	if err := s.DB.WithContext(c.Request.Context()).Create(&row).Error; err != nil {
		return nil, err
	}
	s.logOrderTag(c, adminID, "order_tag.create", row.ID.String(), fmt.Sprintf("标签「%s」（颜色：%s）", row.Name, row.Color))
	return &row, nil
}

// UpdateOrderTag edits a tag in the current tenant.
func (s *Service) UpdateOrderTag(c *gin.Context, id uuid.UUID, body OrderTagBody, adminID *uuid.UUID) (*OrderTag, error) {
	row, err := s.findOrderTagScoped(c, id)
	if err != nil {
		return nil, err
	}
	if err := validateOrderTagBody(row, body, false); err != nil {
		return nil, err
	}
	if err := s.ensureTagNameFree(c, row.TenantID, row.Name, row.ID); err != nil {
		return nil, err
	}
	if err := s.DB.WithContext(c.Request.Context()).Save(row).Error; err != nil {
		return nil, err
	}
	s.logOrderTag(c, adminID, "order_tag.update", row.ID.String(), fmt.Sprintf("标签「%s」（颜色：%s）", row.Name, row.Color))
	return row, nil
}

// DeleteOrderTag removes a tag and all its order links in the current tenant.
func (s *Service) DeleteOrderTag(c *gin.Context, id uuid.UUID, adminID *uuid.UUID) error {
	row, err := s.findOrderTagScoped(c, id)
	if err != nil {
		return err
	}
	if err := s.DB.WithContext(c.Request.Context()).Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("tag_id = ? AND tenant_id = ?", row.ID, row.TenantID).
			Delete(&OrderTagLink{}).Error; err != nil {
			return err
		}
		return tx.Where("id = ? AND tenant_id = ?", row.ID, row.TenantID).
			Delete(&OrderTag{}).Error
	}); err != nil {
		return err
	}
	s.logOrderTag(c, adminID, "order_tag.delete", row.ID.String(), fmt.Sprintf("标签「%s」（含订单打标记录）", row.Name))
	return nil
}

func (s *Service) findOrderTagScoped(c *gin.Context, id uuid.UUID) (*OrderTag, error) {
	if s == nil || s.DB == nil {
		return nil, fmt.Errorf("order: no db")
	}
	tid, err := adminperm.TenantIDFromGin(c)
	if err != nil {
		return nil, err
	}
	var row OrderTag
	if err := s.DB.WithContext(c.Request.Context()).
		First(&row, "id = ? AND tenant_id = ?", id, tid).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrOrderTagNotFound
		}
		return nil, err
	}
	return &row, nil
}

func (s *Service) ensureTagNameFree(c *gin.Context, tid int64, name string, selfID uuid.UUID) error {
	var n int64
	tx := s.DB.WithContext(c.Request.Context()).Model(&OrderTag{}).
		Where("tenant_id = ? AND name = ?", tid, name)
	if selfID != uuid.Nil {
		tx = tx.Where("id <> ?", selfID)
	}
	if err := tx.Count(&n).Error; err != nil {
		return err
	}
	if n > 0 {
		return fmt.Errorf("标签「%s」已存在", name)
	}
	return nil
}

// OrderTagOpBody is the payload for打标/去标 (单订单与批量共用).
type OrderTagOpBody struct {
	TagIDs []string `json:"tagIds"`
}

// BatchOrderTagBody is the批量打标/去标 payload.
type BatchOrderTagBody struct {
	OrderIDs []string `json:"orderIds"`
	TagIDs   []string `json:"tagIds"`
	// Action: add（默认）或 remove.
	Action string `json:"action"`
}

// BatchOrderTagResult reports how many links were actually written / removed
// (重复打标不计数，幂等).
type BatchOrderTagResult struct {
	Orders  int   `json:"orders"`
	Tags    int   `json:"tags"`
	Applied int64 `json:"applied"`
	Removed int64 `json:"removed"`
}

// AddOrderTags attaches tags to one order (幂等：已有链接跳过).
func (s *Service) AddOrderTags(c *gin.Context, orderID uuid.UUID, tagIDs []string, adminID *uuid.UUID) ([]OrderTagBrief, error) {
	o, err := s.findOrderBare(c, orderID)
	if err != nil {
		return nil, err
	}
	tags, err := s.resolveTenantTags(c, o.TenantID, tagIDs)
	if err != nil {
		return nil, err
	}
	if _, err := s.attachOrderTags(c, o.TenantID, []uuid.UUID{o.ID}, tags, TagLinkSourceManual); err != nil {
		return nil, err
	}
	s.logOrderTag(c, adminID, "order_tag.attach", o.ID.String(),
		fmt.Sprintf("订单 %s 打标签：%s", o.OrderNo, joinTagNames(tags)))
	return s.orderTagBriefs(c, o.TenantID, o.ID)
}

// RemoveOrderTag detaches one tag from one order.
func (s *Service) RemoveOrderTag(c *gin.Context, orderID, tagID uuid.UUID, adminID *uuid.UUID) ([]OrderTagBrief, error) {
	o, err := s.findOrderBare(c, orderID)
	if err != nil {
		return nil, err
	}
	tag, err := s.findOrderTagScoped(c, tagID)
	if err != nil {
		return nil, err
	}
	if err := s.DB.WithContext(c.Request.Context()).
		Where("tenant_id = ? AND order_id = ? AND tag_id = ?", o.TenantID, o.ID, tag.ID).
		Delete(&OrderTagLink{}).Error; err != nil {
		return nil, err
	}
	s.logOrderTag(c, adminID, "order_tag.detach", o.ID.String(),
		fmt.Sprintf("订单 %s 移除标签：%s", o.OrderNo, tag.Name))
	return s.orderTagBriefs(c, o.TenantID, o.ID)
}

// BatchTagOrders attaches / removes tags on multiple orders (tenant + store
// scope enforced; 重复提交幂等).
func (s *Service) BatchTagOrders(c *gin.Context, body BatchOrderTagBody, adminID *uuid.UUID) (*BatchOrderTagResult, error) {
	if s == nil || s.DB == nil {
		return nil, fmt.Errorf("order: no db")
	}
	action := strings.TrimSpace(body.Action)
	if action == "" {
		action = "add"
	}
	if action != "add" && action != "remove" {
		return nil, fmt.Errorf("无效的批量打标动作：%s", action)
	}
	if len(body.OrderIDs) == 0 {
		return nil, fmt.Errorf("请选择订单")
	}
	if len(body.OrderIDs) > orderTagBatchLimit {
		return nil, fmt.Errorf("单次批量打标最多 %d 个订单", orderTagBatchLimit)
	}
	tid, err := adminperm.TenantIDFromGin(c)
	if err != nil {
		return nil, err
	}
	orderIDs := make([]uuid.UUID, 0, len(body.OrderIDs))
	for _, raw := range body.OrderIDs {
		id, err := uuid.Parse(strings.TrimSpace(raw))
		if err != nil {
			return nil, fmt.Errorf("无效的订单 ID：%s", raw)
		}
		orderIDs = append(orderIDs, id)
	}
	tags, err := s.resolveTenantTags(c, tid, body.TagIDs)
	if err != nil {
		return nil, err
	}
	// Scope check: every requested order must be visible in tenant + store
	// scope, otherwise the whole batch is rejected (no partial cross-scope writes).
	tx := s.DB.WithContext(c.Request.Context()).Model(&Order{}).
		Where("tenant_id = ? AND id IN ?", tid, orderIDs)
	tx, err = adminperm.ApplyStoreScope(c, s.DB, tx, "shop_id")
	if err != nil {
		return nil, err
	}
	var visible int64
	if err := tx.Count(&visible).Error; err != nil {
		return nil, err
	}
	if visible != int64(len(orderIDs)) {
		return nil, gorm.ErrRecordNotFound
	}
	res := &BatchOrderTagResult{Orders: len(orderIDs), Tags: len(tags)}
	if action == "add" {
		applied, err := s.attachOrderTags(c, tid, orderIDs, tags, TagLinkSourceManual)
		if err != nil {
			return nil, err
		}
		res.Applied = applied
	} else {
		tagIDs := make([]uuid.UUID, len(tags))
		for i, t := range tags {
			tagIDs[i] = t.ID
		}
		del := s.DB.WithContext(c.Request.Context()).
			Where("tenant_id = ? AND order_id IN ? AND tag_id IN ?", tid, orderIDs, tagIDs).
			Delete(&OrderTagLink{})
		if del.Error != nil {
			return nil, del.Error
		}
		res.Removed = del.RowsAffected
	}
	s.logOrderTag(c, adminID, "order_tag.batch", "",
		fmt.Sprintf("批量%s：%d 个订单 × 标签 %s", batchTagActionLabel(action), len(orderIDs), joinTagNames(tags)))
	return res, nil
}

func batchTagActionLabel(action string) string {
	if action == "remove" {
		return "去标"
	}
	return "打标"
}

// resolveTenantTags loads tags by id and rejects missing / cross-tenant ids.
func (s *Service) resolveTenantTags(c *gin.Context, tid int64, raw []string) ([]OrderTag, error) {
	if len(raw) == 0 {
		return nil, fmt.Errorf("请选择标签")
	}
	seen := map[uuid.UUID]bool{}
	ids := make([]uuid.UUID, 0, len(raw))
	for _, r := range raw {
		id, err := uuid.Parse(strings.TrimSpace(r))
		if err != nil {
			return nil, fmt.Errorf("无效的标签 ID：%s", r)
		}
		if !seen[id] {
			seen[id] = true
			ids = append(ids, id)
		}
	}
	var tags []OrderTag
	if err := s.DB.WithContext(c.Request.Context()).
		Where("tenant_id = ? AND id IN ?", tid, ids).Find(&tags).Error; err != nil {
		return nil, err
	}
	if len(tags) != len(ids) {
		return nil, ErrOrderTagNotFound
	}
	return tags, nil
}

// attachOrderTags inserts links, skipping existing ones (ON CONFLICT DO
// NOTHING on the order+tag unique index → 幂等).
func (s *Service) attachOrderTags(c *gin.Context, tid int64, orderIDs []uuid.UUID, tags []OrderTag, source string) (int64, error) {
	links := make([]OrderTagLink, 0, len(orderIDs)*len(tags))
	for _, oid := range orderIDs {
		for _, t := range tags {
			links = append(links, OrderTagLink{TenantID: tid, OrderID: oid, TagID: t.ID, Source: source})
		}
	}
	if len(links) == 0 {
		return 0, nil
	}
	res := s.DB.WithContext(c.Request.Context()).Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "order_id"}, {Name: "tag_id"}},
		DoNothing: true,
	}).Create(&links)
	if res.Error != nil {
		return 0, res.Error
	}
	return res.RowsAffected, nil
}

// orderTagBriefs returns the order's current tags for handler responses.
func (s *Service) orderTagBriefs(c *gin.Context, tid int64, orderID uuid.UUID) ([]OrderTagBrief, error) {
	var rows []OrderTagBrief
	if err := s.DB.WithContext(c.Request.Context()).Raw(`
		SELECT t.id, t.name, t.color
		FROM order_tag_links l
		JOIN order_tags t ON t.id = l.tag_id
		WHERE l.tenant_id = ? AND l.order_id = ?
		ORDER BY t.name ASC
	`, tid, orderID).Scan(&rows).Error; err != nil {
		return nil, err
	}
	if rows == nil {
		rows = []OrderTagBrief{}
	}
	return rows, nil
}

func joinTagNames(tags []OrderTag) string {
	names := make([]string, len(tags))
	for i, t := range tags {
		names[i] = "「" + t.Name + "」"
	}
	return strings.Join(names, "、")
}

func (s *Service) logOrderTag(c *gin.Context, adminID *uuid.UUID, action, resourceID, msg string) {
	if s == nil || s.OpLog == nil {
		return
	}
	_ = s.OpLog.Write(c, operationlog.WriteOpts{
		AdminUserID: adminID,
		Action:      action,
		Resource:    "order_tag",
		ResourceID:  resourceID,
		Status:      "success",
		Message:     msg,
	})
}
