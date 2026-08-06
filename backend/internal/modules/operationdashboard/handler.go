package operationdashboard

import (
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/trademind-ai/trademind/backend/internal/modules/reports"
	"github.com/trademind-ai/trademind/backend/internal/pkg/response"
)

// Handler serves dashboard HTTP API. Reports (optional) attaches today's
// sales / profit KPIs to the big-screen endpoint with the same #276 SQL
// pushdown口径 as /reports/profit.
type Handler struct {
	Svc     *Service
	Reports *reports.Service
}

func parseRFC3339Dashboard(s string) (*time.Time, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return nil, nil
	}
	t, err := time.Parse(time.RFC3339, s)
	if err != nil {
		return nil, err
	}
	return &t, nil
}

func (h *Handler) bindQuery(c *gin.Context) (Query, error) {
	startPtr, err := parseRFC3339Dashboard(c.Query("start"))
	if err != nil {
		return Query{}, errStartTime
	}
	endPtr, err := parseRFC3339Dashboard(c.Query("end"))
	if err != nil {
		return Query{}, errEndTime
	}
	shopID := strings.TrimSpace(c.Query("shopId"))
	if shopID != "" {
		if _, perr := uuid.Parse(shopID); perr != nil {
			return Query{}, errShopID
		}
	}
	sc := scopeFromContext(c, h.Svc.DB)
	return Query{
		Start:    startPtr,
		End:      endPtr,
		Platform: c.Query("platform"),
		ShopID:   shopID,
		Source:   c.Query("source"),
		Scope:    sc,
	}, nil
}

var (
	errStartTime = &parseTimeErr{field: "start"}
	errEndTime   = &parseTimeErr{field: "end"}
	errShopID    = errors.New("invalid shopId (must be a UUID)")
)

type parseTimeErr struct{ field string }

func (e *parseTimeErr) Error() string {
	return "invalid " + e.field + " time (RFC3339)"
}

// ProductOperations GET /api/v1/dashboard/product-operations
func (h *Handler) ProductOperations(c *gin.Context) {
	if h == nil || h.Svc == nil {
		response.Fail(c, 500, response.CodeInternalError, "dashboard unavailable")
		return
	}
	q, err := h.bindQuery(c)
	if err != nil {
		response.Fail(c, http.StatusBadRequest, response.CodeBadRequest, err.Error())
		return
	}
	out, err := h.Svc.GetProductOperationDashboard(c.Request.Context(), q, q.Scope)
	if err != nil {
		response.Fail(c, http.StatusBadRequest, response.CodeBadRequest, err.Error())
		return
	}
	response.OK(c, out)
}

// Screen GET /api/v1/dashboard/screen
func (h *Handler) Screen(c *gin.Context) {
	if h == nil || h.Svc == nil {
		response.Fail(c, 500, response.CodeInternalError, "dashboard unavailable")
		return
	}
	q, err := h.bindQuery(c)
	if err != nil {
		response.Fail(c, http.StatusBadRequest, response.CodeBadRequest, err.Error())
		return
	}
	out, err := h.Svc.GetScreen(c.Request.Context(), q, q.Scope)
	if err != nil {
		response.Fail(c, http.StatusBadRequest, response.CodeBadRequest, err.Error())
		return
	}
	if h.Reports != nil {
		today, rerr := reports.ResolveRange(1, "", "")
		if rerr == nil {
			filter := reports.ProfitFilter{Platform: strings.TrimSpace(q.Platform)}
			if v := q.ShopID; v != "" {
				if u, perr := uuid.Parse(v); perr == nil {
					filter.ShopID = &u
				}
			}
			if rep, perr := h.Reports.ProfitReportFiltered(c, reports.DimensionShop, today, filter); perr == nil && rep != nil {
				out.Today.PaidOrderCount = rep.Summary.OrderCount
				out.Today.SalesBase = rep.Summary.RevenueBase
				out.Today.BaseCurrency = rep.BaseCurrency
				out.Today.UnconvertedCurrencies = rep.Summary.UnconvertedCurrencies
				out.Today.GrossProfitBase = rep.Summary.GrossProfitBase
				out.Today.MarginPercent = rep.Summary.MarginPercent
			}
		}
	}
	response.OK(c, out)
}

// Overview GET /api/v1/dashboard/overview
func (h *Handler) Overview(c *gin.Context) {
	if h == nil || h.Svc == nil {
		response.Fail(c, 500, response.CodeInternalError, "dashboard unavailable")
		return
	}
	q, err := h.bindQuery(c)
	if err != nil {
		response.Fail(c, http.StatusBadRequest, response.CodeBadRequest, err.Error())
		return
	}
	out, err := h.Svc.GetOverview(c.Request.Context(), q, q.Scope)
	if err != nil {
		response.Fail(c, http.StatusBadRequest, response.CodeBadRequest, err.Error())
		return
	}
	response.OK(c, out)
}

// Todos GET /api/v1/dashboard/todos
func (h *Handler) Todos(c *gin.Context) {
	if h == nil || h.Svc == nil {
		response.Fail(c, 500, response.CodeInternalError, "dashboard unavailable")
		return
	}
	q, err := h.bindQuery(c)
	if err != nil {
		response.Fail(c, http.StatusBadRequest, response.CodeBadRequest, err.Error())
		return
	}
	out, err := h.Svc.GetTodos(c.Request.Context(), q, q.Scope)
	if err != nil {
		response.Fail(c, http.StatusBadRequest, response.CodeBadRequest, err.Error())
		return
	}
	response.OK(c, out)
}

// Health GET /api/v1/dashboard/health
func (h *Handler) Health(c *gin.Context) {
	if h == nil || h.Svc == nil {
		response.Fail(c, 500, response.CodeInternalError, "dashboard unavailable")
		return
	}
	q, err := h.bindQuery(c)
	if err != nil {
		response.Fail(c, http.StatusBadRequest, response.CodeBadRequest, err.Error())
		return
	}
	out, err := h.Svc.GetHealth(c.Request.Context(), q, q.Scope)
	if err != nil {
		response.Fail(c, http.StatusBadRequest, response.CodeBadRequest, err.Error())
		return
	}
	response.OK(c, out)
}
