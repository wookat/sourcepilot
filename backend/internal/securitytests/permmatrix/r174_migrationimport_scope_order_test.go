package permmatrix

import (
	"fmt"
	"net/http"
	"testing"

	"github.com/stretchr/testify/require"
)

// TestMigrationImportScopeBeforeBody pins the site-wide validation order on
// the migration import wizard routes (R173-line2 P2-2): for shop-scoped kinds
// the store-operate gate must answer before request-body shape validation, so
// a view-only principal gets 403/40303 and an invisible shop stays 404 even
// when columns/rows/mapping (and fileHash for commit) are missing.
func TestMigrationImportScopeBeforeBody(t *testing.T) {
	h := sharedHarness(t)

	viewTok := h.Personas[personaViewOnly].Token
	operatorTok := h.Personas[personaOperator].Token

	emptyShape := func(shopID string) string {
		return fmt.Sprintf(`{"kind":"product","shopId":"%s"}`, shopID)
	}

	for _, route := range []string{"/api/v1/imports/validate", "/api/v1/imports/commit"} {
		// view-only shop + empty body: scope wins over body-shape validation
		w := h.doBody(t, http.MethodPost, route, viewTok, emptyShape(h.ShopViewOnly.String()))
		require.Equalf(t, http.StatusForbidden, w.Code,
			"POST %s [viewOnlyOperator, empty shape]: scope gate must answer before body validation, got %d: %s",
			route, w.Code, w.Body.String())
		requireCode40303(t, w, "POST "+route+" (empty shape)")

		// invisible shop + empty body: existence stays hidden (404, not 400)
		w = h.doBody(t, http.MethodPost, route, operatorTok, emptyShape(h.ShopUngranted.String()))
		require.Equalf(t, http.StatusNotFound, w.Code,
			"POST %s [operator without grant, empty shape]: invisible shop must stay 404, got %d: %s",
			route, w.Code, w.Body.String())

		// operable shop + empty body: body-shape validation still enforced (400)
		w = h.doBody(t, http.MethodPost, route, operatorTok, emptyShape(h.ShopGranted.String()))
		require.Equalf(t, http.StatusBadRequest, w.Code,
			"POST %s [operator, operable shop, empty shape]: expected 400 shape validation, got %d: %s",
			route, w.Code, w.Body.String())
	}
}
