package permmatrix

import (
	"encoding/json"
	"net/http"
	"testing"

	"github.com/google/uuid"
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

// TestOperationTaskStoreScope is regression evidence for the round61 P2 fix:
// operation tasks are shop-scoped business data. Operators only see tasks
// bound to granted shops; unscoped (tenant-level) tasks are admin-only;
// out-of-scope or cross-tenant direct reads are 404 (no existence leak).
func TestOperationTaskStoreScope(t *testing.T) {
	h := sharedHarness(t)
	adminP := h.Personas[personaAdmin]
	op := h.Personas[personaOperator]
	ro := h.Personas[personaReadonly]
	cross := h.Personas[personaCrossTenant]

	createTask := func(shopID string, tag string) string {
		body := map[string]any{
			"sourceType": "manual",
			"taskType":   "product_content",
			"platform":   "douyin",
			"title":      "perm-matrix optask " + tag,
			"payload":    map[string]any{"title": "safe"},
			"priority":   "normal",
		}
		if shopID != "" {
			body["shopId"] = shopID
		}
		raw, err := json.Marshal(body)
		require.NoError(t, err)
		w := h.doBody(t, http.MethodPost, "/api/v1/operation-tasks", adminP.Token, string(raw))
		require.Equal(t, http.StatusCreated, w.Code, w.Body.String())
		var env envelope
		require.NoError(t, json.Unmarshal(w.Body.Bytes(), &env))
		var task struct {
			ID string `json:"id"`
		}
		require.NoError(t, json.Unmarshal(env.Data, &task))
		return task.ID
	}
	grantedTask := createTask(h.ShopGranted.String(), "granted-"+uuid.NewString()[:8])
	ungrantedTask := createTask(h.ShopUngranted.String(), "ungranted-"+uuid.NewString()[:8])
	tenantTask := createTask("", "tenant-"+uuid.NewString()[:8])

	listTaskIDs := func(token string) []string {
		w := h.do(t, http.MethodGet, "/api/v1/operation-tasks?limit=100", token)
		require.Equal(t, http.StatusOK, w.Code)
		var env envelope
		require.NoError(t, json.Unmarshal(w.Body.Bytes(), &env))
		var data struct {
			Items []struct {
				ID string `json:"id"`
			} `json:"items"`
		}
		require.NoError(t, json.Unmarshal(env.Data, &data))
		ids := make([]string, 0, len(data.Items))
		for _, item := range data.Items {
			ids = append(ids, item.ID)
		}
		return ids
	}

	adminIDs := listTaskIDs(adminP.Token)
	require.Contains(t, adminIDs, grantedTask)
	require.Contains(t, adminIDs, ungrantedTask)
	require.Contains(t, adminIDs, tenantTask)

	opIDs := listTaskIDs(op.Token)
	require.Contains(t, opIDs, grantedTask, "operator must see task bound to granted shop")
	require.NotContains(t, opIDs, ungrantedTask, "operator must not see task bound to ungranted shop")
	require.NotContains(t, opIDs, tenantTask, "operator must not see tenant-level task")

	require.Empty(t, listTaskIDs(ro.Token), "readonly without grants must get empty task list")

	w := h.do(t, http.MethodGet, "/api/v1/operation-tasks/"+ungrantedTask, op.Token)
	require.Equal(t, http.StatusNotFound, w.Code, "operator reading out-of-scope task must get 404")
	w = h.do(t, http.MethodGet, "/api/v1/operation-tasks/"+grantedTask, op.Token)
	require.Equal(t, http.StatusOK, w.Code)
	w = h.do(t, http.MethodGet, "/api/v1/operation-tasks/"+grantedTask, ro.Token)
	require.Equal(t, http.StatusNotFound, w.Code, "readonly without grants reading task must get 404")
	w = h.do(t, http.MethodGet, "/api/v1/operation-tasks/"+grantedTask, cross.Token)
	require.Equal(t, http.StatusNotFound, w.Code, "cross-tenant read must get 404")
	w = h.do(t, http.MethodGet, "/api/v1/operation-tasks/"+ungrantedTask+"/events?limit=10", op.Token)
	require.Equal(t, http.StatusNotFound, w.Code, "out-of-scope child reads must get 404")
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
