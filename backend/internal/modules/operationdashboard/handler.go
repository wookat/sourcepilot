package operationdashboard

import (
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/trademind-ai/trademind/backend/internal/modules/operationlog"
	"github.com/trademind-ai/trademind/backend/internal/modules/reports"
	"github.com/trademind-ai/trademind/backend/internal/modules/settings"
	"github.com/trademind-ai/trademind/backend/internal/pkg/adminperm"
	"github.com/trademind-ai/trademind/backend/internal/pkg/response"
)

// Handler serves dashboard HTTP API. Reports (optional) attaches today's
// sales / profit KPIs to the big-screen endpoint with the same #276 SQL
// pushdown口径 as /reports/profit. Settings (optional) stores the tenant
// big-screen card layout; OpLog (optional) audits layout updates.
type Handler struct {
	Svc      *Service
	Reports  *reports.Service
	Settings *settings.Service
	OpLog    *operationlog.Service
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
	sc := scopeFromContext(c, h.Svc.DB)
	return Query{
		Start:    startPtr,
		End:      endPtr,
		Platform: c.Query("platform"),
		ShopID:   c.Query("shopId"),
		Source:   c.Query("source"),
		Scope:    sc,
	}, nil
}

var (
	errStartTime = &parseTimeErr{field: "start"}
	errEndTime   = &parseTimeErr{field: "end"}
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
	cards := defaultScreenCards()
	if tenantID, terr := adminperm.TenantIDFromGin(c); terr == nil {
		cards = h.loadScreenCards(c.Request.Context(), tenantID)
	}
	enabled := enabledCardSet(cards)
	out, err := h.Svc.GetScreen(c.Request.Context(), q, q.Scope, enabled)
	if err != nil {
		response.Fail(c, http.StatusBadRequest, response.CodeBadRequest, err.Error())
		return
	}
	out.Cards = cards
	if h.Reports != nil && (enabled[CardKPIOrders] || enabled[CardKPISales] || enabled[CardKPIProfit]) {
		today, rerr := reports.ResolveRange(1, "", "")
		if rerr == nil {
			if rep, perr := h.Reports.ProfitReport(c, reports.DimensionShop, today); perr == nil && rep != nil {
				out.Today.PaidOrderCount = rep.Summary.OrderCount
				out.Today.SalesBase = rep.Summary.RevenueBase
				out.Today.BaseCurrency = rep.BaseCurrency
				out.Today.UnconvertedCurrencies = rep.Summary.UnconvertedCurrencies
				out.Today.GrossProfitBase = rep.Summary.GrossProfitBase
				out.Today.MarginPercent = rep.Summary.MarginPercent
				for _, m := range rep.Summary.Revenue {
					if m.BaseAmount == nil {
						out.Today.UnconvertedRevenue = append(out.Today.UnconvertedRevenue,
							ScreenMoneyDTO{Currency: m.Currency, Amount: m.Amount})
					} else if m.Currency != rep.BaseCurrency {
						out.Today.ConvertedCurrencies = append(out.Today.ConvertedCurrencies, m.Currency)
					}
				}
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
