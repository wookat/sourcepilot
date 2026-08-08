package permmatrix

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"github.com/trademind-ai/trademind/backend/internal/modules/customerchat"
	"github.com/trademind-ai/trademind/backend/internal/modules/order"
	"github.com/trademind-ai/trademind/backend/internal/pkg/model"
	"github.com/trademind-ai/trademind/backend/internal/pkg/response"
)

// viewOnlyProbe describes one mutating store-scoped route probed with the
// viewOnlyOperator persona.
type viewOnlyProbe struct {
	method string
	path   func(orderID, itemID uuid.UUID) string
	body   string
	// childLookup marks routes addressing an unseeded child resource
	// (shipment/tag by random id): resolution order may answer 404 before the
	// store gate, so both 403 and 404 are acceptable — a success is not.
	childLookup bool
	// readCompute marks POST routes that perform no mutation (candidate
	// suggestions): the view grant makes them answer 200.
	readCompute bool
}

func requireCode40303(t *testing.T, w *httptest.ResponseRecorder, label string) {
	t.Helper()
	var body struct {
		Code    int    `json:"code"`
		Message string `json:"message"`
	}
	require.NoErrorf(t, json.Unmarshal(w.Body.Bytes(), &body), "%s: %s", label, w.Body.String())
	require.Equalf(t, response.CodeStorePermissionDenied, body.Code,
		"%s: view-only 403 must use business code 40303, got %d: %s", label, body.Code, w.Body.String())
	require.Equalf(t, "店铺无操作权限", body.Message,
		"%s: 40303 message must use the unified copy, got: %s", label, w.Body.String())
}

// TestViewOnlyPersonaStoreWriteScope is the R160 view-only persona tier
// (R159 audit P2): an operator whose only store grant is scope "view" must be
// rejected with 403 (business code 40303) on every mutating order-by-id /
// order-item / buyer-message-draft route addressing that store, with zero
// mutation, while reads keep working. The probe registry is checked for
// completeness against the mounted router, so newly added mutating routes in
// these families fail this test until covered.
func TestViewOnlyPersonaStoreWriteScope(t *testing.T) {
	h := sharedHarness(t)
	cleanup := func() {
		require.NoError(t, h.DB.Exec("DELETE FROM order_items WHERE order_id IN (SELECT id FROM orders WHERE order_no LIKE 'perm-matrix-r125-r160-%')").Error)
		r125Cleanup(t, h)
	}
	cleanup()
	t.Cleanup(cleanup)

	tok := h.Personas[personaViewOnly].Token
	o := r125SeedOrder(t, h, h.ShopViewOnly, "r160-view")
	item := &order.OrderItem{
		HardDeleteBase: model.HardDeleteBase{ID: uuid.New()},
		OrderID:        o.ID,
		ProductTitle:   "perm-matrix-r160 item",
		Quantity:       1,
	}
	require.NoError(t, h.DB.Create(item).Error)

	orderPath := func(suffix string) func(orderID, itemID uuid.UUID) string {
		return func(orderID, _ uuid.UUID) string {
			return "/api/v1/orders/" + orderID.String() + suffix
		}
	}
	probes := map[string]viewOnlyProbe{
		"PUT /api/v1/orders/:id":    {method: http.MethodPut, path: orderPath(""), body: `{"customerName":"tampered"}`},
		"DELETE /api/v1/orders/:id": {method: http.MethodDelete, path: orderPath(""), body: `{}`},
		"POST /api/v1/orders/:id/items": {method: http.MethodPost, path: orderPath("/items"),
			body: `{"productTitle":"probe","quantity":1}`},
		"PUT /api/v1/orders/:id/items/:itemId": {method: http.MethodPut,
			path: func(orderID, itemID uuid.UUID) string {
				return fmt.Sprintf("/api/v1/orders/%s/items/%s", orderID, itemID)
			}, body: `{"productTitle":"probe","quantity":1}`},
		"DELETE /api/v1/orders/:id/items/:itemId": {method: http.MethodDelete,
			path: func(orderID, itemID uuid.UUID) string {
				return fmt.Sprintf("/api/v1/orders/%s/items/%s", orderID, itemID)
			}, body: `{}`},
		"POST /api/v1/orders/:id/deduct-inventory":  {method: http.MethodPost, path: orderPath("/deduct-inventory"), body: `{}`},
		"POST /api/v1/orders/:id/restore-inventory": {method: http.MethodPost, path: orderPath("/restore-inventory"), body: `{}`},
		"POST /api/v1/orders/:id/match-skus":        {method: http.MethodPost, path: orderPath("/match-skus"), body: `{}`},
		"POST /api/v1/orders/:id/shipments": {method: http.MethodPost, path: orderPath("/shipments"),
			body: `{"carrier":"probe","trackingNumber":"probe"}`},
		"PUT /api/v1/orders/:id/shipments/:shipmentId": {method: http.MethodPut,
			path: func(orderID, _ uuid.UUID) string {
				return fmt.Sprintf("/api/v1/orders/%s/shipments/%s", orderID, uuid.NewString())
			}, body: `{}`, childLookup: true},
		"DELETE /api/v1/orders/:id/shipments/:shipmentId": {method: http.MethodDelete,
			path: func(orderID, _ uuid.UUID) string {
				return fmt.Sprintf("/api/v1/orders/%s/shipments/%s", orderID, uuid.NewString())
			}, body: `{}`, childLookup: true},
		"POST /api/v1/orders/:id/shipments/:shipmentId/refresh-tracking": {method: http.MethodPost,
			path: func(orderID, _ uuid.UUID) string {
				return fmt.Sprintf("/api/v1/orders/%s/shipments/%s/refresh-tracking", orderID, uuid.NewString())
			}, body: `{}`},
		"POST /api/v1/orders/:id/tags": {method: http.MethodPost, path: orderPath("/tags"),
			body: fmt.Sprintf(`{"tagIds":[%q]}`, uuid.NewString())},
		"DELETE /api/v1/orders/:id/tags/:tagId": {method: http.MethodDelete,
			path: func(orderID, _ uuid.UUID) string {
				return fmt.Sprintf("/api/v1/orders/%s/tags/%s", orderID, uuid.NewString())
			}, body: `{}`, childLookup: true},
		"POST /api/v1/orders/:id/sku-candidates/batch": {method: http.MethodPost,
			path: orderPath("/sku-candidates/batch"), body: `{"orderItemIds":[]}`, readCompute: true},
		"POST /api/v1/order-items/:itemId/bind-sku": {method: http.MethodPost,
			path: func(_, itemID uuid.UUID) string {
				return "/api/v1/order-items/" + itemID.String() + "/bind-sku"
			}, body: fmt.Sprintf(`{"productSkuId":%q}`, uuid.NewString())},
	}

	// Completeness: every mounted mutating order-by-id / order-item route must
	// be probed with the view-only persona.
	for _, r := range h.registeredRoutes() {
		if r.Method == http.MethodGet {
			continue
		}
		if !strings.HasPrefix(r.Path, "/api/v1/orders/:id") &&
			!strings.HasPrefix(r.Path, "/api/v1/order-items/:itemId") {
			continue
		}
		require.Contains(t, probes, routeKey(r.Method, r.Path),
			"mutating order-by-id route %s %s has no view-only probe; add it to TestViewOnlyPersonaStoreWriteScope",
			r.Method, r.Path)
	}

	for key, p := range probes {
		w := h.doBody(t, p.method, p.path(o.ID, item.ID), tok, p.body)
		if p.readCompute {
			require.Equalf(t, http.StatusOK, w.Code,
				"%s [viewOnlyOperator]: read-compute route must stay readable, got %d: %s", key, w.Code, w.Body.String())
			continue
		}
		if p.childLookup && w.Code == http.StatusNotFound {
			continue
		}
		require.Equalf(t, http.StatusForbidden, w.Code,
			"%s [viewOnlyOperator]: expected 403, got %d: %s", key, w.Code, w.Body.String())
		requireCode40303(t, w, key)
	}

	// Buyer message draft writes must apply the same gate.
	tpl := &customerchat.CustomerReplyTemplate{
		TenantID: tenantA, GroupKey: "logistics", Name: "perm-matrix-r125-r160-tpl",
		Content: "hello {订单号}", Enabled: true, DefaultLanguage: "zh-CN",
	}
	require.NoError(t, h.DB.Create(tpl).Error)
	sid := h.ShopViewOnly
	draft := &customerchat.BuyerMessageDraft{
		TenantID: tenantA, OrderID: o.ID, Node: "paid",
		RuleID: uuid.New(), TemplateID: tpl.ID, TemplateName: tpl.Name,
		Platform: "manual", ShopID: &sid, OrderNo: o.OrderNo,
		CustomerName: "perm-matrix-r160", Content: "original content",
		Language: "zh-CN", LangSource: customerchat.BuyerMsgLangSourceManual,
		Status: customerchat.BuyerMsgDraftPending,
	}
	require.NoError(t, h.DB.Create(draft).Error)
	draftProbes := []struct {
		method, path, body string
	}{
		{http.MethodPut, "/api/v1/customer/buyer-messages/drafts/%s", `{"content":"tampered"}`},
		{http.MethodPost, "/api/v1/customer/buyer-messages/drafts/%s/regenerate", `{"language":"en"}`},
		{http.MethodPost, "/api/v1/customer/buyer-messages/drafts/%s/mark-sent", "{}"},
		{http.MethodPost, "/api/v1/customer/buyer-messages/drafts/%s/ignore", "{}"},
	}
	for _, p := range draftProbes {
		w := h.doBody(t, p.method, fmt.Sprintf(p.path, draft.ID), tok, p.body)
		require.Equalf(t, http.StatusForbidden, w.Code,
			"%s %s [viewOnlyOperator]: %s", p.method, p.path, w.Body.String())
		requireCode40303(t, w, p.method+" "+p.path)
	}

	// Zero mutation: order, item and draft stay untouched.
	var name string
	require.NoError(t, h.DB.Table("orders").Where("id = ?", o.ID).Pluck("customer_name", &name).Error)
	require.Equal(t, "perm-matrix-r125", name, "view-only order must stay untouched")
	var cnt int64
	require.NoError(t, h.DB.Model(&order.Order{}).Where("id = ? AND deleted_at IS NULL", o.ID).Count(&cnt).Error)
	require.EqualValues(t, 1, cnt, "denied probes must not delete the order")
	require.NoError(t, h.DB.Model(&order.OrderItem{}).Where("id = ?", item.ID).Count(&cnt).Error)
	require.EqualValues(t, 1, cnt, "denied probes must not delete the order item")
	var after customerchat.BuyerMessageDraft
	require.NoError(t, h.DB.Where("id = ?", draft.ID).First(&after).Error)
	require.Equal(t, customerchat.BuyerMsgDraftPending, after.Status)
	require.Equal(t, "original content", after.Content)

	// Reads keep working: the view grant makes the store's data visible.
	w := h.doBody(t, http.MethodGet, "/api/v1/orders/"+o.ID.String(), tok, "")
	require.Equal(t, http.StatusOK, w.Code, "view-only persona must still read the order: %s", w.Body.String())
}
