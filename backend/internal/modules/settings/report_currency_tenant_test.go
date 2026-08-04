package settings

import (
	"encoding/json"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/trademind-ai/trademind/backend/internal/pkg/ctxkey"
	"github.com/trademind-ai/trademind/backend/internal/providers/fxrate"
)

func reportCurrencyCtx(t *testing.T, tenantID int64, method, body string) (*gin.Context, *httptest.ResponseRecorder) {
	t.Helper()
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(method, "/api/v1/settings/report-currency", strings.NewReader(body))
	c.Request.Header.Set("Content-Type", "application/json")
	c.Set(ctxkey.TenantID, tenantID)
	return c, w
}

func decodeReportCurrency(t *testing.T, w *httptest.ResponseRecorder) ReportCurrencyDTO {
	t.Helper()
	var env struct {
		Data ReportCurrencyDTO `json:"data"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &env); err != nil {
		t.Fatalf("decode %s: %v", w.Body.String(), err)
	}
	return env.Data
}

// Regression (R97): report base currency / manual rates are stored per tenant.
// One tenant's configuration must never be visible to (or overwrite) another
// tenant's, and unconfigured tenants get the default base with no rates.
func TestReportCurrencyIsTenantScoped(t *testing.T) {
	svc := newSettingsTestSvc(t)
	h := &Handler{Svc: svc}

	// Tenant 1 configures CNY + USD rate.
	c, w := reportCurrencyCtx(t, 1, "PUT", `{"baseCurrency":"CNY","rates":[{"currency":"USD","rate":"7.20"}]}`)
	h.PutReportCurrency(c)
	if w.Code != 200 {
		t.Fatalf("tenant 1 put: %d %s", w.Code, w.Body.String())
	}

	// Tenant 2 configures USD base, no rates.
	c, w = reportCurrencyCtx(t, 2, "PUT", `{"baseCurrency":"USD","rates":[]}`)
	h.PutReportCurrency(c)
	if w.Code != 200 {
		t.Fatalf("tenant 2 put: %d %s", w.Code, w.Body.String())
	}

	// Each tenant reads back only its own configuration.
	c, w = reportCurrencyCtx(t, 1, "GET", "")
	h.GetReportCurrency(c)
	dto := decodeReportCurrency(t, w)
	if dto.BaseCurrency != "CNY" || len(dto.Rates) != 1 || dto.Rates[0].Currency != "USD" || dto.Rates[0].Rate != "7.2" {
		t.Fatalf("tenant 1 get: %+v", dto)
	}
	c, w = reportCurrencyCtx(t, 2, "GET", "")
	h.GetReportCurrency(c)
	dto = decodeReportCurrency(t, w)
	if dto.BaseCurrency != "USD" || len(dto.Rates) != 0 {
		t.Fatalf("tenant 2 get: %+v", dto)
	}

	// Unconfigured tenant 3 sees defaults, not tenant 1/2 configuration.
	c, w = reportCurrencyCtx(t, 3, "GET", "")
	h.GetReportCurrency(c)
	dto = decodeReportCurrency(t, w)
	if dto.BaseCurrency != fxrate.DefaultBaseCurrency || len(dto.Rates) != 0 {
		t.Fatalf("tenant 3 must see defaults: %+v", dto)
	}

	// Rows are persisted under each tenant id — nothing written to tenant 0.
	for _, tc := range []struct {
		tenant int64
		want   int64
	}{{0, 0}, {1, 3}, {2, 3}} {
		var n int64
		if err := svc.DB.Model(&Setting{}).
			Where("tenant_id = ? AND group_key = ?", tc.tenant, fxrate.SettingsGroup).Count(&n).Error; err != nil {
			t.Fatal(err)
		}
		if n != tc.want {
			t.Fatalf("tenant %d rows: got %d want %d", tc.tenant, n, tc.want)
		}
	}
}

// Missing tenant context must be rejected instead of silently writing to a
// shared bucket.
func TestReportCurrencyRequiresTenantContext(t *testing.T) {
	svc := newSettingsTestSvc(t)
	h := &Handler{Svc: svc}
	gin.SetMode(gin.TestMode)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("GET", "/api/v1/settings/report-currency", nil)
	h.GetReportCurrency(c)
	if w.Code != 403 {
		t.Fatalf("get without tenant: %d", w.Code)
	}

	w = httptest.NewRecorder()
	c, _ = gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("PUT", "/api/v1/settings/report-currency", strings.NewReader(`{"baseCurrency":"CNY"}`))
	c.Request.Header.Set("Content-Type", "application/json")
	h.PutReportCurrency(c)
	if w.Code != 403 {
		t.Fatalf("put without tenant: %d", w.Code)
	}
}
