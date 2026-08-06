package customerchat

import (
	"errors"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/trademind-ai/trademind/backend/internal/pkg/adminperm"
	"github.com/trademind-ai/trademind/backend/internal/pkg/response"
	"gorm.io/gorm"
)

func (h *Handler) buyerMsgUnavailable(c *gin.Context) bool {
	if h == nil || h.Svc == nil || h.Svc.DB == nil {
		response.Fail(c, 500, response.CodeInternalError, "customer chat unavailable")
		return true
	}
	return false
}

func (h *Handler) buyerMsgDenyWrite(c *gin.Context, action string) bool {
	if !adminperm.CanWriteCustomer(c, h.Svc.DB) {
		response.Fail(c, 403, response.CodeForbidden, "readonly 账号不可"+action)
		return true
	}
	return false
}

func buyerMsgHandleErr(c *gin.Context, err error) {
	if errors.Is(err, ErrBuyerMsgRuleNotFound) || errors.Is(err, ErrBuyerMsgDraftNotFound) ||
		errors.Is(err, gorm.ErrRecordNotFound) {
		response.Fail(c, 404, response.CodeNotFound, "not found")
		return
	}
	if errors.Is(err, adminperm.ErrStoreNotOperable) {
		response.Fail(c, 403, response.CodeForbidden, "店铺无操作权限")
		return
	}
	response.Fail(c, 400, response.CodeBadRequest, err.Error())
}

// ListBuyerMsgRules GET /api/v1/customer/buyer-message-rules
func (h *Handler) ListBuyerMsgRules(c *gin.Context) {
	if h.buyerMsgUnavailable(c) {
		return
	}
	rows, err := h.Svc.ListBuyerMsgRules(c)
	if err != nil {
		response.HandleError(c, err)
		return
	}
	response.OK(c, gin.H{
		"list":     rows,
		"canWrite": adminperm.CanWriteCustomer(c, h.Svc.DB),
	})
}

// CreateBuyerMsgRule POST /api/v1/customer/buyer-message-rules
func (h *Handler) CreateBuyerMsgRule(c *gin.Context) {
	if h.buyerMsgUnavailable(c) || h.buyerMsgDenyWrite(c, "创建自动消息规则") {
		return
	}
	var body BuyerMsgRuleBody
	if err := c.ShouldBindJSON(&body); err != nil {
		response.Fail(c, 400, response.CodeBadRequest, "invalid json body")
		return
	}
	out, err := h.Svc.CreateBuyerMsgRule(c, body, adminUUID(c))
	if err != nil {
		buyerMsgHandleErr(c, err)
		return
	}
	response.OK(c, out)
}

// UpdateBuyerMsgRule PUT /api/v1/customer/buyer-message-rules/:id
func (h *Handler) UpdateBuyerMsgRule(c *gin.Context) {
	if h.buyerMsgUnavailable(c) || h.buyerMsgDenyWrite(c, "编辑自动消息规则") {
		return
	}
	id, err := uuid.Parse(strings.TrimSpace(c.Param("id")))
	if err != nil {
		response.Fail(c, 400, response.CodeBadRequest, "invalid id")
		return
	}
	var body BuyerMsgRuleBody
	if err := c.ShouldBindJSON(&body); err != nil {
		response.Fail(c, 400, response.CodeBadRequest, "invalid json body")
		return
	}
	out, err := h.Svc.UpdateBuyerMsgRule(c, id, body, adminUUID(c))
	if err != nil {
		buyerMsgHandleErr(c, err)
		return
	}
	response.OK(c, out)
}

// DeleteBuyerMsgRule DELETE /api/v1/customer/buyer-message-rules/:id
func (h *Handler) DeleteBuyerMsgRule(c *gin.Context) {
	if h.buyerMsgUnavailable(c) || h.buyerMsgDenyWrite(c, "删除自动消息规则") {
		return
	}
	id, err := uuid.Parse(strings.TrimSpace(c.Param("id")))
	if err != nil {
		response.Fail(c, 400, response.CodeBadRequest, "invalid id")
		return
	}
	if err := h.Svc.DeleteBuyerMsgRule(c, id, adminUUID(c)); err != nil {
		buyerMsgHandleErr(c, err)
		return
	}
	response.OK(c, gin.H{"ok": true})
}

// EstimateBuyerMsgBackfill GET /api/v1/customer/buyer-message-rules/backfill-estimate
// 返回「回溯存量」开启时将生成的草稿数量预估（node 必填；platforms / shopIds
// 为逗号分隔的可选过滤）。只读接口，不产生任何草稿。
func (h *Handler) EstimateBuyerMsgBackfill(c *gin.Context) {
	if h.buyerMsgUnavailable(c) {
		return
	}
	node := strings.TrimSpace(c.Query("node"))
	estimated, err := h.Svc.EstimateBuyerMsgBackfill(c, node,
		splitCSVQuery(c.Query("platforms")), splitCSVQuery(c.Query("shopIds")))
	if err != nil {
		buyerMsgHandleErr(c, err)
		return
	}
	response.OK(c, gin.H{"estimated": estimated})
}

func splitCSVQuery(raw string) []string {
	out := []string{}
	for _, v := range strings.Split(raw, ",") {
		if t := strings.TrimSpace(v); t != "" {
			out = append(out, t)
		}
	}
	return out
}

// GenerateBuyerMsgDrafts POST /api/v1/customer/buyer-messages/generate
func (h *Handler) GenerateBuyerMsgDrafts(c *gin.Context) {
	if h.buyerMsgUnavailable(c) || h.buyerMsgDenyWrite(c, "触发草稿生成") {
		return
	}
	tid, err := adminperm.TenantIDFromGin(c)
	if err != nil {
		response.HandleError(c, err)
		return
	}
	created, err := h.Svc.GenerateBuyerMsgDrafts(c.Request.Context(), tid)
	if err != nil {
		response.HandleError(c, err)
		return
	}
	response.OK(c, gin.H{"created": created})
}

// ListBuyerMsgDrafts GET /api/v1/customer/buyer-messages/drafts
func (h *Handler) ListBuyerMsgDrafts(c *gin.Context) {
	if h.buyerMsgUnavailable(c) {
		return
	}
	q := BuyerMsgDraftQuery{
		Node:     c.Query("node"),
		Status:   c.Query("status"),
		Platform: c.Query("platform"),
		Keyword:  c.Query("keyword"),
	}
	q.Page, _ = strconv.Atoi(c.Query("page"))
	q.PageSize, _ = strconv.Atoi(c.Query("pageSize"))
	if raw := strings.TrimSpace(c.Query("shopId")); raw != "" {
		id, err := uuid.Parse(raw)
		if err != nil {
			response.Fail(c, 400, response.CodeBadRequest, "invalid shopId")
			return
		}
		q.ShopID = &id
	}
	out, err := h.Svc.ListBuyerMsgDrafts(c, q)
	if err != nil {
		buyerMsgHandleErr(c, err)
		return
	}
	response.OK(c, out)
}

// BuyerMsgDraftUpdateBody binds PUT /customer/buyer-messages/drafts/:id.
type BuyerMsgDraftUpdateBody struct {
	Content string `json:"content"`
}

// UpdateBuyerMsgDraft PUT /api/v1/customer/buyer-messages/drafts/:id
func (h *Handler) UpdateBuyerMsgDraft(c *gin.Context) {
	if h.buyerMsgUnavailable(c) || h.buyerMsgDenyWrite(c, "编辑待发消息") {
		return
	}
	id, err := uuid.Parse(strings.TrimSpace(c.Param("id")))
	if err != nil {
		response.Fail(c, 400, response.CodeBadRequest, "invalid id")
		return
	}
	var body BuyerMsgDraftUpdateBody
	if err := c.ShouldBindJSON(&body); err != nil {
		response.Fail(c, 400, response.CodeBadRequest, "invalid json body")
		return
	}
	out, err := h.Svc.UpdateBuyerMsgDraft(c, id, body.Content, adminUUID(c))
	if err != nil {
		buyerMsgHandleErr(c, err)
		return
	}
	response.OK(c, out)
}

// MarkBuyerMsgDraftSent POST /api/v1/customer/buyer-messages/drafts/:id/mark-sent
func (h *Handler) MarkBuyerMsgDraftSent(c *gin.Context) {
	if h.buyerMsgUnavailable(c) || h.buyerMsgDenyWrite(c, "标记已发送") {
		return
	}
	id, err := uuid.Parse(strings.TrimSpace(c.Param("id")))
	if err != nil {
		response.Fail(c, 400, response.CodeBadRequest, "invalid id")
		return
	}
	out, err := h.Svc.MarkBuyerMsgDraftSent(c, id, adminUUID(c))
	if err != nil {
		buyerMsgHandleErr(c, err)
		return
	}
	response.OK(c, out)
}

// IgnoreBuyerMsgDraft POST /api/v1/customer/buyer-messages/drafts/:id/ignore
func (h *Handler) IgnoreBuyerMsgDraft(c *gin.Context) {
	if h.buyerMsgUnavailable(c) || h.buyerMsgDenyWrite(c, "忽略待发消息") {
		return
	}
	id, err := uuid.Parse(strings.TrimSpace(c.Param("id")))
	if err != nil {
		response.Fail(c, 400, response.CodeBadRequest, "invalid id")
		return
	}
	out, err := h.Svc.IgnoreBuyerMsgDraft(c, id, adminUUID(c))
	if err != nil {
		buyerMsgHandleErr(c, err)
		return
	}
	response.OK(c, out)
}

// BuyerMsgBatchMarkSentBody binds POST /customer/buyer-messages/drafts/batch-mark-sent.
type BuyerMsgBatchMarkSentBody struct {
	IDs []string `json:"ids"`
}

// BatchMarkBuyerMsgDraftsSent POST /api/v1/customer/buyer-messages/drafts/batch-mark-sent
func (h *Handler) BatchMarkBuyerMsgDraftsSent(c *gin.Context) {
	if h.buyerMsgUnavailable(c) || h.buyerMsgDenyWrite(c, "批量标记已发送") {
		return
	}
	var body BuyerMsgBatchMarkSentBody
	if err := c.ShouldBindJSON(&body); err != nil {
		response.Fail(c, 400, response.CodeBadRequest, "invalid json body")
		return
	}
	ids := make([]uuid.UUID, 0, len(body.IDs))
	for _, raw := range body.IDs {
		id, err := uuid.Parse(strings.TrimSpace(raw))
		if err != nil {
			response.Fail(c, 400, response.CodeBadRequest, "非法草稿 ID: "+raw)
			return
		}
		ids = append(ids, id)
	}
	out, err := h.Svc.BatchMarkBuyerMsgDraftsSent(c, ids, adminUUID(c))
	if err != nil {
		buyerMsgHandleErr(c, err)
		return
	}
	response.OK(c, out)
}
