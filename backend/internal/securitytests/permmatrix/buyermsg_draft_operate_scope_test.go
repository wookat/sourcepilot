package permmatrix

import (
	"fmt"
	"net/http"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"github.com/trademind-ai/trademind/backend/internal/modules/admin"
	"github.com/trademind-ai/trademind/backend/internal/modules/customerchat"
	"github.com/trademind-ai/trademind/backend/internal/modules/order"
	"github.com/trademind-ai/trademind/backend/internal/pkg/adminperm"
)

// TestBuyerMsgDraftViewOnlyStoreScope is R159 audit evidence: buyer message
// draft write routes (edit / regenerate language / mark-sent / ignore / batch)
// must reject a store the caller can only view with 403 and leave the
// draft untouched, matching the R148/R125 cadence for store-scoped writes.
// A draft of an operable store stays writable.
func TestBuyerMsgDraftViewOnlyStoreScope(t *testing.T) {
	h := sharedHarness(t)
	r125Cleanup(t, h)
	t.Cleanup(func() { r125Cleanup(t, h) })

	viewShop, err := h.seedShop(tenantA, "perm-matrix-r159-view-only")
	require.NoError(t, err)
	grant := &admin.UserStorePermission{
		ID:              uuid.New(),
		UserID:          h.Personas[personaOperator].UserID,
		StoreID:         viewShop,
		Platform:        "manual",
		PermissionScope: admin.StorePermScopeView,
	}
	require.NoError(t, h.DB.Create(grant).Error)
	t.Cleanup(func() {
		h.DB.Exec("DELETE FROM user_store_permissions WHERE id = ?", grant.ID)
		adminperm.InvalidateUserPermissionCache(h.Personas[personaOperator].UserID)
	})
	adminperm.InvalidateUserPermissionCache(h.Personas[personaOperator].UserID)

	tpl := &customerchat.CustomerReplyTemplate{
		TenantID: tenantA, GroupKey: "logistics", Name: "perm-matrix-r125-r159-tpl",
		Content: "hello {订单号}", Enabled: true, DefaultLanguage: "zh-CN",
	}
	require.NoError(t, h.DB.Create(tpl).Error)
	require.NoError(t, h.DB.Create(&customerchat.CustomerReplyTemplateVariant{
		TenantID: tenantA, TemplateID: tpl.ID, Language: "en", Content: "hello {订单号} en",
	}).Error)

	mkDraft := func(shopID uuid.UUID, tag string) *customerchat.BuyerMessageDraft {
		o := r125SeedOrder(t, h, shopID, tag)
		sid := shopID
		d := &customerchat.BuyerMessageDraft{
			TenantID: tenantA, OrderID: o.ID, Node: "paid",
			RuleID: uuid.New(), TemplateID: tpl.ID, TemplateName: tpl.Name,
			Platform: "manual", ShopID: &sid, OrderNo: o.OrderNo,
			CustomerName: "perm-matrix-r159", Content: "original content",
			Language: "zh-CN", LangSource: customerchat.BuyerMsgLangSourceManual,
			Status: customerchat.BuyerMsgDraftPending,
		}
		require.NoError(t, h.DB.Create(d).Error)
		return d
	}
	viewDraft := mkDraft(viewShop, "r159-view")
	operableDraft := mkDraft(h.ShopGranted, "r159-operate")

	opTok := h.Personas[personaOperator].Token
	probes := []struct {
		method, path, body string
	}{
		{http.MethodPut, "/api/v1/customer/buyer-messages/drafts/%s", `{"content":"tampered by view-only"}`},
		{http.MethodPost, "/api/v1/customer/buyer-messages/drafts/%s/regenerate", `{"language":"en"}`},
		{http.MethodPost, "/api/v1/customer/buyer-messages/drafts/%s/mark-sent", "{}"},
		{http.MethodPost, "/api/v1/customer/buyer-messages/drafts/%s/ignore", "{}"},
	}
	for _, p := range probes {
		w := h.doBody(t, p.method, fmt.Sprintf(p.path, viewDraft.ID), opTok, p.body)
		require.Equalf(t, http.StatusForbidden, w.Code,
			"%s %s view-only store: %s", p.method, p.path, w.Body.String())
	}

	// Batch mark-sent must skip view-only stores instead of updating them.
	w := h.doBody(t, http.MethodPost, "/api/v1/customer/buyer-messages/drafts/batch-mark-sent",
		opTok, fmt.Sprintf(`{"ids":[%q]}`, viewDraft.ID))
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())

	var after customerchat.BuyerMessageDraft
	require.NoError(t, h.DB.Where("id = ?", viewDraft.ID).First(&after).Error)
	require.Equal(t, customerchat.BuyerMsgDraftPending, after.Status,
		"view-only store draft must not change status")
	require.Equal(t, "original content", after.Content,
		"view-only store draft content must stay untouched")
	require.Equal(t, "zh-CN", after.Language,
		"view-only store draft language must stay untouched")

	// Order write routes must apply the same view-only 403 gate.
	viewOrder := r125SeedOrder(t, h, viewShop, "r159-order-view")
	tag := &order.OrderTag{TenantID: tenantA, Name: "perm-matrix-r125-r159-tag"}
	require.NoError(t, h.DB.Create(tag).Error)
	t.Cleanup(func() { h.DB.Exec("DELETE FROM order_tags WHERE id = ?", tag.ID) })
	orderProbes := []struct {
		method, path, body string
	}{
		{http.MethodPut, "/api/v1/orders/%s", `{"customerName":"tampered"}`},
		{http.MethodPost, "/api/v1/orders/%s/items", `{"productTitle":"x","quantity":1}`},
		{http.MethodPost, "/api/v1/orders/%s/tags", `{"tagIds":[]}`},
		{http.MethodPost, "/api/v1/orders/%s/deduct-inventory", "{}"},
		{http.MethodPost, "/api/v1/orders/%s/match-skus", "{}"},
		{http.MethodDelete, "/api/v1/orders/%s", ""},
	}
	for _, p := range orderProbes {
		w := h.doBody(t, p.method, fmt.Sprintf(p.path, viewOrder.ID), opTok, p.body)
		require.Equalf(t, http.StatusForbidden, w.Code,
			"%s %s view-only store: %s", p.method, p.path, w.Body.String())
	}
	// Refresh-tracking resolves the order before the shipment, so a view-only
	// store must 403 here too.
	w = h.doBody(t, http.MethodPost,
		fmt.Sprintf("/api/v1/orders/%s/shipments/%s/refresh-tracking", viewOrder.ID, uuid.New()), opTok, "{}")
	require.Equal(t, http.StatusForbidden, w.Code, w.Body.String())

	// Print workbench mark is a write flag; view-only stores must be rejected.
	w = h.doBody(t, http.MethodPost, "/api/v1/orders/print/mark", opTok,
		fmt.Sprintf(`{"ids":[%q]}`, viewOrder.ID))
	require.Equal(t, http.StatusForbidden, w.Code, w.Body.String())

	// Batch tagging must reject when any order sits in a view-only store.
	w = h.doBody(t, http.MethodPost, "/api/v1/orders/batch-tags", opTok,
		fmt.Sprintf(`{"action":"add","orderIds":[%q],"tagIds":[%q]}`, viewOrder.ID, tag.ID))
	require.Equal(t, http.StatusForbidden, w.Code, w.Body.String())

	// Binding a SKU addresses the line item, so the parent order's store gate
	// must apply as well.
	viewItem := &order.OrderItem{
		OrderID:      viewOrder.ID,
		ProductTitle: "perm-matrix-r159-line", Quantity: 1, UnitPrice: 66, TotalPrice: 66,
	}
	require.NoError(t, h.DB.Create(viewItem).Error)
	w = h.doBody(t, http.MethodPost, fmt.Sprintf("/api/v1/order-items/%s/bind-sku", viewItem.ID), opTok,
		fmt.Sprintf(`{"productSkuId":%q}`, uuid.New()))
	require.Equal(t, http.StatusForbidden, w.Code, w.Body.String())

	var orderName string
	require.NoError(t, h.DB.Table("orders").Where("id = ?", viewOrder.ID).
		Pluck("customer_name", &orderName).Error)
	require.Equal(t, "perm-matrix-r125", orderName,
		"view-only store order must stay untouched")

	// Creating / moving an order into a view-only store is a write too.
	w = h.doBody(t, http.MethodPost, "/api/v1/orders", opTok,
		fmt.Sprintf(`{"orderNo":"perm-matrix-r125-r159-create","platform":"manual","shopId":%q,"customerName":"x"}`, viewShop))
	require.Equal(t, http.StatusForbidden, w.Code, w.Body.String())

	operableOrder := r125SeedOrder(t, h, h.ShopGranted, "r159-order-op")
	w = h.doBody(t, http.MethodPut, fmt.Sprintf("/api/v1/orders/%s", operableOrder.ID), opTok,
		fmt.Sprintf(`{"shopId":%q}`, viewShop))
	require.Equal(t, http.StatusForbidden, w.Code, w.Body.String())

	// Operable store keeps working (no over-blocking).
	w = h.doBody(t, http.MethodPut, fmt.Sprintf("/api/v1/orders/%s", operableOrder.ID), opTok,
		`{"customerName":"updated by operator"}`)
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())

	w = h.doBody(t, http.MethodPost, fmt.Sprintf("/api/v1/customer/buyer-messages/drafts/%s/regenerate", operableDraft.ID),
		opTok, `{"language":"en"}`)
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	w = h.doBody(t, http.MethodPost, fmt.Sprintf("/api/v1/customer/buyer-messages/drafts/%s/mark-sent", operableDraft.ID),
		opTok, "{}")
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
}
