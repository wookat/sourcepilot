package selection

import (
	"errors"
	"net/http"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"github.com/trademind-ai/trademind/backend/internal/pkg/ctxkey"
	"github.com/trademind-ai/trademind/backend/internal/pkg/response"
)

// Handler exposes the selection HTTP API.
type Handler struct {
	Svc *Service
}

func selectionAdminUUID(c *gin.Context) *uuid.UUID {
	if v, ok := c.Get(ctxkey.AdminID); ok {
		if s, ok := v.(string); ok {
			if u, err := uuid.Parse(strings.TrimSpace(s)); err == nil {
				return &u
			}
		}
	}
	return nil
}

func parseIDParam(c *gin.Context) (uuid.UUID, bool) {
	id, err := uuid.Parse(strings.TrimSpace(c.Param("id")))
	if err != nil {
		response.Fail(c, http.StatusBadRequest, response.CodeBadRequest, "invalid id")
		return uuid.Nil, false
	}
	return id, true
}

// CreateTask POST /api/v1/selection/tasks
func (h *Handler) CreateTask(c *gin.Context) {
	if h == nil || h.Svc == nil {
		response.Fail(c, http.StatusInternalServerError, response.CodeInternalError, "selection unavailable")
		return
	}
	var body CreateTaskBody
	if err := c.ShouldBindJSON(&body); err != nil {
		response.Fail(c, http.StatusBadRequest, response.CodeBadRequest, "invalid json body")
		return
	}
	task, err := h.Svc.CreateTask(c, body, selectionAdminUUID(c))
	if err != nil {
		if errors.Is(err, ErrQueueUnavailable) {
			response.Fail(c, http.StatusServiceUnavailable, response.CodeServiceUnavailable, err.Error())
			return
		}
		response.Fail(c, http.StatusBadRequest, response.CodeBadRequest, err.Error())
		return
	}
	response.OK(c, task)
}

// ListTasks GET /api/v1/selection/tasks
func (h *Handler) ListTasks(c *gin.Context) {
	if h == nil || h.Svc == nil {
		response.Fail(c, http.StatusInternalServerError, response.CodeInternalError, "selection unavailable")
		return
	}
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("pageSize", "20"))
	out, err := h.Svc.ListTasks(c.Request.Context(), page, pageSize, c.Query("status"))
	if err != nil {
		response.Fail(c, http.StatusInternalServerError, response.CodeInternalError, err.Error())
		return
	}
	response.OK(c, out)
}

// GetTask GET /api/v1/selection/tasks/:id
func (h *Handler) GetTask(c *gin.Context) {
	if h == nil || h.Svc == nil {
		response.Fail(c, http.StatusInternalServerError, response.CodeInternalError, "selection unavailable")
		return
	}
	id, ok := parseIDParam(c)
	if !ok {
		return
	}
	out, err := h.Svc.GetTask(c.Request.Context(), id)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			response.Fail(c, http.StatusNotFound, response.CodeNotFound, "task not found")
			return
		}
		response.Fail(c, http.StatusInternalServerError, response.CodeInternalError, err.Error())
		return
	}
	response.OK(c, out)
}

// ListCandidates GET /api/v1/selection/tasks/:id/candidates
func (h *Handler) ListCandidates(c *gin.Context) {
	if h == nil || h.Svc == nil {
		response.Fail(c, http.StatusInternalServerError, response.CodeInternalError, "selection unavailable")
		return
	}
	id, ok := parseIDParam(c)
	if !ok {
		return
	}
	out, err := h.Svc.ListCandidates(c.Request.Context(), id)
	if err != nil {
		response.Fail(c, http.StatusInternalServerError, response.CodeInternalError, err.Error())
		return
	}
	response.OK(c, out)
}

// Decide POST /api/v1/selection/candidates/:id/decision
func (h *Handler) Decide(c *gin.Context) {
	if h == nil || h.Svc == nil {
		response.Fail(c, http.StatusInternalServerError, response.CodeInternalError, "selection unavailable")
		return
	}
	id, ok := parseIDParam(c)
	if !ok {
		return
	}
	var body DecisionBody
	if err := c.ShouldBindJSON(&body); err != nil {
		response.Fail(c, http.StatusBadRequest, response.CodeBadRequest, "invalid json body")
		return
	}
	out, err := h.Svc.Decide(c, id, body.Decision, selectionAdminUUID(c))
	if err != nil {
		if errors.Is(err, ErrNotScored) {
			response.Fail(c, http.StatusConflict, response.CodeBadRequest, "candidate not scored yet")
			return
		}
		response.Fail(c, http.StatusBadRequest, response.CodeBadRequest, err.Error())
		return
	}
	response.OK(c, out)
}

// ToDraft POST /api/v1/selection/candidates/:id/to-draft
func (h *Handler) ToDraft(c *gin.Context) {
	if h == nil || h.Svc == nil {
		response.Fail(c, http.StatusInternalServerError, response.CodeInternalError, "selection unavailable")
		return
	}
	id, ok := parseIDParam(c)
	if !ok {
		return
	}
	out, err := h.Svc.ToDraft(c, id, selectionAdminUUID(c))
	if err != nil {
		switch {
		case errors.Is(err, ErrAlreadyDrafted):
			response.OK(c, out)
			return
		case errors.Is(err, ErrNotFound):
			response.Fail(c, http.StatusNotFound, response.CodeNotFound, "candidate not found")
			return
		case errors.Is(err, ErrNotScored), errors.Is(err, ErrNotApproved):
			response.Fail(c, http.StatusConflict, response.CodeBadRequest, err.Error())
			return
		}
		response.Fail(c, http.StatusInternalServerError, response.CodeInternalError, err.Error())
		return
	}
	response.OK(c, out)
}

// Retry POST /api/v1/selection/tasks/:id/retry
func (h *Handler) Retry(c *gin.Context) {
	if h == nil || h.Svc == nil {
		response.Fail(c, http.StatusInternalServerError, response.CodeInternalError, "selection unavailable")
		return
	}
	id, ok := parseIDParam(c)
	if !ok {
		return
	}
	out, err := h.Svc.Retry(c.Request.Context(), id)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			response.Fail(c, http.StatusNotFound, response.CodeNotFound, "task not found")
			return
		}
		if errors.Is(err, ErrQueueUnavailable) {
			response.Fail(c, http.StatusServiceUnavailable, response.CodeServiceUnavailable, err.Error())
			return
		}
		response.Fail(c, http.StatusBadRequest, response.CodeBadRequest, err.Error())
		return
	}
	response.OK(c, out)
}
