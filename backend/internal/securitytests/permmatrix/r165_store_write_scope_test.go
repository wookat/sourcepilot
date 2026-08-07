package permmatrix

import (
	"net/http"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"github.com/trademind-ai/trademind/backend/internal/modules/order"
	"github.com/trademind-ai/trademind/backend/internal/modules/shop"
)

// TestR165ReviewDecisionStoreOperateScope is R165 line-2 audit evidence: the
// order-review workbench decides on client-supplied order ids, so releasing or
// rejecting an order must require an operate/manage grant on its store. A
// view-only grant used to pass the visibility-only gate and actually flip
// review_status to "approved".
func TestR165ReviewDecisionStoreOperateScope(t *testing.T) {
	h := sharedHarness(t)
	r125Cleanup(t, h)
	t.Cleanup(func() { r125Cleanup(t, h) })

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

	viewOrder := seedPending(h.ShopViewOnly, "r165-view")
	body := func(id uuid.UUID) string { return `{"orderIds":["` + id.String() + `"],"remark":"r165"}` }

	for _, path := range []string{"/api/v1/order-review/approve", "/api/v1/order-review/reject"} {
		w := h.doBody(t, http.MethodPost, path, h.Personas[personaViewOnly].Token, body(viewOrder.ID))
		require.Equalf(t, http.StatusForbidden, w.Code,
			"%s [viewOnlyOperator]: expected 403, got %d: %s", path, w.Code, w.Body.String())
		requireCode40303(t, w, path)
		require.Equalf(t, order.ReviewStatusPending, reviewStatus(viewOrder.ID),
			"%s must not mutate review_status of a view-only store order", path)
	}

	// Orders of a store the caller cannot even see stay a per-row "not found"
	// result (200 envelope, zero applied) so existence is not leaked.
	ungranted := seedPending(h.ShopUngranted, "r165-ungranted")
	w := h.doBody(t, http.MethodPost, "/api/v1/order-review/approve",
		h.Personas[personaOperator].Token, body(ungranted.ID))
	require.NotEqual(t, order.ReviewStatusApproved, reviewStatus(ungranted.ID),
		"operator without a grant must not approve: %s", w.Body.String())

	// Not over-blocked: an operate grant still approves.
	granted := seedPending(h.ShopGranted, "r165-granted")
	w = h.doBody(t, http.MethodPost, "/api/v1/order-review/approve",
		h.Personas[personaOperator].Token, body(granted.ID))
	require.Equal(t, http.StatusOK, w.Code, "granted operator must approve: %s", w.Body.String())
	require.Equal(t, order.ReviewStatusApproved, reviewStatus(granted.ID))
}

// TestR165OrderExceptionMarkScope is R165 line-2 audit evidence: the exception
// workbench mark routes resolved the source row by raw id with no tenant or
// store filter, so any authenticated account could mark/ignore/unmark another
// tenant's order, and a view-only grant could write marks on its own tenant.
func TestR165OrderExceptionMarkScope(t *testing.T) {
	h := sharedHarness(t)
	cleanup := func() {
		require.NoError(t, h.DB.Exec("DELETE FROM order_exception_marks WHERE remark LIKE 'r165-%'").Error)
		r125Cleanup(t, h)
	}
	cleanup()
	t.Cleanup(cleanup)

	viewOrder := r125SeedOrder(t, h, h.ShopViewOnly, "r165-exc-view")
	grantedOrder := r125SeedOrder(t, h, h.ShopGranted, "r165-exc-granted")
	markCount := func(orderID uuid.UUID) int64 {
		var n int64
		require.NoError(t, h.DB.Table("order_exception_marks").
			Where("source_id = ?", orderID).Count(&n).Error)
		return n
	}
	base := func(orderID uuid.UUID) string {
		return "/api/v1/orders/exceptions/order/" + orderID.String()
	}

	// view-only grant: every mark write is denied with 40303, no rows written.
	for _, p := range []struct{ method, suffix string }{
		{http.MethodPost, "/handle"},
		{http.MethodPost, "/ignore"},
		{http.MethodDelete, "/mark"},
	} {
		w := h.doBody(t, p.method, base(viewOrder.ID)+p.suffix,
			h.Personas[personaViewOnly].Token, `{"exceptionType":"negative_margin","remark":"r165-view"}`)
		require.Equalf(t, http.StatusForbidden, w.Code,
			"%s%s [viewOnlyOperator]: expected 403, got %d: %s", p.method, p.suffix, w.Code, w.Body.String())
		requireCode40303(t, w, p.method+p.suffix)
	}
	require.EqualValues(t, 0, markCount(viewOrder.ID), "denied view-only writes must not create marks")

	// cross-tenant: tenant B admin must not reach a tenant A order.
	for _, p := range []struct{ method, suffix string }{
		{http.MethodPost, "/handle"},
		{http.MethodPost, "/ignore"},
		{http.MethodDelete, "/mark"},
	} {
		w := h.doBody(t, p.method, base(grantedOrder.ID)+p.suffix,
			h.Personas[personaCrossTenant].Token, `{"exceptionType":"negative_margin","remark":"r165-cross"}`)
		require.Equalf(t, http.StatusNotFound, w.Code,
			"%s%s [crossTenant]: expected 404, got %d: %s", p.method, p.suffix, w.Code, w.Body.String())
	}
	require.EqualValues(t, 0, markCount(grantedOrder.ID), "cross-tenant writes must not create marks")

	// Not over-blocked: an operate grant still marks its own store's order.
	w := h.doBody(t, http.MethodPost, base(grantedOrder.ID)+"/handle",
		h.Personas[personaOperator].Token, `{"exceptionType":"negative_margin","remark":"r165-ok"}`)
	require.Equal(t, http.StatusOK, w.Code, "granted operator must mark: %s", w.Body.String())
	require.EqualValues(t, 1, markCount(grantedOrder.ID))
}

// TestR165ShopDeleteStoreOperateScope is R165 line-2 audit evidence: shop
// deletion only filtered by tenant, so any same-tenant operator — including
// one with no grant at all or a view-only grant — could soft-delete a store.
func TestR165ShopDeleteStoreOperateScope(t *testing.T) {
	h := sharedHarness(t)
	alive := func(id uuid.UUID) int64 {
		var n int64
		require.NoError(t, h.DB.Model(&shop.Shop{}).
			Where("id = ? AND deleted_at IS NULL", id).Count(&n).Error)
		return n
	}

	w := h.doBody(t, http.MethodDelete, "/api/v1/shops/"+h.ShopUngranted.String(),
		h.Personas[personaOperator].Token, "{}")
	require.Equalf(t, http.StatusNotFound, w.Code,
		"operator without a grant must get 404, got %d: %s", w.Code, w.Body.String())
	require.EqualValues(t, 1, alive(h.ShopUngranted), "denied delete must not remove the shop")

	w = h.doBody(t, http.MethodDelete, "/api/v1/shops/"+h.ShopViewOnly.String(),
		h.Personas[personaViewOnly].Token, "{}")
	require.Equalf(t, http.StatusForbidden, w.Code,
		"view-only grant must get 403, got %d: %s", w.Code, w.Body.String())
	requireCode40303(t, w, "DELETE /api/v1/shops/:id")
	require.EqualValues(t, 1, alive(h.ShopViewOnly), "denied delete must not remove the shop")

	w = h.doBody(t, http.MethodDelete, "/api/v1/shops/"+h.ShopViewOnly.String(),
		h.Personas[personaCrossTenant].Token, "{}")
	require.Equalf(t, http.StatusNotFound, w.Code,
		"cross-tenant delete must get 404, got %d: %s", w.Code, w.Body.String())
	require.EqualValues(t, 1, alive(h.ShopViewOnly))
}

// TestR165ShopAuthWriteStoreOperateScope is R165 line-2 audit evidence: shop
// authorization writes (stored platform credentials, OAuth refresh/revoke,
// shop-info sync) resolved the shop through the visibility-only scope, so a
// view-only grant could overwrite or revoke a store's platform credentials.
func TestR165ShopAuthWriteStoreOperateScope(t *testing.T) {
	h := sharedHarness(t)

	probes := []struct{ method, path, body string }{
		{http.MethodPut, "/api/v1/shops/" + h.ShopViewOnly.String() + "/auth",
			`{"authType":"api_key","appKey":"r165","appSecret":"r165","accessToken":"r165"}`},
		{http.MethodPost, "/api/v1/shops/" + h.ShopViewOnly.String() + "/oauth/douyin/refresh", "{}"},
		{http.MethodPost, "/api/v1/shops/" + h.ShopViewOnly.String() + "/oauth/douyin/revoke", "{}"},
		{http.MethodPost, "/api/v1/shops/" + h.ShopViewOnly.String() + "/oauth/douyin/sync-shop-info", "{}"},
		{http.MethodPost, "/api/v1/shops/" + h.ShopViewOnly.String() + "/oauth/tiktok/callback",
			`{"code":"r165","state":"r165"}`},
	}
	for _, p := range probes {
		w := h.doBody(t, p.method, p.path, h.Personas[personaViewOnly].Token, p.body)
		require.Equalf(t, http.StatusForbidden, w.Code,
			"%s %s [viewOnlyOperator]: expected 403, got %d: %s", p.method, p.path, w.Code, w.Body.String())
		requireCode40303(t, w, p.method+" "+p.path)
	}
	var tokens int64
	require.NoError(t, h.DB.Table("shop_auth_tokens").
		Where("shop_id = ?", h.ShopViewOnly).Count(&tokens).Error)
	require.EqualValues(t, 0, tokens, "denied auth writes must not store credentials")
}

// TestR165SyncStoreOperateScope is R165 line-2 audit evidence: starting or
// retrying an order / customer-message sync writes to the store and calls the
// platform, but the gate only required visibility (and the order-sync retry had
// no store gate at all), so a view-only grant could trigger platform syncs.
func TestR165SyncStoreOperateScope(t *testing.T) {
	h := sharedHarness(t)

	probes := []string{
		"/api/v1/shops/" + h.ShopViewOnly.String() + "/sync-orders",
		"/api/v1/shops/" + h.ShopViewOnly.String() + "/sync-customer-messages",
	}
	for _, path := range probes {
		w := h.doBody(t, http.MethodPost, path, h.Personas[personaViewOnly].Token, "{}")
		require.Equalf(t, http.StatusForbidden, w.Code,
			"%s [viewOnlyOperator]: expected 403, got %d: %s", path, w.Code, w.Body.String())
		requireCode40303(t, w, path)
	}

	// A sync task of a view-only store must not be retriable either; the store
	// gate has to fire before the task-status check.
	taskID := uuid.New()
	require.NoError(t, h.DB.Exec(
		`INSERT INTO order_sync_tasks (id, tenant_id, shop_id, platform, task_type, status, mode, created_at, updated_at)
		 VALUES (?, ?, ?, 'manual', 'incremental', 'failed', 'sync', NOW(), NOW())`,
		taskID, tenantA, h.ShopViewOnly).Error)
	t.Cleanup(func() { _ = h.DB.Exec("DELETE FROM order_sync_tasks WHERE id = ?", taskID).Error })

	w := h.doBody(t, http.MethodPost,
		"/api/v1/order-sync/tasks/"+taskID.String()+"/retry", h.Personas[personaViewOnly].Token, "{}")
	require.Equalf(t, http.StatusForbidden, w.Code,
		"order-sync retry [viewOnlyOperator]: expected 403, got %d: %s", w.Code, w.Body.String())
	requireCode40303(t, w, "POST /api/v1/order-sync/tasks/:id/retry")
	var status string
	require.NoError(t, h.DB.Table("order_sync_tasks").Where("id = ?", taskID).
		Pluck("status", &status).Error)
	require.Equal(t, "failed", status, "denied retry must not reset the task")
}

// TestR165PublishTargetStoreOperateScope is R165 line-2 audit evidence:
// creating publish drafts / tasks for a target store is a store write that
// reaches the platform, so it must require an operate grant. A view-only grant
// used to reach batch creation.
func TestR165PublishTargetStoreOperateScope(t *testing.T) {
	h := sharedHarness(t)
	cleanup := func() {
		require.NoError(t, h.DB.Exec("DELETE FROM product_publish_batches WHERE tenant_id = ?", tenantA).Error)
	}
	t.Cleanup(cleanup)

	pid := uuid.New()
	require.NoError(t, h.DB.Exec(
		`INSERT INTO products (id, tenant_id, title, status, source, currency, created_at, updated_at)
		 VALUES (?, ?, 'perm-matrix-r165 product', 'draft', 'manual', 'CNY', NOW(), NOW())`,
		pid, tenantA).Error)
	t.Cleanup(func() { _ = h.DB.Exec("DELETE FROM products WHERE id = ?", pid).Error })

	targets := `{"targets":[{"platform":"manual","shopId":"` + h.ShopViewOnly.String() + `"}]}`
	probes := []struct{ path, body string }{
		{"/api/v1/products/" + pid.String() + "/publish-targets/create-drafts", targets},
		{"/api/v1/product-publish/batch-targets/create-drafts",
			`{"productIds":["` + pid.String() + `"],"targets":[{"platform":"manual","shopId":"` + h.ShopViewOnly.String() + `"}]}`},
		{"/api/v1/products/" + pid.String() + "/publish",
			`{"shopId":"` + h.ShopViewOnly.String() + `"}`},
	}
	for _, p := range probes {
		w := h.doBody(t, http.MethodPost, p.path, h.Personas[personaViewOnly].Token, p.body)
		require.Equalf(t, http.StatusForbidden, w.Code,
			"%s [viewOnlyOperator]: expected 403, got %d: %s", p.path, w.Code, w.Body.String())
		requireCode40303(t, w, p.path)
	}
	var batches int64
	require.NoError(t, h.DB.Table("product_publish_batches").
		Where("tenant_id = ?", tenantA).Count(&batches).Error)
	require.EqualValues(t, 0, batches, "denied publish writes must not create batches")
}
