package order

import (
	"errors"
	"net/http"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/trademind-ai/trademind/backend/internal/pkg/response"
)

func parseReviewRuleID(c *gin.Context) (uuid.UUID, bool) {
	id, err := uuid.Parse(strings.TrimSpace(c.Param("ruleId")))
	if err != nil {
		response.Fail(c, http.StatusBadRequest, response.CodeBadRequest, "invalid rule id")
		return uuid.Nil, false
	}
	return id, true
}

// GetReviewRules GET /order-review-rules
func (h *Handler) GetReviewRules(c *gin.Context) {
	rows, err := h.Svc.ListReviewRules(c)
	if err != nil {
		response.HandleError(c, err)
		return
	}
	response.OK(c, gin.H{"items": rows})
}

// PostReviewRule POST /order-review-rules
func (h *Handler) PostReviewRule(c *gin.Context) {
	var body ReviewRuleBody
	if err := c.ShouldBindJSON(&body); err != nil {
		response.Fail(c, http.StatusBadRequest, response.CodeBadRequest, "invalid json body")
		return
	}
	row, err := h.Svc.CreateReviewRule(c, body, adminUUID(c))
	if err != nil {
		if failRuleShopScope(c, err) {
			return
		}
		response.Fail(c, http.StatusBadRequest, response.CodeBadRequest, err.Error())
		return
	}
	response.OK(c, row)
}

// PutReviewRule PUT /order-review-rules/:ruleId
func (h *Handler) PutReviewRule(c *gin.Context) {
	id, ok := parseReviewRuleID(c)
	if !ok {
		return
	}
	var body ReviewRuleBody
	if err := c.ShouldBindJSON(&body); err != nil {
		response.Fail(c, http.StatusBadRequest, response.CodeBadRequest, "invalid json body")
		return
	}
	row, err := h.Svc.UpdateReviewRule(c, id, body, adminUUID(c))
	if err != nil {
		if errors.Is(err, ErrReviewRuleNotFound) {
			response.Fail(c, http.StatusNotFound, response.CodeNotFound, "审单规则不存在")
			return
		}
		if failRuleShopScope(c, err) {
			return
		}
		response.Fail(c, http.StatusBadRequest, response.CodeBadRequest, err.Error())
		return
	}
	response.OK(c, row)
}

// DeleteReviewRule DELETE /order-review-rules/:ruleId
func (h *Handler) DeleteReviewRule(c *gin.Context) {
	id, ok := parseReviewRuleID(c)
	if !ok {
		return
	}
	if err := h.Svc.DeleteReviewRule(c, id, adminUUID(c)); err != nil {
		if errors.Is(err, ErrReviewRuleNotFound) {
			response.Fail(c, http.StatusNotFound, response.CodeNotFound, "审单规则不存在")
			return
		}
		response.Fail(c, http.StatusBadRequest, response.CodeBadRequest, err.Error())
		return
	}
	response.OK(c, gin.H{"ok": true})
}

// PostReviewRuleDryRun POST /order-review-rules/dry-run
func (h *Handler) PostReviewRuleDryRun(c *gin.Context) {
	var body ReviewRuleBody
	if err := c.ShouldBindJSON(&body); err != nil {
		response.Fail(c, http.StatusBadRequest, response.CodeBadRequest, "invalid json body")
		return
	}
	res, err := h.Svc.DryRunReviewRule(c, body)
	if err != nil {
		response.Fail(c, http.StatusBadRequest, response.CodeBadRequest, err.Error())
		return
	}
	response.OK(c, res)
}

// GetReviewWorkbench GET /order-review
func (h *Handler) GetReviewWorkbench(c *gin.Context) {
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	ps, _ := strconv.Atoi(c.DefaultQuery("pageSize", "20"))
	res, err := h.Svc.ListReviewWorkbench(c, ReviewWorkbenchQuery{
		Page:         page,
		PageSize:     ps,
		ReviewStatus: strings.TrimSpace(c.Query("reviewStatus")),
		Keyword:      strings.TrimSpace(c.Query("keyword")),
	})
	if err != nil {
		response.Fail(c, http.StatusBadRequest, response.CodeBadRequest, err.Error())
		return
	}
	response.OK(c, res)
}

// PostReviewApprove POST /order-review/approve（单个/批量放行）
func (h *Handler) PostReviewApprove(c *gin.Context) {
	var body ReviewDecisionBody
	if err := c.ShouldBindJSON(&body); err != nil {
		response.Fail(c, http.StatusBadRequest, response.CodeBadRequest, "invalid json body")
		return
	}
	res, err := h.Svc.ApproveReviewOrders(c, body, adminUUID(c))
	if err != nil {
		response.Fail(c, http.StatusBadRequest, response.CodeBadRequest, err.Error())
		return
	}
	response.OK(c, res)
}

// PostReviewReject POST /order-review/reject（拒绝，入取消动线）
func (h *Handler) PostReviewReject(c *gin.Context) {
	var body ReviewDecisionBody
	if err := c.ShouldBindJSON(&body); err != nil {
		response.Fail(c, http.StatusBadRequest, response.CodeBadRequest, "invalid json body")
		return
	}
	res, err := h.Svc.RejectReviewOrders(c, body, adminUUID(c))
	if err != nil {
		response.Fail(c, http.StatusBadRequest, response.CodeBadRequest, err.Error())
		return
	}
	response.OK(c, res)
}
