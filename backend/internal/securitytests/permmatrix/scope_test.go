package permmatrix

import (
	"encoding/json"
	"net/http"
	"testing"

	"github.com/stretchr/testify/require"
)

type envelope struct {
	Code int             `json:"code"`
	Data json.RawMessage `json:"data"`
}

func listShopIDs(t *testing.T, raw []byte) []string {
	t.Helper()
	var env envelope
	require.NoError(t, json.Unmarshal(raw, &env))
	var data struct {
		List []struct {
			ID string `json:"id"`
		} `json:"list"`
	}
	require.NoError(t, json.Unmarshal(env.Data, &data))
	ids := make([]string, 0, len(data.List))
	for _, s := range data.List {
		ids = append(ids, s.ID)
	}
	return ids
}

// TestCrossTenantShopIsolation asserts a tenant B admin can never see or read
// tenant A shops (list is scoped, direct read is 404).
func TestCrossTenantShopIsolation(t *testing.T) {
	h := sharedHarness(t)
	cross := h.Personas[personaCrossTenant]

	w := h.do(t, http.MethodGet, "/api/v1/shops?pageSize=200", cross.Token)
	require.Equal(t, http.StatusOK, w.Code)
	for _, id := range listShopIDs(t, w.Body.Bytes()) {
		require.NotEqual(t, h.ShopGranted.String(), id, "tenant B admin must not see tenant A shop in list")
		require.NotEqual(t, h.ShopUngranted.String(), id, "tenant B admin must not see tenant A shop in list")
	}

	w = h.do(t, http.MethodGet, "/api/v1/shops/"+h.ShopGranted.String(), cross.Token)
	require.Equal(t, http.StatusNotFound, w.Code, "tenant B admin reading tenant A shop must get 404")
}

// TestOperatorStoreScope asserts an operator with a single store grant only
// sees that store, and cannot read or mutate ungranted stores.
func TestOperatorStoreScope(t *testing.T) {
	h := sharedHarness(t)
	op := h.Personas[personaOperator]

	w := h.do(t, http.MethodGet, "/api/v1/shops?pageSize=200", op.Token)
	require.Equal(t, http.StatusOK, w.Code)
	ids := listShopIDs(t, w.Body.Bytes())
	require.Contains(t, ids, h.ShopGranted.String(), "operator must see granted shop")
	require.NotContains(t, ids, h.ShopUngranted.String(), "operator must not see ungranted shop")

	w = h.do(t, http.MethodGet, "/api/v1/shops/"+h.ShopUngranted.String(), op.Token)
	require.Contains(t, []int{http.StatusForbidden, http.StatusNotFound}, w.Code,
		"operator reading ungranted shop must be denied, got %d", w.Code)
}

// TestReadonlyStoreScope asserts a readonly account without store grants gets
// an empty shop list and cannot mutate existing resources (route-level 403).
func TestReadonlyStoreScope(t *testing.T) {
	h := sharedHarness(t)
	ro := h.Personas[personaReadonly]

	w := h.do(t, http.MethodGet, "/api/v1/shops?pageSize=200", ro.Token)
	require.Equal(t, http.StatusOK, w.Code)
	require.Empty(t, listShopIDs(t, w.Body.Bytes()), "readonly without grants must get empty shop list")

	w = h.do(t, http.MethodPut, "/api/v1/shops/"+h.ShopGranted.String(), ro.Token)
	require.Equal(t, http.StatusForbidden, w.Code, "readonly write on existing shop must be 403")
}

// TestReadonlyWriteGuardRegression is regression evidence for the round52 P0
// fixes: settings/test-image and settings/test-ocr previously had no
// settings.manage permission check, and write endpoints without route-level
// guards returned 404/400 instead of 403 for readonly accounts.
func TestReadonlyWriteGuardRegression(t *testing.T) {
	h := sharedHarness(t)
	for _, path := range []string{"/api/v1/settings/test-image", "/api/v1/settings/test-ocr"} {
		for _, pk := range []string{personaReadonly, personaOperator} {
			w := h.do(t, http.MethodPost, path, h.Personas[pk].Token)
			require.Equalf(t, http.StatusForbidden, w.Code, "%s [%s]: settings.manage guard must reject", path, pk)
		}
		w := h.do(t, http.MethodPost, path, h.Personas[personaAdmin].Token)
		require.NotEqualf(t, http.StatusForbidden, w.Code, "%s [admin]: must pass guard", path)
	}

	// Central guard: readonly write on an existing resource is 403, not 404.
	w := h.do(t, http.MethodDelete, "/api/v1/shops/"+h.ShopUngranted.String(), h.Personas[personaReadonly].Token)
	require.Equal(t, http.StatusForbidden, w.Code)
}
