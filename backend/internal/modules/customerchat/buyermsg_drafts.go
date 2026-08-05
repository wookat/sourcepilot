package customerchat

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"gorm.io/datatypes"

	"github.com/trademind-ai/trademind/backend/internal/pkg/adminperm"
)

// ErrBuyerMsgDraftNotFound is returned for missing / cross-tenant drafts (404).
var ErrBuyerMsgDraftNotFound = errors.New("buyer message draft not found")

// BuyerMsgDraftQuery filters GET /customer/buyer-messages/drafts.
type BuyerMsgDraftQuery struct {
	Page     int
	PageSize int
	Node     string
	Status   string
	Platform string
	ShopID   *uuid.UUID
	Keyword  string
}

// BuyerMsgDraftRow is the API shape for one draft.
type BuyerMsgDraftRow struct {
	ID             uuid.UUID  `json:"id"`
	OrderID        uuid.UUID  `json:"orderId"`
	OrderNo        string     `json:"orderNo"`
	CustomerName   string     `json:"customerName"`
	Node           string     `json:"node"`
	RuleID         uuid.UUID  `json:"ruleId"`
	TemplateID     uuid.UUID  `json:"templateId"`
	TemplateName   string     `json:"templateName"`
	Platform       string     `json:"platform"`
	ShopID         *uuid.UUID `json:"shopId,omitempty"`
	ShopName       string     `json:"shopName,omitempty"`
	Content        string     `json:"content"`
	MissingVars    []string   `json:"missingVars"`
	Status         string     `json:"status"`
	ConversationID *uuid.UUID `json:"conversationId,omitempty"`
	SentAt         *time.Time `json:"sentAt,omitempty"`
	IgnoredAt      *time.Time `json:"ignoredAt,omitempty"`
	CreatedAt      time.Time  `json:"createdAt"`
	UpdatedAt      time.Time  `json:"updatedAt"`
}

// BuyerMsgDraftList paginates drafts.
type BuyerMsgDraftList struct {
	List     []BuyerMsgDraftRow `json:"list"`
	Total    int64              `json:"total"`
	Page     int                `json:"page"`
	PageSize int                `json:"pageSize"`
	CanWrite bool               `json:"canWrite"`
}

func (s *Service) buyerMsgDraftRow(d BuyerMessageDraft, shopNames map[uuid.UUID]string) BuyerMsgDraftRow {
	row := BuyerMsgDraftRow{
		ID:             d.ID,
		OrderID:        d.OrderID,
		OrderNo:        d.OrderNo,
		CustomerName:   d.CustomerName,
		Node:           d.Node,
		RuleID:         d.RuleID,
		TemplateID:     d.TemplateID,
		TemplateName:   d.TemplateName,
		Platform:       d.Platform,
		ShopID:         d.ShopID,
		Content:        d.Content,
		MissingVars:    jsonToStrings(d.MissingVars),
		Status:         d.Status,
		ConversationID: d.ConversationID,
		SentAt:         d.SentAt,
		IgnoredAt:      d.IgnoredAt,
		CreatedAt:      d.CreatedAt,
		UpdatedAt:      d.UpdatedAt,
	}
	if d.ShopID != nil {
		row.ShopName = shopNames[*d.ShopID]
	}
	return row
}

// ListBuyerMsgDrafts lists tenant drafts with node / shop / status filters.
func (s *Service) ListBuyerMsgDrafts(c *gin.Context, q BuyerMsgDraftQuery) (*BuyerMsgDraftList, error) {
	tid, err := adminperm.TenantIDFromGin(c)
	if err != nil {
		return nil, err
	}
	db := s.DB.WithContext(c.Request.Context()).Model(&BuyerMessageDraft{}).
		Where("tenant_id = ?", tid)
	if n := strings.TrimSpace(q.Node); n != "" {
		if !IsValidBuyerMsgNode(n) {
			return nil, fmt.Errorf("订单节点不合法，可选值：%s", strings.Join(BuyerMsgNodes, "/"))
		}
		db = db.Where("node = ?", n)
	}
	if st := strings.TrimSpace(q.Status); st != "" {
		if !IsValidBuyerMsgDraftStatus(st) {
			return nil, fmt.Errorf("状态不合法，可选值：%s", strings.Join(BuyerMsgDraftStatuses, "/"))
		}
		db = db.Where("status = ?", st)
	}
	if p := strings.TrimSpace(q.Platform); p != "" {
		db = db.Where("platform = ?", p)
	}
	if q.ShopID != nil {
		db = db.Where("shop_id = ?", *q.ShopID)
	}
	if kw := strings.TrimSpace(q.Keyword); kw != "" {
		like := "%" + kw + "%"
		db = db.Where("order_no LIKE ? OR customer_name LIKE ? OR content LIKE ?", like, like, like)
	}
	var total int64
	if err := db.Count(&total).Error; err != nil {
		return nil, err
	}
	page := q.Page
	if page < 1 {
		page = 1
	}
	pageSize := q.PageSize
	if pageSize < 1 || pageSize > 200 {
		pageSize = 20
	}
	var rows []BuyerMessageDraft
	if err := db.Order("created_at DESC").
		Offset((page - 1) * pageSize).Limit(pageSize).Find(&rows).Error; err != nil {
		return nil, err
	}
	shopIDs := make([]uuid.UUID, 0, len(rows))
	for _, r := range rows {
		if r.ShopID != nil {
			shopIDs = append(shopIDs, *r.ShopID)
		}
	}
	shopNames := s.buyerMsgShopNames(c, shopIDs)
	out := make([]BuyerMsgDraftRow, 0, len(rows))
	for _, r := range rows {
		out = append(out, s.buyerMsgDraftRow(r, shopNames))
	}
	return &BuyerMsgDraftList{
		List: out, Total: total, Page: page, PageSize: pageSize,
		CanWrite: adminperm.CanWriteCustomer(c, s.DB),
	}, nil
}

func (s *Service) buyerMsgShopNames(c *gin.Context, ids []uuid.UUID) map[uuid.UUID]string {
	out := map[uuid.UUID]string{}
	if len(ids) == 0 {
		return out
	}
	type row struct {
		ID   uuid.UUID `gorm:"column:id"`
		Name string    `gorm:"column:name"`
	}
	var rows []row
	if err := s.DB.WithContext(c.Request.Context()).Table("shops").
		Select("id, shop_name AS name").Where("id IN ?", ids).Scan(&rows).Error; err != nil {
		return out
	}
	for _, r := range rows {
		out[r.ID] = r.Name
	}
	return out
}

func (s *Service) findTenantDraft(c *gin.Context, id uuid.UUID) (*BuyerMessageDraft, error) {
	tid, err := adminperm.TenantIDFromGin(c)
	if err != nil {
		return nil, err
	}
	var row BuyerMessageDraft
	if err := s.DB.WithContext(c.Request.Context()).
		Where("id = ? AND tenant_id = ?", id, tid).First(&row).Error; err != nil {
		return nil, ErrBuyerMsgDraftNotFound
	}
	return &row, nil
}

const buyerMsgDraftContentMaxLen = 4000

// UpdateBuyerMsgDraft edits the content of one pending draft.
func (s *Service) UpdateBuyerMsgDraft(c *gin.Context, id uuid.UUID, content string, adminID *uuid.UUID) (*BuyerMsgDraftRow, error) {
	row, err := s.findTenantDraft(c, id)
	if err != nil {
		return nil, err
	}
	content = strings.TrimSpace(content)
	if content == "" {
		return nil, fmt.Errorf("消息内容不能为空")
	}
	if len(content) > buyerMsgDraftContentMaxLen {
		return nil, fmt.Errorf("消息内容过长（最多 %d 字符）", buyerMsgDraftContentMaxLen)
	}
	if row.Status != BuyerMsgDraftPending {
		return nil, fmt.Errorf("仅待发送状态的草稿可编辑")
	}
	// 重算缺失变量：编辑后仍保留的 {变量} 占位即为缺失，补全后警告消除
	_, missing := FillBuyerMsgTemplate(content, nil)
	var missingJSON datatypes.JSON
	if len(missing) > 0 {
		if b, err := json.Marshal(missing); err == nil {
			missingJSON = datatypes.JSON(b)
		}
	}
	if err := s.DB.WithContext(c.Request.Context()).Model(row).
		Updates(map[string]interface{}{"content": content, "missing_vars": missingJSON}).Error; err != nil {
		return nil, err
	}
	row.Content = content
	row.MissingVars = missingJSON
	s.writeBuyerMsgOpLog(c, adminID, "customer.buyer_message_draft.update", row.ID.String(), row.OrderNo)
	out := s.buyerMsgDraftRow(*row, s.buyerMsgShopNamesForDraft(c, row))
	return &out, nil
}

func (s *Service) buyerMsgShopNamesForDraft(c *gin.Context, row *BuyerMessageDraft) map[uuid.UUID]string {
	if row.ShopID == nil {
		return map[uuid.UUID]string{}
	}
	return s.buyerMsgShopNames(c, []uuid.UUID{*row.ShopID})
}

// MarkBuyerMsgDraftSent records the human "已在平台后台发送" receipt for one draft.
func (s *Service) MarkBuyerMsgDraftSent(c *gin.Context, id uuid.UUID, adminID *uuid.UUID) (*BuyerMsgDraftRow, error) {
	row, err := s.findTenantDraft(c, id)
	if err != nil {
		return nil, err
	}
	if row.Status == BuyerMsgDraftSent {
		out := s.buyerMsgDraftRow(*row, s.buyerMsgShopNamesForDraft(c, row))
		return &out, nil // idempotent
	}
	if row.Status != BuyerMsgDraftPending {
		return nil, fmt.Errorf("仅待发送状态的草稿可标记已发送")
	}
	now := time.Now().UTC()
	if err := s.DB.WithContext(c.Request.Context()).Model(row).Updates(map[string]any{
		"status": BuyerMsgDraftSent, "sent_at": now, "sent_by": adminID,
	}).Error; err != nil {
		return nil, err
	}
	row.Status = BuyerMsgDraftSent
	row.SentAt = &now
	row.SentBy = adminID
	s.writeBuyerMsgOpLog(c, adminID, "customer.buyer_message_draft.mark_sent", row.ID.String(), row.OrderNo)
	out := s.buyerMsgDraftRow(*row, s.buyerMsgShopNamesForDraft(c, row))
	return &out, nil
}

// IgnoreBuyerMsgDraft marks one pending draft as ignored (won't be regenerated).
func (s *Service) IgnoreBuyerMsgDraft(c *gin.Context, id uuid.UUID, adminID *uuid.UUID) (*BuyerMsgDraftRow, error) {
	row, err := s.findTenantDraft(c, id)
	if err != nil {
		return nil, err
	}
	if row.Status == BuyerMsgDraftIgnored {
		out := s.buyerMsgDraftRow(*row, s.buyerMsgShopNamesForDraft(c, row))
		return &out, nil // idempotent
	}
	if row.Status != BuyerMsgDraftPending {
		return nil, fmt.Errorf("仅待发送状态的草稿可忽略")
	}
	now := time.Now().UTC()
	if err := s.DB.WithContext(c.Request.Context()).Model(row).Updates(map[string]any{
		"status": BuyerMsgDraftIgnored, "ignored_at": now,
	}).Error; err != nil {
		return nil, err
	}
	row.Status = BuyerMsgDraftIgnored
	row.IgnoredAt = &now
	s.writeBuyerMsgOpLog(c, adminID, "customer.buyer_message_draft.ignore", row.ID.String(), row.OrderNo)
	out := s.buyerMsgDraftRow(*row, s.buyerMsgShopNamesForDraft(c, row))
	return &out, nil
}

// BatchMarkBuyerMsgSentResult reports a batch mark-sent outcome.
type BatchMarkBuyerMsgSentResult struct {
	Updated int `json:"updated"`
	Skipped int `json:"skipped"`
}

// BatchMarkBuyerMsgDraftsSent marks multiple pending drafts as sent; non-pending
// or cross-tenant ids are counted as skipped (never an error).
func (s *Service) BatchMarkBuyerMsgDraftsSent(c *gin.Context, ids []uuid.UUID, adminID *uuid.UUID) (*BatchMarkBuyerMsgSentResult, error) {
	tid, err := adminperm.TenantIDFromGin(c)
	if err != nil {
		return nil, err
	}
	if len(ids) == 0 {
		return nil, fmt.Errorf("ids 不能为空")
	}
	if len(ids) > 200 {
		return nil, fmt.Errorf("单次最多批量处理 200 条")
	}
	now := time.Now().UTC()
	res := s.DB.WithContext(c.Request.Context()).Model(&BuyerMessageDraft{}).
		Where("tenant_id = ? AND id IN ? AND status = ?", tid, ids, BuyerMsgDraftPending).
		Updates(map[string]any{"status": BuyerMsgDraftSent, "sent_at": now, "sent_by": adminID})
	if res.Error != nil {
		return nil, res.Error
	}
	updated := int(res.RowsAffected)
	s.writeBuyerMsgOpLog(c, adminID, "customer.buyer_message_draft.batch_mark_sent",
		fmt.Sprintf("count=%d", updated), "")
	return &BatchMarkBuyerMsgSentResult{Updated: updated, Skipped: len(ids) - updated}, nil
}
