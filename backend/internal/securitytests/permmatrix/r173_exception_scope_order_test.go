package permmatrix

import (
	"net/http"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/trademind-ai/trademind/backend/internal/modules/ordersync"
)

// TestExceptionMarkScopeBeforeBody pins the site-wide validation order on the
// exception workbench mark routes: the store-operate scope gate must answer
// before request-body validation, so a view-only principal gets 403/40303 and
// an invisible source stays 404 even when the body is missing exceptionType.
func TestExceptionMarkScopeBeforeBody(t *testing.T) {
	h := sharedHarness(t)
	cleanup := func() {
		require.NoError(t, h.DB.Exec("DELETE FROM order_sync_tasks WHERE cursor = 'perm-matrix-r173'").Error)
	}
	cleanup()
	t.Cleanup(cleanup)

	ost := &ordersync.OrderSyncTask{
		TenantID: tenantA, ShopID: h.ShopViewOnly, Platform: "manual",
		TaskType: "manual", Status: ordersync.StatusFailed, Mode: "manual",
		Cursor: "perm-matrix-r173",
	}
	require.NoError(t, h.DB.Create(ost).Error)
	granted := &ordersync.OrderSyncTask{
		TenantID: tenantA, ShopID: h.ShopGranted, Platform: "manual",
		TaskType: "manual", Status: ordersync.StatusFailed, Mode: "manual",
		Cursor: "perm-matrix-r173",
	}
	require.NoError(t, h.DB.Create(granted).Error)

	viewTok := h.Personas[personaViewOnly].Token
	operatorTok := h.Personas[personaOperator].Token
	base := "/api/v1/orders/exceptions/order_sync_task/" + ost.ID.String()

	for _, action := range []string{"handle", "ignore"} {
		path := base + "/" + action
		// view-only store + empty body: scope wins over body validation
		w := h.doBody(t, http.MethodPost, path, viewTok, `{}`)
		require.Equalf(t, http.StatusForbidden, w.Code,
			"POST %s [viewOnlyOperator, empty body]: scope gate must answer before body validation, got %d: %s",
			path, w.Code, w.Body.String())
		requireCode40303(t, w, "POST "+path+" (empty body)")

		// invisible store + empty body: existence stays hidden (404, not 400)
		w = h.doBody(t, http.MethodPost, path, operatorTok, `{}`)
		require.Equalf(t, http.StatusNotFound, w.Code,
			"POST %s [operator without grant, empty body]: invisible source must stay 404, got %d: %s",
			path, w.Code, w.Body.String())

		// operable store + empty body: body validation still enforced (400)
		w = h.doBody(t, http.MethodPost,
			"/api/v1/orders/exceptions/order_sync_task/"+granted.ID.String()+"/"+action, operatorTok, `{}`)
		require.Equalf(t, http.StatusBadRequest, w.Code,
			"POST %s [operator, operable store, empty body]: expected 400 exceptionType 不能为空, got %d: %s",
			action, w.Code, w.Body.String())
	}
}
