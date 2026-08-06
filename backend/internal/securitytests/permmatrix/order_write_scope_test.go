package permmatrix

import (
	"fmt"
	"net/http"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"github.com/trademind-ai/trademind/backend/internal/modules/order"
	"github.com/trademind-ai/trademind/backend/internal/pkg/model"
)

// orderWriteProbe describes one mutating order route probed for store scope.
type orderWriteProbe struct {
	method string
	// path builds the request path from the seeded ungranted order + item.
	path func(orderID, itemID uuid.UUID) string
	body string
}

// TestOrderByIDWriteStoreScope is round117 regression evidence for the R116
// audit P2 item "lookup order by id then mutate": every mutating route under
// /api/v1/orders/:id (plus the order-item bind route) must resolve the target
// order tenant- and store-scoped. An operator without a grant on the order's
// shop and a cross-tenant admin both get 404 (no existence leak, no mutation).
// The registry below is checked for completeness against the mounted router,
// so newly added order-by-id write routes fail this test until covered.
func TestOrderByIDWriteStoreScope(t *testing.T) {
	h := sharedHarness(t)

	require.NoError(t, h.DB.Exec("DELETE FROM order_items WHERE order_id IN (SELECT id FROM orders WHERE order_no LIKE 'perm-matrix-r117-%')").Error)
	require.NoError(t, h.DB.Exec("DELETE FROM orders WHERE order_no LIKE 'perm-matrix-r117-%'").Error)

	shopID := h.ShopUngranted
	o := &order.Order{
		TenantID:          tenantA,
		Platform:          "manual",
		ShopID:            &shopID,
		OrderNo:           "perm-matrix-r117-" + uuid.NewString()[:8],
		CustomerName:      "perm-matrix",
		Status:            "pending",
		PaymentStatus:     "unpaid",
		FulfillmentStatus: "unfulfilled",
		Currency:          "CNY",
	}
	require.NoError(t, h.DB.Create(o).Error)
	item := &order.OrderItem{
		HardDeleteBase: model.HardDeleteBase{ID: uuid.New()},
		OrderID:        o.ID,
		ProductTitle:   "perm-matrix item",
		Quantity:       1,
	}
	require.NoError(t, h.DB.Create(item).Error)

	orderPath := func(suffix string) func(orderID, itemID uuid.UUID) string {
		return func(orderID, _ uuid.UUID) string {
			return "/api/v1/orders/" + orderID.String() + suffix
		}
	}
	probes := map[string]orderWriteProbe{
		"PUT /api/v1/orders/:id":    {http.MethodPut, orderPath(""), `{}`},
		"DELETE /api/v1/orders/:id": {http.MethodDelete, orderPath(""), `{}`},
		"POST /api/v1/orders/:id/items": {http.MethodPost, orderPath("/items"),
			`{"productTitle":"probe","quantity":1}`},
		"PUT /api/v1/orders/:id/items/:itemId": {http.MethodPut,
			func(orderID, itemID uuid.UUID) string {
				return fmt.Sprintf("/api/v1/orders/%s/items/%s", orderID, itemID)
			}, `{"productTitle":"probe","quantity":1}`},
		"DELETE /api/v1/orders/:id/items/:itemId": {http.MethodDelete,
			func(orderID, itemID uuid.UUID) string {
				return fmt.Sprintf("/api/v1/orders/%s/items/%s", orderID, itemID)
			}, `{}`},
		"POST /api/v1/orders/:id/deduct-inventory":  {http.MethodPost, orderPath("/deduct-inventory"), `{}`},
		"POST /api/v1/orders/:id/restore-inventory": {http.MethodPost, orderPath("/restore-inventory"), `{}`},
		"POST /api/v1/orders/:id/match-skus":        {http.MethodPost, orderPath("/match-skus"), `{}`},
		"POST /api/v1/orders/:id/shipments": {http.MethodPost, orderPath("/shipments"),
			`{"carrier":"probe","trackingNumber":"probe"}`},
		"PUT /api/v1/orders/:id/shipments/:shipmentId": {http.MethodPut,
			func(orderID, _ uuid.UUID) string {
				return fmt.Sprintf("/api/v1/orders/%s/shipments/%s", orderID, uuid.NewString())
			}, `{}`},
		"DELETE /api/v1/orders/:id/shipments/:shipmentId": {http.MethodDelete,
			func(orderID, _ uuid.UUID) string {
				return fmt.Sprintf("/api/v1/orders/%s/shipments/%s", orderID, uuid.NewString())
			}, `{}`},
		"POST /api/v1/orders/:id/shipments/:shipmentId/refresh-tracking": {http.MethodPost,
			func(orderID, _ uuid.UUID) string {
				return fmt.Sprintf("/api/v1/orders/%s/shipments/%s/refresh-tracking", orderID, uuid.NewString())
			}, `{}`},
		"POST /api/v1/orders/:id/tags": {http.MethodPost, orderPath("/tags"),
			fmt.Sprintf(`{"tagIds":[%q]}`, uuid.NewString())},
		"DELETE /api/v1/orders/:id/tags/:tagId": {http.MethodDelete,
			func(orderID, _ uuid.UUID) string {
				return fmt.Sprintf("/api/v1/orders/%s/tags/%s", orderID, uuid.NewString())
			}, `{}`},
		"POST /api/v1/orders/:id/sku-candidates/batch": {http.MethodPost,
			orderPath("/sku-candidates/batch"), `{"orderItemIds":[]}`},
		// GET probe: sku-candidate reads resolve the parent order scoped too.
		"GET /api/v1/order-items/:itemId/sku-candidates": {http.MethodGet,
			func(_, itemID uuid.UUID) string {
				return "/api/v1/order-items/" + itemID.String() + "/sku-candidates"
			}, `{}`},
		"POST /api/v1/order-items/:itemId/bind-sku": {http.MethodPost,
			func(_, itemID uuid.UUID) string {
				return "/api/v1/order-items/" + itemID.String() + "/bind-sku"
			}, fmt.Sprintf(`{"productSkuId":%q}`, uuid.NewString())},
	}

	// Completeness: every mounted mutating order-by-id route must be probed.
	for _, r := range h.registeredRoutes() {
		if r.Method == http.MethodGet {
			continue
		}
		if !strings.HasPrefix(r.Path, "/api/v1/orders/:id") &&
			!strings.HasPrefix(r.Path, "/api/v1/order-items/:itemId") {
			continue
		}
		require.Contains(t, probes, routeKey(r.Method, r.Path),
			"mutating order-by-id route %s %s has no store-scope probe; add it to TestOrderByIDWriteStoreScope",
			r.Method, r.Path)
	}

	for key, p := range probes {
		for _, pk := range []string{personaOperator, personaCrossTenant} {
			w := h.doBody(t, p.method, p.path(o.ID, item.ID), h.Personas[pk].Token, p.body)
			require.Equalf(t, http.StatusNotFound, w.Code,
				"%s [%s]: expected 404 for out-of-scope order, got %d: %s",
				key, pk, w.Code, w.Body.String())
		}
	}

	// The order and its item must be untouched after all denied probes.
	var cnt int64
	require.NoError(t, h.DB.Model(&order.Order{}).Where("id = ? AND deleted_at IS NULL", o.ID).Count(&cnt).Error)
	require.EqualValues(t, 1, cnt, "denied probes must not delete the order")
	require.NoError(t, h.DB.Model(&order.OrderItem{}).Where("id = ?", item.ID).Count(&cnt).Error)
	require.EqualValues(t, 1, cnt, "denied probes must not delete the order item")
}
