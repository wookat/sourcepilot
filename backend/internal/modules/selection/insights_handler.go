package selection

import (
	"errors"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"github.com/trademind-ai/trademind/backend/internal/pkg/adminperm"
	"github.com/trademind-ai/trademind/backend/internal/pkg/response"
	"github.com/trademind-ai/trademind/backend/internal/providers/markettrend"
)

// GetCandidateInsights GET /api/v1/selection/candidates/:id/insights
func (h *Handler) GetCandidateInsights(c *gin.Context) {
	if h == nil || h.Svc == nil {
		response.Fail(c, http.StatusInternalServerError, response.CodeInternalError, "selection unavailable")
		return
	}
	id, ok := parseIDParam(c)
	if !ok {
		return
	}
	tenantID, err := adminperm.TenantIDFromGin(c)
	if err != nil {
		response.Fail(c, http.StatusForbidden, response.CodeForbidden, "tenant context missing")
		return
	}
	out, err := h.Svc.CandidateInsights(c.Request.Context(), tenantID, id)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			response.Fail(c, http.StatusNotFound, response.CodeNotFound, "candidate not found")
			return
		}
		response.Fail(c, http.StatusInternalServerError, response.CodeInternalError, err.Error())
		return
	}
	response.OK(c, out)
}

// GetCandidatePriceTrend GET /api/v1/selection/candidates/:id/price-trend
func (h *Handler) GetCandidatePriceTrend(c *gin.Context) {
	if h == nil || h.Svc == nil {
		response.Fail(c, http.StatusInternalServerError, response.CodeInternalError, "selection unavailable")
		return
	}
	id, ok := parseIDParam(c)
	if !ok {
		return
	}
	tenantID, err := adminperm.TenantIDFromGin(c)
	if err != nil {
		response.Fail(c, http.StatusForbidden, response.CodeForbidden, "tenant context missing")
		return
	}
	out, err := h.Svc.CandidatePriceTrend(c.Request.Context(), tenantID, id)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			response.Fail(c, http.StatusNotFound, response.CodeNotFound, "candidate not found")
			return
		}
		response.Fail(c, http.StatusInternalServerError, response.CodeInternalError, err.Error())
		return
	}
	response.OK(c, out)
}

// CompareCandidates GET /api/v1/selection/compare?ids=a,b,c
func (h *Handler) CompareCandidates(c *gin.Context) {
	if h == nil || h.Svc == nil {
		response.Fail(c, http.StatusInternalServerError, response.CodeInternalError, "selection unavailable")
		return
	}
	tenantID, err := adminperm.TenantIDFromGin(c)
	if err != nil {
		response.Fail(c, http.StatusForbidden, response.CodeForbidden, "tenant context missing")
		return
	}
	raw := strings.Split(c.Query("ids"), ",")
	ids := make([]uuid.UUID, 0, len(raw))
	for _, part := range raw {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		id, err := uuid.Parse(part)
		if err != nil {
			response.Fail(c, http.StatusBadRequest, response.CodeBadRequest, "invalid ids")
			return
		}
		ids = append(ids, id)
	}
	out, err := h.Svc.CompareCandidates(c.Request.Context(), tenantID, ids)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			response.Fail(c, http.StatusNotFound, response.CodeNotFound, "candidate not found")
			return
		}
		response.Fail(c, http.StatusBadRequest, response.CodeBadRequest, err.Error())
		return
	}
	response.OK(c, out)
}

// GetMarketSources GET /api/v1/selection/market-sources
func (h *Handler) GetMarketSources(c *gin.Context) {
	if h == nil || h.Svc == nil {
		response.Fail(c, http.StatusInternalServerError, response.CodeInternalError, "selection unavailable")
		return
	}
	if _, err := adminperm.TenantIDFromGin(c); err != nil {
		response.Fail(c, http.StatusForbidden, response.CodeForbidden, "tenant context missing")
		return
	}
	if h.Svc.Trend == nil {
		response.OK(c, []markettrend.SourceStatus{})
		return
	}
	response.OK(c, h.Svc.Trend.Status(c.Request.Context()))
}
