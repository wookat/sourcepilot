package mcpaudit

import (
	"net/http"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/trademind-ai/trademind/backend/internal/pkg/adminperm"
	"github.com/trademind-ai/trademind/backend/internal/pkg/response"
)

// Handler serves the tenant-scoped MCP tool-call audit log.
type Handler struct {
	Svc *Service
}

// logView is the API shape of one audit row.
type logView struct {
	ID          string `json:"id"`
	TokenID     string `json:"tokenId"`
	TokenName   string `json:"tokenName"`
	TokenMasked string `json:"tokenMasked"`
	Tool        string `json:"tool"`
	Status      string `json:"status"`
	DurationMs  int64  `json:"durationMs"`
	CreatedAt   string `json:"createdAt"`
}

// List GET /mcp/audit-logs
func (h *Handler) List(c *gin.Context) {
	if h == nil || h.Svc == nil {
		response.Fail(c, http.StatusInternalServerError, response.CodeInternalError, "mcp audit unavailable")
		return
	}
	tid, err := adminperm.TenantIDFromGin(c)
	if err != nil {
		response.Fail(c, http.StatusUnauthorized, response.CodeUnauthorized, "tenant scope required")
		return
	}
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("pageSize", "20"))
	res, err := h.Svc.List(c.Request.Context(), tid, ListFilter{
		Tool:     strings.TrimSpace(c.Query("tool")),
		Status:   strings.TrimSpace(c.Query("status")),
		Page:     page,
		PageSize: pageSize,
	})
	if err != nil {
		response.HandleError(c, err)
		return
	}
	items := make([]logView, 0, len(res.Items))
	for _, r := range res.Items {
		items = append(items, logView{
			ID:          r.ID.String(),
			TokenID:     r.TokenID.String(),
			TokenName:   r.TokenName,
			TokenMasked: r.TokenMasked,
			Tool:        r.Tool,
			Status:      r.Status,
			DurationMs:  r.DurationMs,
			CreatedAt:   r.CreatedAt.UTC().Format("2006-01-02T15:04:05Z07:00"),
		})
	}
	response.OK(c, gin.H{"total": res.Total, "items": items})
}

// Register mounts the audit log read route (parent already covers /api/v1).
func Register(parent *gin.RouterGroup, h *Handler) {
	if parent == nil || h == nil {
		return
	}
	parent.GET("/mcp/audit-logs", h.List)
}
