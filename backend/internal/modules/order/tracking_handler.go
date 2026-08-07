package order

import (
	"errors"
	"fmt"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/trademind-ai/trademind/backend/internal/modules/waybill"
	"github.com/trademind-ai/trademind/backend/internal/pkg/adminperm"
	"github.com/trademind-ai/trademind/backend/internal/pkg/response"
	"github.com/trademind-ai/trademind/backend/internal/providers/tracking"
	"gorm.io/gorm"
)

// resolvePrintTemplate loads the template for ?templateId= (tenant scoped),
// falling back to the tenant default; nil when the waybill module is not
// wired (legacy print keeps working without template metadata).
func (h *Handler) resolvePrintTemplate(c *gin.Context) (*waybill.Template, error) {
	if h.Svc.Waybill == nil {
		return nil, nil
	}
	if raw := strings.TrimSpace(c.Query("templateId")); raw != "" {
		id, err := uuid.Parse(raw)
		if err != nil {
			return nil, fmt.Errorf("invalid templateId")
		}
		return h.Svc.Waybill.GetTemplate(c, id)
	}
	return h.Svc.Waybill.DefaultTemplate(c)
}

// GetPrintSheets GET /orders/print/sheets?ids=a,b,c — picking/shipping
// documents for manual label attachment (no e-waybill integration).
func (h *Handler) GetPrintSheets(c *gin.Context) {
	if h == nil || h.Svc == nil {
		response.Fail(c, http.StatusInternalServerError, response.CodeInternalError, "orders unavailable")
		return
	}
	var ids []uuid.UUID
	for _, raw := range strings.Split(c.Query("ids"), ",") {
		raw = strings.TrimSpace(raw)
		if raw == "" {
			continue
		}
		id, err := uuid.Parse(raw)
		if err != nil {
			response.Fail(c, http.StatusBadRequest, response.CodeBadRequest, "invalid ids")
			return
		}
		ids = append(ids, id)
	}
	sheets, err := h.Svc.PrintSheets(c, ids)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			response.Fail(c, http.StatusNotFound, response.CodeNotFound, "not found")
			return
		}
		response.Fail(c, http.StatusBadRequest, response.CodeBadRequest, err.Error())
		return
	}
	tpl, err := h.resolvePrintTemplate(c)
	if err != nil {
		if errors.Is(err, waybill.ErrTemplateNotFound) {
			response.Fail(c, http.StatusNotFound, response.CodeNotFound, "面单模板不存在")
			return
		}
		response.Fail(c, http.StatusBadRequest, response.CodeBadRequest, err.Error())
		return
	}
	response.OK(c, gin.H{"items": sheets, "template": tpl})
}

// PostRefreshShipmentTracking POST /orders/:id/shipments/:shipmentId/refresh-tracking.
// It goes through the TrackingProvider abstraction; the built-in manual
// provider cannot fetch remote events, so it reports supported=false and the
// operator keeps updating the shipment status by hand (existing PUT flow).
func (h *Handler) PostRefreshShipmentTracking(c *gin.Context) {
	if h == nil || h.Svc == nil {
		response.Fail(c, http.StatusInternalServerError, response.CodeInternalError, "orders unavailable")
		return
	}
	oid, err := uuid.Parse(strings.TrimSpace(c.Param("id")))
	if err != nil {
		response.Fail(c, http.StatusBadRequest, response.CodeBadRequest, "invalid id")
		return
	}
	sid, err := uuid.Parse(strings.TrimSpace(c.Param("shipmentId")))
	if err != nil {
		response.Fail(c, http.StatusBadRequest, response.CodeBadRequest, "invalid shipmentId")
		return
	}
	row, err := h.Svc.RefreshShipmentTracking(c, oid, sid, adminUUID(c))
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			response.Fail(c, http.StatusNotFound, response.CodeNotFound, "not found")
			return
		}
		if errors.Is(err, adminperm.ErrStoreNotOperable) {
			response.Fail(c, http.StatusForbidden, response.CodeStorePermissionDenied, "店铺无操作权限")
			return
		}
		response.Fail(c, http.StatusBadRequest, response.CodeBadRequest, err.Error())
		return
	}
	provider := tracking.Default()
	response.OK(c, gin.H{
		"provider":  provider.Name(),
		"supported": provider.SupportsFetch(),
		"message":   trackingRefreshMessage(provider),
		"shipment":  row,
	})
}

func trackingRefreshMessage(p tracking.Provider) string {
	if p != nil && p.SupportsFetch() {
		return "已从物流商刷新轨迹"
	}
	return "当前为手工物流模式（manual provider），请人工更新运单状态；接入真实物流轨迹 API 后此按钮将自动刷新。"
}
