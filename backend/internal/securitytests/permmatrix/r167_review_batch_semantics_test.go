package permmatrix

import (
	"net/http"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"github.com/trademind-ai/trademind/backend/internal/modules/admin"
	"github.com/trademind-ai/trademind/backend/internal/modules/order"
)

// TestR167ReviewBatchWholeBatchDenied pins the R167 decision for the
// order-review batch endpoints: when a batch mixes orders from an operable
// store with orders from a view-only store, the whole batch is rejected
// up-front with 403/40303 and nothing is applied — not even the rows the
// caller could otherwise operate on. Row-level semantics (200 envelope with
// per-row failures) would let the operable part of a mixed batch take effect.
func TestR167ReviewBatchWholeBatchDenied(t *testing.T) {
	h := sharedHarness(t)
	r125Cleanup(t, h)
	t.Cleanup(func() { r125Cleanup(t, h) })

	// Give the view-only persona an operate grant on a second store so the
	// batch genuinely mixes operable and view-only rows.
	operateGrant := &admin.UserStorePermission{
		ID:              uuid.New(),
		UserID:          h.Personas[personaViewOnly].UserID,
		StoreID:         h.ShopGranted,
		Platform:        "manual",
		PermissionScope: admin.StorePermScopeOperate,
	}
	require.NoError(t, h.DB.Create(operateGrant).Error)
	t.Cleanup(func() {
		_ = h.DB.Unscoped().Delete(&admin.UserStorePermission{}, "id = ?", operateGrant.ID).Error
	})

	seedPending := func(shopID uuid.UUID, tag string) *order.Order {
		o := r125SeedOrder(t, h, shopID, tag)
		require.NoError(t, h.DB.Model(&order.Order{}).Where("id = ?", o.ID).
			Update("review_status", order.ReviewStatusPending).Error)
		return o
	}
	reviewStatus := func(id uuid.UUID) string {
		var st string
		require.NoError(t, h.DB.Table("orders").Where("id = ?", id).Pluck("review_status", &st).Error)
		return st
	}

	operable := seedPending(h.ShopGranted, "r167-operable")
	viewOnly := seedPending(h.ShopViewOnly, "r167-view")
	body := `{"orderIds":["` + operable.ID.String() + `","` + viewOnly.ID.String() + `"],"remark":"r167"}`

	for _, path := range []string{"/api/v1/order-review/approve", "/api/v1/order-review/reject"} {
		w := h.doBody(t, http.MethodPost, path, h.Personas[personaViewOnly].Token, body)
		require.Equalf(t, http.StatusForbidden, w.Code,
			"%s mixed batch: whole batch must be denied, got %d: %s", path, w.Code, w.Body.String())
		requireCode40303(t, w, path)
		require.Equalf(t, order.ReviewStatusPending, reviewStatus(operable.ID),
			"%s must not apply the operable row of a denied batch", path)
		require.Equalf(t, order.ReviewStatusPending, reviewStatus(viewOnly.ID),
			"%s must not apply the view-only row of a denied batch", path)
	}

	// Not over-blocked: the same caller approves a batch of only operable rows.
	w := h.doBody(t, http.MethodPost, "/api/v1/order-review/approve",
		h.Personas[personaViewOnly].Token,
		`{"orderIds":["`+operable.ID.String()+`"],"remark":"r167"}`)
	require.Equal(t, http.StatusOK, w.Code, "operable-only batch must pass: %s", w.Body.String())
	require.Equal(t, order.ReviewStatusApproved, reviewStatus(operable.ID))
}
