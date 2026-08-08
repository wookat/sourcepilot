package permmatrix

import (
	"encoding/json"
	"net/http"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"github.com/trademind-ai/trademind/backend/internal/modules/ordersync"
)

// TestR189EmptyStoreScopeTaskCenterFailures is the regression for the R189 P1:
// the task center store filter only applied when the allowed store set was
// non-empty, so a non-admin holding no store grant (readonly persona, or any
// principal whose grants could not be resolved) read every store's failures of
// the tenant. A non-nil but empty scope must hide store-bound rows.
func TestR189EmptyStoreScopeTaskCenterFailures(t *testing.T) {
	h := sharedHarness(t)

	seedFailure := func(shopID uuid.UUID) uuid.UUID {
		task := &ordersync.OrderSyncTask{
			TenantID:     tenantA,
			ShopID:       shopID,
			Platform:     "manual",
			TaskType:     "orders",
			Status:       ordersync.StatusFailed,
			Mode:         "manual",
			ErrorMessage: "r189 empty scope probe",
		}
		require.NoError(t, h.DB.Create(task).Error)
		t.Cleanup(func() { h.DB.Unscoped().Delete(task) })
		return task.ID
	}
	granted := seedFailure(h.ShopGranted)
	ungranted := seedFailure(h.ShopUngranted)

	shopIDsOf := func(raw []byte) []string {
		var env envelope
		require.NoError(t, json.Unmarshal(raw, &env))
		var data struct {
			List []struct {
				ID     string `json:"id"`
				ShopID string `json:"shopId"`
			} `json:"list"`
		}
		require.NoError(t, json.Unmarshal(env.Data, &data))
		out := make([]string, 0, len(data.List))
		for _, row := range data.List {
			out = append(out, row.ShopID)
		}
		return out
	}

	const path = "/api/v1/task-center/failures?pageSize=200"

	// Readonly without any store grant: empty allowed set, nothing store-bound.
	w := h.do(t, http.MethodGet, path, h.Personas[personaReadonly].Token)
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	shops := shopIDsOf(w.Body.Bytes())
	require.NotContains(t, shops, h.ShopGranted.String(), "no-grant persona must not read granted-shop failures")
	require.NotContains(t, shops, h.ShopUngranted.String(), "no-grant persona must not read any shop failures")

	// Operator with one grant keeps seeing only that store.
	w = h.do(t, http.MethodGet, path, h.Personas[personaOperator].Token)
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	shops = shopIDsOf(w.Body.Bytes())
	require.Contains(t, shops, h.ShopGranted.String(), "operator must keep reading granted-shop failures")
	require.NotContains(t, shops, h.ShopUngranted.String(), "operator must not read ungranted-shop failures")

	// Admin of the tenant still sees both.
	w = h.do(t, http.MethodGet, path, h.Personas[personaAdmin].Token)
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	shops = shopIDsOf(w.Body.Bytes())
	require.Contains(t, shops, h.ShopGranted.String())
	require.Contains(t, shops, h.ShopUngranted.String())

	// Cross-tenant admin sees neither.
	w = h.do(t, http.MethodGet, path, h.Personas[personaCrossTenant].Token)
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	shops = shopIDsOf(w.Body.Bytes())
	require.NotContains(t, shops, h.ShopGranted.String())
	require.NotContains(t, shops, h.ShopUngranted.String())

	_ = granted
	_ = ungranted
}
