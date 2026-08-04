package waybill

import (
	"errors"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/trademind-ai/trademind/backend/internal/pkg/ctxkey"
	"github.com/trademind-ai/trademind/backend/internal/pkg/response"
)

// Handler serves /waybill-templates and /shipping-rules routes.
type Handler struct {
	Svc *Service
}

func adminUUID(c *gin.Context) *uuid.UUID {
	if v, ok := c.Get(ctxkey.AdminID); ok {
		if s, ok := v.(string); ok {
			if u, err := uuid.Parse(strings.TrimSpace(s)); err == nil {
				return &u
			}
		}
	}
	return nil
}

func (h *Handler) ok() bool { return h != nil && h.Svc != nil }

func parseID(c *gin.Context) (uuid.UUID, bool) {
	id, err := uuid.Parse(strings.TrimSpace(c.Param("id")))
	if err != nil {
		response.Fail(c, http.StatusBadRequest, response.CodeBadRequest, "invalid id")
		return uuid.Nil, false
	}
	return id, true
}

// ListTemplates GET /waybill-templates
func (h *Handler) ListTemplates(c *gin.Context) {
	if !h.ok() {
		response.Fail(c, http.StatusInternalServerError, response.CodeInternalError, "waybill unavailable")
		return
	}
	rows, err := h.Svc.ListTemplates(c)
	if err != nil {
		response.HandleError(c, err)
		return
	}
	response.OK(c, gin.H{"items": rows})
}

// CreateTemplate POST /waybill-templates
func (h *Handler) CreateTemplate(c *gin.Context) {
	if !h.ok() {
		response.Fail(c, http.StatusInternalServerError, response.CodeInternalError, "waybill unavailable")
		return
	}
	var body TemplateBody
	if err := c.ShouldBindJSON(&body); err != nil {
		response.Fail(c, http.StatusBadRequest, response.CodeBadRequest, "invalid json body")
		return
	}
	row, err := h.Svc.CreateTemplate(c, body, adminUUID(c))
	if err != nil {
		response.Fail(c, http.StatusBadRequest, response.CodeBadRequest, err.Error())
		return
	}
	response.OK(c, row)
}

// UpdateTemplate PUT /waybill-templates/:id
func (h *Handler) UpdateTemplate(c *gin.Context) {
	if !h.ok() {
		response.Fail(c, http.StatusInternalServerError, response.CodeInternalError, "waybill unavailable")
		return
	}
	id, ok := parseID(c)
	if !ok {
		return
	}
	var body TemplateBody
	if err := c.ShouldBindJSON(&body); err != nil {
		response.Fail(c, http.StatusBadRequest, response.CodeBadRequest, "invalid json body")
		return
	}
	row, err := h.Svc.UpdateTemplate(c, id, body, adminUUID(c))
	if err != nil {
		if errors.Is(err, ErrTemplateNotFound) {
			response.Fail(c, http.StatusNotFound, response.CodeNotFound, "面单模板不存在")
			return
		}
		response.Fail(c, http.StatusBadRequest, response.CodeBadRequest, err.Error())
		return
	}
	response.OK(c, row)
}

// DeleteTemplate DELETE /waybill-templates/:id
func (h *Handler) DeleteTemplate(c *gin.Context) {
	if !h.ok() {
		response.Fail(c, http.StatusInternalServerError, response.CodeInternalError, "waybill unavailable")
		return
	}
	id, ok := parseID(c)
	if !ok {
		return
	}
	if err := h.Svc.DeleteTemplate(c, id, adminUUID(c)); err != nil {
		if errors.Is(err, ErrTemplateNotFound) {
			response.Fail(c, http.StatusNotFound, response.CodeNotFound, "面单模板不存在")
			return
		}
		response.Fail(c, http.StatusBadRequest, response.CodeBadRequest, err.Error())
		return
	}
	response.OK(c, gin.H{"ok": true})
}

// ListRules GET /shipping-rules
func (h *Handler) ListRules(c *gin.Context) {
	if !h.ok() {
		response.Fail(c, http.StatusInternalServerError, response.CodeInternalError, "waybill unavailable")
		return
	}
	rows, err := h.Svc.ListRules(c)
	if err != nil {
		response.HandleError(c, err)
		return
	}
	response.OK(c, gin.H{"items": rows})
}

// CreateRule POST /shipping-rules
func (h *Handler) CreateRule(c *gin.Context) {
	if !h.ok() {
		response.Fail(c, http.StatusInternalServerError, response.CodeInternalError, "waybill unavailable")
		return
	}
	var body RuleBody
	if err := c.ShouldBindJSON(&body); err != nil {
		response.Fail(c, http.StatusBadRequest, response.CodeBadRequest, "invalid json body")
		return
	}
	row, err := h.Svc.CreateRule(c, body, adminUUID(c))
	if err != nil {
		response.Fail(c, http.StatusBadRequest, response.CodeBadRequest, err.Error())
		return
	}
	response.OK(c, row)
}

// UpdateRule PUT /shipping-rules/:id
func (h *Handler) UpdateRule(c *gin.Context) {
	if !h.ok() {
		response.Fail(c, http.StatusInternalServerError, response.CodeInternalError, "waybill unavailable")
		return
	}
	id, ok := parseID(c)
	if !ok {
		return
	}
	var body RuleBody
	if err := c.ShouldBindJSON(&body); err != nil {
		response.Fail(c, http.StatusBadRequest, response.CodeBadRequest, "invalid json body")
		return
	}
	row, err := h.Svc.UpdateRule(c, id, body, adminUUID(c))
	if err != nil {
		if errors.Is(err, ErrRuleNotFound) {
			response.Fail(c, http.StatusNotFound, response.CodeNotFound, "发货规则不存在")
			return
		}
		response.Fail(c, http.StatusBadRequest, response.CodeBadRequest, err.Error())
		return
	}
	response.OK(c, row)
}

// DeleteRule DELETE /shipping-rules/:id
func (h *Handler) DeleteRule(c *gin.Context) {
	if !h.ok() {
		response.Fail(c, http.StatusInternalServerError, response.CodeInternalError, "waybill unavailable")
		return
	}
	id, ok := parseID(c)
	if !ok {
		return
	}
	if err := h.Svc.DeleteRule(c, id, adminUUID(c)); err != nil {
		if errors.Is(err, ErrRuleNotFound) {
			response.Fail(c, http.StatusNotFound, response.CodeNotFound, "发货规则不存在")
			return
		}
		response.Fail(c, http.StatusBadRequest, response.CodeBadRequest, err.Error())
		return
	}
	response.OK(c, gin.H{"ok": true})
}

// RecommendBody POST /shipping-rules/recommend.
type RecommendBody struct {
	Province string   `json:"province"`
	Platform string   `json:"platform"`
	WeightKg *float64 `json:"weightKg"`
	Amount   *float64 `json:"amount"`
}

// Recommend POST /shipping-rules/recommend evaluates rules for raw attrs
// (rule testing panel; order flows use /orders/shipping-recommendations).
func (h *Handler) Recommend(c *gin.Context) {
	if !h.ok() {
		response.Fail(c, http.StatusInternalServerError, response.CodeInternalError, "waybill unavailable")
		return
	}
	var body RecommendBody
	if err := c.ShouldBindJSON(&body); err != nil {
		response.Fail(c, http.StatusBadRequest, response.CodeBadRequest, "invalid json body")
		return
	}
	rec, err := h.Svc.Recommend(c, MatchAttrs{
		Province: body.Province,
		Platform: body.Platform,
		WeightKg: body.WeightKg,
		Amount:   body.Amount,
	})
	if err != nil {
		response.HandleError(c, err)
		return
	}
	response.OK(c, rec)
}
