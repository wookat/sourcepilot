package permmatrix

import (
	"fmt"
	"net/http"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"github.com/trademind-ai/trademind/backend/internal/modules/customerchat"
	"github.com/trademind-ai/trademind/backend/internal/modules/migrationimport"
	"github.com/trademind-ai/trademind/backend/internal/modules/ordersync"
)

// TestMigrationImportShopTenantScope pins the tenant gate on the migration
// import wizard (R176 P1-1): the target shop of an import must be resolved
// inside the caller's tenant, so a foreign-tenant (or non-existent) shopId is
// indistinguishable from a missing shop (404) and never opens an import job.
// Before the fix an admin principal skipped store checks entirely, so
// /imports/commit answered 200 and persisted an import_jobs row carrying
// another tenant's shop_id.
func TestMigrationImportShopTenantScope(t *testing.T) {
	h := sharedHarness(t)
	cleanup := func() {
		require.NoError(t, h.DB.Exec("DELETE FROM import_jobs WHERE batch_key LIKE 'r176-%'").Error)
	}
	cleanup()
	t.Cleanup(cleanup)

	crossTok := h.Personas[personaCrossTenant].Token
	adminTok := h.Personas[personaAdmin].Token

	shape := func(shopID, hash string) string {
		return fmt.Sprintf(
			`{"kind":"product","shopId":%q,"columns":["商品名称"],"rows":[["r176"]],"mapping":{"title":0},"fileHash":%q}`,
			shopID, hash)
	}

	// tenant B admin targeting a tenant A shop: 404 on both wizard steps.
	for _, route := range []string{"/api/v1/imports/validate", "/api/v1/imports/commit"} {
		w := h.doBody(t, http.MethodPost, route, crossTok, shape(h.ShopGranted.String(), "r176-cross"))
		require.Equalf(t, http.StatusNotFound, w.Code,
			"POST %s [crossTenantAdmin -> tenant A shop]: foreign shop must stay 404, got %d: %s",
			route, w.Code, w.Body.String())

		// non-existent shop id: same answer, no existence oracle.
		w = h.doBody(t, http.MethodPost, route, adminTok, shape(uuid.NewString(), "r176-missing"))
		require.Equalf(t, http.StatusNotFound, w.Code,
			"POST %s [admin -> unknown shop]: unknown shop must be 404, got %d: %s",
			route, w.Code, w.Body.String())
	}

	// Zero mutation: no import job may reference a foreign-tenant shop.
	var jobs int64
	require.NoError(t, h.DB.Model(&migrationimport.ImportJob{}).
		Where("tenant_id <> ? AND shop_id = ?", tenantA, h.ShopGranted).Count(&jobs).Error)
	require.Zero(t, jobs, "denied import must not persist a job for a foreign-tenant shop")
}

// TestCustomerChatReplyScopeBeforeBody extends the site-wide "scope before
// body" order (#347/#349) to the customer-service reply routes (R176):
// mark-replied and send-platform-message must answer the store-operate gate
// before validating the reply payload, so a view-only principal gets
// 403/40303 and an invisible conversation stays 404 on an empty body.
func TestCustomerChatReplyScopeBeforeBody(t *testing.T) {
	h := sharedHarness(t)
	cleanup := func() {
		require.NoError(t, h.DB.Exec("DELETE FROM customer_conversations WHERE customer_name = 'perm-matrix-r176'").Error)
	}
	cleanup()
	t.Cleanup(cleanup)

	viewShop := h.ShopViewOnly
	grantedShop := h.ShopGranted
	convVO := &customerchat.CustomerConversation{TenantID: tenantA, ShopID: &viewShop, Platform: "manual",
		CustomerName: "perm-matrix-r176", Status: "open"}
	require.NoError(t, h.DB.Create(convVO).Error)
	convGranted := &customerchat.CustomerConversation{TenantID: tenantA, ShopID: &grantedShop, Platform: "manual",
		CustomerName: "perm-matrix-r176", Status: "open"}
	require.NoError(t, h.DB.Create(convGranted).Error)

	viewTok := h.Personas[personaViewOnly].Token
	operatorTok := h.Personas[personaOperator].Token

	for _, action := range []string{"mark-replied", "send-platform-message"} {
		path := "/api/v1/customer/conversations/" + convVO.ID.String() + "/" + action
		w := h.doBody(t, http.MethodPost, path, viewTok, `{}`)
		require.Equalf(t, http.StatusForbidden, w.Code,
			"POST %s [viewOnlyOperator, empty body]: scope gate must answer before body validation, got %d: %s",
			path, w.Code, w.Body.String())
		requireCode40303(t, w, "POST "+path+" (empty body)")

		w = h.doBody(t, http.MethodPost, path, operatorTok, `{}`)
		require.Equalf(t, http.StatusNotFound, w.Code,
			"POST %s [operator without grant, empty body]: invisible conversation must stay 404, got %d: %s",
			path, w.Code, w.Body.String())

		// operable store keeps enforcing the payload contract
		granted := "/api/v1/customer/conversations/" + convGranted.ID.String() + "/" + action
		w = h.doBody(t, http.MethodPost, granted, operatorTok, `{}`)
		require.Equalf(t, http.StatusBadRequest, w.Code,
			"POST %s [operator, operable store, empty body]: expected 400 payload validation, got %d: %s",
			granted, w.Code, w.Body.String())
	}

	var msgs int64
	require.NoError(t, h.DB.Table("customer_messages").
		Where("conversation_id = ?", convVO.ID).Count(&msgs).Error)
	require.Zero(t, msgs, "denied reply must not persist a customer message")
}

// TestExceptionBindSKUScopeBeforeBody closes the last exception-workbench
// route that still bound the JSON body before the store-operate gate (R176):
// bind-sku must answer 403/40303 for a view-only source and 404 for an
// invisible source even when the body is not valid JSON.
func TestExceptionBindSKUScopeBeforeBody(t *testing.T) {
	h := sharedHarness(t)
	cleanup := func() {
		require.NoError(t, h.DB.Exec("DELETE FROM order_sync_tasks WHERE cursor = 'perm-matrix-r176'").Error)
	}
	cleanup()
	t.Cleanup(cleanup)

	viewTask := &ordersync.OrderSyncTask{TenantID: tenantA, ShopID: h.ShopViewOnly, Platform: "manual",
		TaskType: "manual", Status: ordersync.StatusFailed, Mode: "manual", Cursor: "perm-matrix-r176"}
	require.NoError(t, h.DB.Create(viewTask).Error)
	grantedTask := &ordersync.OrderSyncTask{TenantID: tenantA, ShopID: h.ShopGranted, Platform: "manual",
		TaskType: "manual", Status: ordersync.StatusFailed, Mode: "manual", Cursor: "perm-matrix-r176"}
	require.NoError(t, h.DB.Create(grantedTask).Error)

	viewTok := h.Personas[personaViewOnly].Token
	operatorTok := h.Personas[personaOperator].Token
	path := "/api/v1/orders/exceptions/order_sync_task/" + viewTask.ID.String() + "/bind-sku"

	w := h.doBody(t, http.MethodPost, path, viewTok, `{`)
	require.Equalf(t, http.StatusForbidden, w.Code,
		"POST %s [viewOnlyOperator, malformed body]: scope gate must answer before body binding, got %d: %s",
		path, w.Code, w.Body.String())
	requireCode40303(t, w, "POST "+path+" (malformed body)")

	w = h.doBody(t, http.MethodPost, path, operatorTok, `{`)
	require.Equalf(t, http.StatusNotFound, w.Code,
		"POST %s [operator without grant, malformed body]: invisible source must stay 404, got %d: %s",
		path, w.Code, w.Body.String())

	granted := "/api/v1/orders/exceptions/order_sync_task/" + grantedTask.ID.String() + "/bind-sku"
	w = h.doBody(t, http.MethodPost, granted, operatorTok, `{`)
	require.Equalf(t, http.StatusBadRequest, w.Code,
		"POST %s [operator, operable source, malformed body]: expected 400, got %d: %s",
		granted, w.Code, w.Body.String())
}
