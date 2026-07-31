package sourcing

import (
	"errors"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/trademind-ai/trademind/backend/internal/pkg/ctxkey"
	"github.com/trademind-ai/trademind/backend/internal/pkg/response"
)

// Handler exposes sourcing HTTP API.
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

func handleSourcingError(c *gin.Context, err error) {
	switch {
	case errors.Is(err, ErrNotFound):
		response.Fail(c, 404, response.CodeNotFound, err.Error())
	case errors.Is(err, ErrBadRequest):
		response.Fail(c, 400, response.CodeBadRequest, err.Error())
	case errors.Is(err, ErrConflict):
		response.Fail(c, 409, response.CodeBadRequest, err.Error())
	default:
		response.HandleError(c, err)
	}
}

func (h *Handler) ok() bool { return h != nil && h.Svc != nil }

func atoiQ(c *gin.Context, key string, def int) int {
	s := strings.TrimSpace(c.Query(key))
	if s == "" {
		return def
	}
	n, err := strconv.Atoi(s)
	if err != nil || n < 1 {
		return def
	}
	return n
}

func parseID(c *gin.Context, param string) (uuid.UUID, bool) {
	u, err := uuid.Parse(strings.TrimSpace(c.Param(param)))
	if err != nil {
		response.Fail(c, 400, response.CodeBadRequest, "invalid id")
		return uuid.Nil, false
	}
	return u, true
}

// ListSuppliers GET /suppliers
func (h *Handler) ListSuppliers(c *gin.Context) {
	if !h.ok() {
		response.Fail(c, 500, response.CodeInternalError, "sourcing unavailable")
		return
	}
	res, err := h.Svc.ListSuppliers(c.Request.Context(), SupplierListQuery{
		Page:     atoiQ(c, "page", 1),
		PageSize: atoiQ(c, "pageSize", 20),
		Keyword:  c.Query("keyword"),
		Platform: c.Query("platform"),
		Status:   c.Query("status"),
	})
	if err != nil {
		handleSourcingError(c, err)
		return
	}
	response.OK(c, res)
}

// CreateSupplier POST /suppliers
func (h *Handler) CreateSupplier(c *gin.Context) {
	if !h.ok() {
		response.Fail(c, 500, response.CodeInternalError, "sourcing unavailable")
		return
	}
	var body SupplierBody
	if err := c.ShouldBindJSON(&body); err != nil {
		response.Fail(c, 400, response.CodeBadRequest, "invalid body")
		return
	}
	out, err := h.Svc.CreateSupplier(c.Request.Context(), body, adminUUID(c))
	if err != nil {
		handleSourcingError(c, err)
		return
	}
	response.OK(c, out)
}

// UpdateSupplier PUT /suppliers/:id
func (h *Handler) UpdateSupplier(c *gin.Context) {
	if !h.ok() {
		response.Fail(c, 500, response.CodeInternalError, "sourcing unavailable")
		return
	}
	id, ok := parseID(c, "id")
	if !ok {
		return
	}
	var body SupplierBody
	if err := c.ShouldBindJSON(&body); err != nil {
		response.Fail(c, 400, response.CodeBadRequest, "invalid body")
		return
	}
	out, err := h.Svc.UpdateSupplier(c.Request.Context(), id, body, adminUUID(c))
	if err != nil {
		handleSourcingError(c, err)
		return
	}
	response.OK(c, out)
}

// DeleteSupplier DELETE /suppliers/:id
func (h *Handler) DeleteSupplier(c *gin.Context) {
	if !h.ok() {
		response.Fail(c, 500, response.CodeInternalError, "sourcing unavailable")
		return
	}
	id, ok := parseID(c, "id")
	if !ok {
		return
	}
	if err := h.Svc.DeleteSupplier(c.Request.Context(), id, adminUUID(c)); err != nil {
		handleSourcingError(c, err)
		return
	}
	response.OK(c, gin.H{"deleted": true})
}

// ListProductSources GET /products/:id/sources
func (h *Handler) ListProductSources(c *gin.Context) {
	if !h.ok() {
		response.Fail(c, 500, response.CodeInternalError, "sourcing unavailable")
		return
	}
	pid, ok := parseID(c, "id")
	if !ok {
		return
	}
	out, err := h.Svc.ListProductSources(c.Request.Context(), pid)
	if err != nil {
		handleSourcingError(c, err)
		return
	}
	response.OK(c, gin.H{"items": out})
}

// BindSource POST /products/:id/sources
func (h *Handler) BindSource(c *gin.Context) {
	if !h.ok() {
		response.Fail(c, 500, response.CodeInternalError, "sourcing unavailable")
		return
	}
	pid, ok := parseID(c, "id")
	if !ok {
		return
	}
	var body BindSourceBody
	if err := c.ShouldBindJSON(&body); err != nil {
		response.Fail(c, 400, response.CodeBadRequest, "invalid body")
		return
	}
	out, err := h.Svc.BindSource(c.Request.Context(), pid, body, adminUUID(c))
	if err != nil {
		handleSourcingError(c, err)
		return
	}
	response.OK(c, out)
}

// UpdateSource PUT /product-sources/:id
func (h *Handler) UpdateSource(c *gin.Context) {
	if !h.ok() {
		response.Fail(c, 500, response.CodeInternalError, "sourcing unavailable")
		return
	}
	id, ok := parseID(c, "id")
	if !ok {
		return
	}
	var body UpdateSourceBody
	if err := c.ShouldBindJSON(&body); err != nil {
		response.Fail(c, 400, response.CodeBadRequest, "invalid body")
		return
	}
	out, err := h.Svc.UpdateSource(c.Request.Context(), id, body, adminUUID(c))
	if err != nil {
		handleSourcingError(c, err)
		return
	}
	response.OK(c, out)
}

// SetPrimary POST /product-sources/:id/set-primary
func (h *Handler) SetPrimary(c *gin.Context) {
	if !h.ok() {
		response.Fail(c, 500, response.CodeInternalError, "sourcing unavailable")
		return
	}
	id, ok := parseID(c, "id")
	if !ok {
		return
	}
	out, err := h.Svc.SetPrimary(c.Request.Context(), id, adminUUID(c))
	if err != nil {
		handleSourcingError(c, err)
		return
	}
	response.OK(c, out)
}

// SaveSKUMappings POST /product-sources/:id/sku-mappings
func (h *Handler) SaveSKUMappings(c *gin.Context) {
	if !h.ok() {
		response.Fail(c, 500, response.CodeInternalError, "sourcing unavailable")
		return
	}
	id, ok := parseID(c, "id")
	if !ok {
		return
	}
	var body struct {
		Mappings []SKUMappingBody `json:"mappings"`
	}
	if err := c.ShouldBindJSON(&body); err != nil || len(body.Mappings) == 0 {
		response.Fail(c, 400, response.CodeBadRequest, "invalid body: mappings required")
		return
	}
	out, err := h.Svc.SaveSKUMappings(c.Request.Context(), id, body.Mappings, adminUUID(c))
	if err != nil {
		handleSourcingError(c, err)
		return
	}
	response.OK(c, gin.H{"items": out})
}

// DeleteSKUMapping DELETE /product-source-skus/:id
func (h *Handler) DeleteSKUMapping(c *gin.Context) {
	if !h.ok() {
		response.Fail(c, 500, response.CodeInternalError, "sourcing unavailable")
		return
	}
	id, ok := parseID(c, "id")
	if !ok {
		return
	}
	if err := h.Svc.DeleteSKUMapping(c.Request.Context(), id, adminUUID(c)); err != nil {
		handleSourcingError(c, err)
		return
	}
	response.OK(c, gin.H{"deleted": true})
}

// PriceHistory GET /product-source-skus/:id/price-history?days=90
func (h *Handler) PriceHistory(c *gin.Context) {
	if !h.ok() {
		response.Fail(c, 500, response.CodeInternalError, "sourcing unavailable")
		return
	}
	id, ok := parseID(c, "id")
	if !ok {
		return
	}
	out, err := h.Svc.PriceHistory(c.Request.Context(), id, atoiQ(c, "days", 90))
	if err != nil {
		handleSourcingError(c, err)
		return
	}
	response.OK(c, gin.H{"items": out})
}

// Refresh POST /products/:id/sources/refresh
func (h *Handler) Refresh(c *gin.Context) {
	if !h.ok() {
		response.Fail(c, 500, response.CodeInternalError, "sourcing unavailable")
		return
	}
	pid, ok := parseID(c, "id")
	if !ok {
		return
	}
	out, err := h.Svc.RefreshProductSources(c.Request.Context(), pid, adminUUID(c))
	if err != nil {
		handleSourcingError(c, err)
		return
	}
	response.OK(c, out)
}

// ListSwitchEvents GET /source-switch-events?productId=
func (h *Handler) ListSwitchEvents(c *gin.Context) {
	if !h.ok() {
		response.Fail(c, 500, response.CodeInternalError, "sourcing unavailable")
		return
	}
	var pid *uuid.UUID
	if raw := strings.TrimSpace(c.Query("productId")); raw != "" {
		u, err := uuid.Parse(raw)
		if err != nil {
			response.Fail(c, 400, response.CodeBadRequest, "invalid productId")
			return
		}
		pid = &u
	}
	out, err := h.Svc.ListSwitchEvents(c.Request.Context(), pid, atoiQ(c, "page", 1), atoiQ(c, "pageSize", 20))
	if err != nil {
		handleSourcingError(c, err)
		return
	}
	response.OK(c, out)
}
