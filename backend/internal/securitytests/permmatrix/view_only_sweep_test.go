package permmatrix

import (
	"fmt"
	"net/http"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"github.com/trademind-ai/trademind/backend/internal/modules/customersync"
	"github.com/trademind-ai/trademind/backend/internal/modules/inventory"
	"github.com/trademind-ai/trademind/backend/internal/modules/operationtask"
	"github.com/trademind-ai/trademind/backend/internal/modules/order"
	"github.com/trademind-ai/trademind/backend/internal/modules/ordersync"
	"github.com/trademind-ai/trademind/backend/internal/modules/procurement"
	"github.com/trademind-ai/trademind/backend/internal/modules/productpublish"
	"gorm.io/datatypes"
)

// TestViewOnlyPersonaShopWriteSweep is the R165 whole-site tier of the
// view-only persona gate: every shop-scoped mutation family (sync tasks,
// task center, operation tasks, procurement, inventory sync, product
// publish, order exceptions, inventory-sync P9) must reject a principal
// whose only grant on the store is scope "view" with 403 (business code
// 40303) and zero mutation, while the same resource stays 404 for a
// principal who cannot see the store at all.
func TestViewOnlyPersonaShopWriteSweep(t *testing.T) {
	h := sharedHarness(t)
	cleanup := func() {
		require.NoError(t, h.DB.Exec("DELETE FROM order_sync_tasks WHERE cursor = 'perm-matrix-r165'").Error)
		require.NoError(t, h.DB.Exec("DELETE FROM customer_message_sync_tasks WHERE cursor = 'perm-matrix-r165'").Error)
		require.NoError(t, h.DB.Exec("DELETE FROM inventory_sync_tasks WHERE batch_no = 'perm-matrix-r165'").Error)
		require.NoError(t, h.DB.Exec("DELETE FROM product_publish_tasks WHERE target_key = 'perm-matrix-r165'").Error)
		require.NoError(t, h.DB.Exec("DELETE FROM operation_tasks WHERE source_reference = 'perm-matrix-r165'").Error)
		require.NoError(t, h.DB.Exec("DELETE FROM purchase_order_items WHERE product_title = 'perm-matrix-r165'").Error)
		require.NoError(t, h.DB.Exec("DELETE FROM purchase_orders WHERE external_order_id = 'perm-matrix-r165'").Error)
		require.NoError(t, h.DB.Exec("DELETE FROM orders WHERE order_no LIKE 'perm-matrix-r165-%'").Error)
	}
	cleanup()
	t.Cleanup(cleanup)

	tok := h.Personas[personaViewOnly].Token
	operatorTok := h.Personas[personaOperator].Token
	sid := h.ShopViewOnly

	// --- fixtures on the view-only store ---
	ost := &ordersync.OrderSyncTask{
		TenantID: tenantA, ShopID: sid, Platform: "manual",
		TaskType: "manual", Status: ordersync.StatusFailed, Mode: "manual",
		Cursor: "perm-matrix-r165",
	}
	require.NoError(t, h.DB.Create(ost).Error)
	cst := &customersync.CustomerMessageSyncTask{
		TenantID: tenantA, ShopID: sid, Platform: "manual",
		TaskType: "manual", Status: customersync.StatusFailed, Mode: "manual",
		Cursor: "perm-matrix-r165",
	}
	require.NoError(t, h.DB.Create(cst).Error)
	ist := &inventory.InventorySyncTask{
		TenantID: tenantA, ShopID: sid, ProductID: uuid.New(), Platform: "manual",
		TaskType: "manual", Status: inventory.StatusFailed, BatchNo: "perm-matrix-r165",
	}
	require.NoError(t, h.DB.Create(ist).Error)
	ppt := &productpublish.ProductPublishTask{
		TenantID: tenantA, ShopID: sid, ProductID: uuid.New(), TargetStoreID: sid,
		Platform: "manual", TaskType: "manual", Status: productpublish.TaskFailed,
		Mode: "manual", TargetKey: "perm-matrix-r165",
	}
	require.NoError(t, h.DB.Create(ppt).Error)
	opShop := sid
	opt := &operationtask.OperationTask{
		TenantID: tenantA, ShopID: &opShop,
		SourceType: operationtask.OperationTaskSourceManual, SourceReference: "perm-matrix-r165",
		TaskType: operationtask.OperationTaskTypeProductContent, Platform: operationtask.PlatformLocal,
		Title: "perm-matrix-r165", Payload: datatypes.JSON(`{}`),
		Status: operationtask.OperationTaskStatusSuggested, Priority: operationtask.OperationTaskPriorityNormal,
	}
	require.NoError(t, h.DB.Create(opt).Error)
	// procurement: purchase order linked to a sales order on the view-only store
	so := &order.Order{
		TenantID: tenantA, Platform: "manual", ShopID: &opShop,
		OrderNo: "perm-matrix-r165-so", CustomerName: "perm-matrix-r165", Status: "pending",
	}
	require.NoError(t, h.DB.Create(so).Error)
	// order held for review on the view-only store (review decision probe)
	ro := &order.Order{
		TenantID: tenantA, Platform: "manual", ShopID: &opShop,
		OrderNo: "perm-matrix-r165-review", CustomerName: "perm-matrix-r165", Status: "pending",
		ReviewStatus: order.ReviewStatusHeld,
	}
	require.NoError(t, h.DB.Create(ro).Error)
	po := &procurement.PurchaseOrder{
		TenantID: tenantA, SupplierID: uuid.New(), SourcePlatform: "1688",
		ExternalOrderID: "perm-matrix-r165", Status: "draft",
	}
	require.NoError(t, h.DB.Create(po).Error)
	poi := &procurement.PurchaseOrderItem{
		TenantID: tenantA, PurchaseOrderID: po.ID, SalesOrderID: &so.ID,
		LocalSKUID: uuid.New(), SourceSKUID: uuid.New(),
		ProductTitle: "perm-matrix-r165", Quantity: 1,
	}
	require.NoError(t, h.DB.Create(poi).Error)

	probes := []struct {
		method, path, body string
	}{
		// manual sync entry points create tasks and upsert business rows
		{http.MethodPost, "/api/v1/shops/" + sid.String() + "/sync-orders", `{}`},
		{http.MethodPost, "/api/v1/shops/" + sid.String() + "/sync-customer-messages", `{}`},
		// sync task retries
		{http.MethodPost, "/api/v1/order-sync/tasks/" + ost.ID.String() + "/retry", `{}`},
		{http.MethodPost, "/api/v1/customer/message-sync/tasks/" + cst.ID.String() + "/retry", `{}`},
		{http.MethodPost, "/api/v1/inventory-sync/tasks/" + ist.ID.String() + "/retry", `{}`},
		{http.MethodPost, "/api/v1/product-publish/tasks/" + ppt.ID.String() + "/retry", `{}`},
		// task-center delegated retries
		{http.MethodPost, "/api/v1/task-center/failures/order_sync/" + ost.ID.String() + "/retry", `{}`},
		{http.MethodPost, "/api/v1/task-center/failures/customer_message_sync/" + cst.ID.String() + "/retry", `{}`},
		{http.MethodPost, "/api/v1/task-center/failures/inventory_sync/" + ist.ID.String() + "/retry", `{}`},
		{http.MethodPost, "/api/v1/task-center/failures/product_publish/" + ppt.ID.String() + "/retry", `{}`},
		// operation tasks
		{http.MethodPost, "/api/v1/operation-tasks", fmt.Sprintf(
			`{"shopId":%q,"sourceType":"manual","taskType":"product_content","platform":"local","title":"perm-matrix-r165-create","payload":{}}`, sid)},
		{http.MethodPost, "/api/v1/operation-tasks/" + opt.ID.String() + "/cancel", `{"reason":"perm-matrix-r165"}`},
		{http.MethodPost, "/api/v1/operation-tasks/" + opt.ID.String() + "/drafts", `{"payload":{},"expectedTaskRevision":1}`},
		// order exceptions resolve the source row back to its store
		{http.MethodPost, "/api/v1/orders/exceptions/order_sync_task/" + ost.ID.String() + "/handle", `{"exceptionType":"sync_failed"}`},
		{http.MethodPost, "/api/v1/orders/exceptions/inventory_sync_task/" + ist.ID.String() + "/ignore", `{"exceptionType":"sync_failed"}`},
		// procurement mutations gated by linked sales-order stores
		{http.MethodPost, "/api/v1/procurement/orders/" + po.ID.String() + "/mark-placed", `{}`},
		{http.MethodPost, "/api/v1/procurement/orders/" + po.ID.String() + "/cancel", `{}`},
		// shop record / credential / OAuth mutations
		{http.MethodPut, "/api/v1/shops/" + sid.String(), `{"shopName":"perm-matrix-r165"}`},
		{http.MethodDelete, "/api/v1/shops/" + sid.String(), `{}`},
		{http.MethodPut, "/api/v1/shops/" + sid.String() + "/auth", `{}`},
		{http.MethodPost, "/api/v1/shops/" + sid.String() + "/oauth/douyin/refresh", `{}`},
		{http.MethodPost, "/api/v1/shops/" + sid.String() + "/oauth/douyin/revoke", `{}`},
		{http.MethodPost, "/api/v1/shops/" + sid.String() + "/oauth/douyin/sync-shop-info", `{}`},
		{http.MethodPost, "/api/v1/shops/" + sid.String() + "/oauth/tiktok/callback", `{"code":"x"}`},
		{http.MethodPost, "/api/v1/shops/" + sid.String() + "/oauth/shopee/callback", `{"code":"x"}`},
		{http.MethodPost, "/api/v1/shops/" + sid.String() + "/oauth/lazada/callback", `{"code":"x"}`},
		{http.MethodPost, "/api/v1/shops/" + sid.String() + "/oauth/amazon/callback", `{"code":"x"}`},
		// inventory-sync P9 run creation
		{http.MethodPost, "/api/v1/inventory-sync/runs", fmt.Sprintf(
			`{"shopConnectionId":%q,"platform":"douyin","providerMode":"mock"}`, sid)},
	}
	for _, p := range probes {
		w := h.doBody(t, p.method, p.path, tok, p.body)
		require.Equalf(t, http.StatusForbidden, w.Code,
			"%s %s [viewOnlyOperator]: expected 403, got %d: %s", p.method, p.path, w.Code, w.Body.String())
		requireCode40303(t, w, p.method+" "+p.path)
	}

	// Order review decisions are batch APIs (200 envelope, per-row results):
	// a view-only store row must fail without flipping the review status.
	for _, action := range []string{"approve", "reject"} {
		w := h.doBody(t, http.MethodPost, "/api/v1/order-review/"+action,
			tok, `{"orderIds":["`+ro.ID.String()+`"]}`)
		require.Equalf(t, http.StatusOK, w.Code, "order-review %s: batch envelope expected: %s", action, w.Body.String())
		require.Containsf(t, w.Body.String(), `"failed":1`,
			"order-review %s must reject the view-only row: %s", action, w.Body.String())
		require.Containsf(t, w.Body.String(), "店铺无操作权限",
			"order-review %s must surface the store-operate denial: %s", action, w.Body.String())
	}

	// The same resources stay 404 for a principal who cannot see the store.
	invisible := []struct {
		method, path string
	}{
		{http.MethodPost, "/api/v1/shops/" + sid.String() + "/sync-orders"},
		{http.MethodPost, "/api/v1/order-sync/tasks/" + ost.ID.String() + "/retry"},
		{http.MethodPost, "/api/v1/operation-tasks/" + opt.ID.String() + "/cancel"},
		{http.MethodPost, "/api/v1/inventory-sync/tasks/" + ist.ID.String() + "/retry"},
	}
	for _, p := range invisible {
		w := h.doBody(t, p.method, p.path, operatorTok, `{"reason":"perm-matrix-r165"}`)
		require.Equalf(t, http.StatusNotFound, w.Code,
			"%s %s [operator without grant]: expected 404, got %d: %s", p.method, p.path, w.Code, w.Body.String())
	}

	// --- zero mutation ---
	requireStatus := func(model any, id uuid.UUID, want, label string) {
		t.Helper()
		var row struct{ Status string }
		require.NoError(t, h.DB.Model(model).Select("status").Where("id = ?", id).Scan(&row).Error)
		require.Equalf(t, want, row.Status, "%s: denied writes must not change status", label)
	}
	requireStatus(&ordersync.OrderSyncTask{}, ost.ID, ordersync.StatusFailed, "order sync task")
	requireStatus(&customersync.CustomerMessageSyncTask{}, cst.ID, customersync.StatusFailed, "customer message sync task")
	requireStatus(&inventory.InventorySyncTask{}, ist.ID, inventory.StatusFailed, "inventory sync task")
	requireStatus(&productpublish.ProductPublishTask{}, ppt.ID, productpublish.TaskFailed, "product publish task")
	requireStatus(&operationtask.OperationTask{}, opt.ID, operationtask.OperationTaskStatusSuggested, "operation task")
	requireStatus(&procurement.PurchaseOrder{}, po.ID, "draft", "purchase order")
	var reviewRow struct{ ReviewStatus string }
	require.NoError(t, h.DB.Model(&order.Order{}).Select("review_status").
		Where("id = ?", ro.ID).Scan(&reviewRow).Error)
	require.Equal(t, order.ReviewStatusHeld, reviewRow.ReviewStatus,
		"denied review decision must not change review status")

	var createdSync int64
	require.NoError(t, h.DB.Model(&ordersync.OrderSyncTask{}).
		Where("shop_id = ? AND id <> ?", sid, ost.ID).Count(&createdSync).Error)
	require.Zero(t, createdSync, "denied manual order sync must not create tasks")
	var createdCust int64
	require.NoError(t, h.DB.Model(&customersync.CustomerMessageSyncTask{}).
		Where("shop_id = ? AND id <> ?", sid, cst.ID).Count(&createdCust).Error)
	require.Zero(t, createdCust, "denied manual customer message sync must not create tasks")
	var createdOp int64
	require.NoError(t, h.DB.Model(&operationtask.OperationTask{}).
		Where("title = ?", "perm-matrix-r165-create").Count(&createdOp).Error)
	require.Zero(t, createdOp, "denied operation-task create must not persist a task")
	var createdRuns int64
	require.NoError(t, h.DB.Table("p9_inventory_sync_runs").
		Where("shop_connection_id = ?", sid).Count(&createdRuns).Error)
	require.Zero(t, createdRuns, "denied P9 run create must not persist a run")

	// Reads keep working: the view grant makes the store's data visible.
	reads := []string{
		"/api/v1/order-sync/tasks/" + ost.ID.String(),
		"/api/v1/customer/message-sync/tasks/" + cst.ID.String(),
		"/api/v1/operation-tasks/" + opt.ID.String(),
	}
	for _, path := range reads {
		w := h.doBody(t, http.MethodGet, path, tok, "")
		require.Equalf(t, http.StatusOK, w.Code,
			"GET %s [viewOnlyOperator]: view-only persona must still read: %s", path, w.Body.String())
	}
}
