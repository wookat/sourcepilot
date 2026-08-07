package order

import (
	"errors"
	"net/http"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/trademind-ai/trademind/backend/internal/pkg/adminperm"
	"github.com/trademind-ai/trademind/backend/internal/pkg/response"
	"gorm.io/gorm"
)

// failRuleShopScope maps rule shopIds scope violations: unknown / cross-tenant
// / invisible shops → 404 (no existence leak), view-only shops → 403.
func failRuleShopScope(c *gin.Context, err error) bool {
	if errors.Is(err, gorm.ErrRecordNotFound) {
		response.Fail(c, http.StatusNotFound, response.CodeNotFound, "资源不存在")
		return true
	}
	if errors.Is(err, adminperm.ErrStoreNotOperable) {
		response.Fail(c, http.StatusForbidden, response.CodeStorePermissionDenied, "店铺无操作权限")
		return true
	}
	return false
}

func parseAutomationRuleID(c *gin.Context) (uuid.UUID, bool) {
	id, err := uuid.Parse(strings.TrimSpace(c.Param("ruleId")))
	if err != nil {
		response.Fail(c, http.StatusBadRequest, response.CodeBadRequest, "invalid rule id")
		return uuid.Nil, false
	}
	return id, true
}

// GetAutomationRules GET /order-automation-rules
func (h *Handler) GetAutomationRules(c *gin.Context) {
	rows, err := h.Svc.ListAutomationRules(c)
	if err != nil {
		response.HandleError(c, err)
		return
	}
	response.OK(c, gin.H{"items": rows})
}

// PostAutomationRule POST /order-automation-rules
func (h *Handler) PostAutomationRule(c *gin.Context) {
	var body AutomationRuleBody
	if err := c.ShouldBindJSON(&body); err != nil {
		response.Fail(c, http.StatusBadRequest, response.CodeBadRequest, "invalid json body")
		return
	}
	row, err := h.Svc.CreateAutomationRule(c, body, adminUUID(c))
	if err != nil {
		if failRuleShopScope(c, err) {
			return
		}
		response.Fail(c, http.StatusBadRequest, response.CodeBadRequest, err.Error())
		return
	}
	response.OK(c, row)
}

// PutAutomationRule PUT /order-automation-rules/:ruleId
func (h *Handler) PutAutomationRule(c *gin.Context) {
	id, ok := parseAutomationRuleID(c)
	if !ok {
		return
	}
	var body AutomationRuleBody
	if err := c.ShouldBindJSON(&body); err != nil {
		response.Fail(c, http.StatusBadRequest, response.CodeBadRequest, "invalid json body")
		return
	}
	row, err := h.Svc.UpdateAutomationRule(c, id, body, adminUUID(c))
	if err != nil {
		if errors.Is(err, ErrAutomationRuleNotFound) {
			response.Fail(c, http.StatusNotFound, response.CodeNotFound, "自动化规则不存在")
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

// DeleteAutomationRule DELETE /order-automation-rules/:ruleId
func (h *Handler) DeleteAutomationRule(c *gin.Context) {
	id, ok := parseAutomationRuleID(c)
	if !ok {
		return
	}
	if err := h.Svc.DeleteAutomationRule(c, id, adminUUID(c)); err != nil {
		if errors.Is(err, ErrAutomationRuleNotFound) {
			response.Fail(c, http.StatusNotFound, response.CodeNotFound, "自动化规则不存在")
			return
		}
		response.Fail(c, http.StatusBadRequest, response.CodeBadRequest, err.Error())
		return
	}
	response.OK(c, gin.H{"ok": true})
}

// PostAutomationRuleDryRun POST /order-automation-rules/dry-run
func (h *Handler) PostAutomationRuleDryRun(c *gin.Context) {
	var body AutomationRuleBody
	if err := c.ShouldBindJSON(&body); err != nil {
		response.Fail(c, http.StatusBadRequest, response.CodeBadRequest, "invalid json body")
		return
	}
	res, err := h.Svc.DryRunAutomationRule(c, body)
	if err != nil {
		response.Fail(c, http.StatusBadRequest, response.CodeBadRequest, err.Error())
		return
	}
	response.OK(c, res)
}

// GetAutomationLogs GET /order-automation-logs
func (h *Handler) GetAutomationLogs(c *gin.Context) {
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	ps, _ := strconv.Atoi(c.DefaultQuery("pageSize", "20"))
	res, err := h.Svc.ListAutomationLogs(c, AutomationLogQuery{
		Page:     page,
		PageSize: ps,
		Status:   strings.TrimSpace(c.Query("status")),
		Event:    strings.TrimSpace(c.Query("triggerEvent")),
		Action:   strings.TrimSpace(c.Query("action")),
		RuleID:   strings.TrimSpace(c.Query("ruleId")),
		Keyword:  strings.TrimSpace(c.Query("keyword")),
	})
	if err != nil {
		response.Fail(c, http.StatusBadRequest, response.CodeBadRequest, err.Error())
		return
	}
	response.OK(c, res)
}

// PostAutomationLogRetry POST /order-automation-logs/:logId/retry
func (h *Handler) PostAutomationLogRetry(c *gin.Context) {
	id, err := uuid.Parse(strings.TrimSpace(c.Param("logId")))
	if err != nil {
		response.Fail(c, http.StatusBadRequest, response.CodeBadRequest, "invalid log id")
		return
	}
	row, err := h.Svc.RetryAutomationLog(c, id, adminUUID(c))
	if err != nil {
		if errors.Is(err, ErrAutomationLogNotFound) {
			response.Fail(c, http.StatusNotFound, response.CodeNotFound, "执行日志不存在")
			return
		}
		if errors.Is(err, adminperm.ErrStoreNotOperable) {
			response.Fail(c, http.StatusForbidden, response.CodeStorePermissionDenied, "店铺无操作权限")
			return
		}
		response.Fail(c, http.StatusBadRequest, response.CodeBadRequest, err.Error())
		return
	}
	response.OK(c, row)
}

// GetOrderAutomationTrail GET /orders/:id/automation-logs
func (h *Handler) GetOrderAutomationTrail(c *gin.Context) {
	id, err := uuid.Parse(strings.TrimSpace(c.Param("id")))
	if err != nil {
		response.Fail(c, http.StatusBadRequest, response.CodeBadRequest, "invalid order id")
		return
	}
	items, err := h.Svc.ListOrderAutomationTrail(c, id)
	if err != nil {
		if errors.Is(err, ErrNotFound) || errors.Is(err, gorm.ErrRecordNotFound) {
			response.Fail(c, http.StatusNotFound, response.CodeNotFound, "订单不存在")
			return
		}
		response.HandleError(c, err)
		return
	}
	response.OK(c, gin.H{"items": items})
}
