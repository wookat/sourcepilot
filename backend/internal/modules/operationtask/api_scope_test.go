package operationtask_test

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"github.com/trademind-ai/trademind/backend/internal/modules/admin"
	"github.com/trademind-ai/trademind/backend/internal/modules/operationtask"
	"github.com/trademind-ai/trademind/backend/internal/modules/shop"
	"github.com/trademind-ai/trademind/backend/internal/pkg/ctxkey"
	"gorm.io/gorm"
)

func createScopeShop(t *testing.T, db *gorm.DB, tenantID int64) uuid.UUID {
	t.Helper()
	require.NoError(t, db.AutoMigrate(&shop.Shop{}))
	s := shop.Shop{TenantID: tenantID, Platform: "douyin", ShopName: "scope-shop-" + uuid.NewString(), Status: "active", AuthStatus: "authorized"}
	require.NoError(t, db.Create(&s).Error)
	return s.ID
}

func createScopedTask(t *testing.T, svc *operationtask.APIService, actor operationtask.APIActor, shopID *uuid.UUID, tag string) *operationtask.OperationTaskDetailResponse {
	t.Helper()
	req := operationtask.CreateTaskRequest{
		SourceType: operationtask.OperationTaskSourceManual,
		TaskType:   operationtask.OperationTaskTypeProductContent,
		Platform:   operationtask.PlatformDouyin,
		Title:      "Scoped task " + tag,
		Payload:    json.RawMessage(`{"title":"safe"}`),
		Priority:   operationtask.OperationTaskPriorityNormal,
	}
	if shopID != nil {
		req.ShopID = shopID.String()
	}
	created, err := svc.CreateTask(context.Background(), actor, req, "req-scope-"+tag, "idem-scope-"+tag)
	require.NoError(t, err)
	return created
}

func TestOperationTaskShopScopeReadPaths(t *testing.T) {
	db := openOperationTaskTestDB(t)
	tenantID := int64(101)
	adminID := createAdminUser(t, db, tenantID, admin.RoleAdmin, admin.StatusActive)
	operatorID := createAdminUser(t, db, tenantID, "operator", admin.StatusActive)
	readonlyID := createAdminUser(t, db, tenantID, "readonly", admin.StatusActive)
	crossAdminID := createAdminUser(t, db, 202, admin.RoleAdmin, admin.StatusActive)
	svc := operationtask.NewAPIService(db)

	shopA := createScopeShop(t, db, tenantID)
	shopB := createScopeShop(t, db, tenantID)
	adminActor := operationtask.APIActor{TenantID: tenantID, ActorID: adminID, Role: admin.RoleAdmin}
	taskA := createScopedTask(t, svc, adminActor, &shopA, "a")
	taskB := createScopedTask(t, svc, adminActor, &shopB, "b")
	tenantTask := createScopedTask(t, svc, adminActor, nil, "tenant")

	adminList, err := svc.ListTasks(context.Background(), adminActor, operationtask.OperationTaskListParams{Limit: 10})
	require.NoError(t, err)
	require.Len(t, adminList.Items, 3)

	// Operator authorized for shop A sees only shop A tasks.
	opActor := operationtask.APIActor{TenantID: tenantID, ActorID: operatorID, Role: "operator", AllowedShopIDs: []uuid.UUID{shopA}}
	opList, err := svc.ListTasks(context.Background(), opActor, operationtask.OperationTaskListParams{Limit: 10})
	require.NoError(t, err)
	require.Len(t, opList.Items, 1)
	require.Equal(t, taskA.ID, opList.Items[0].ID)

	// Direct reads outside scope surface as not found (no existence leak).
	_, err = svc.GetTask(context.Background(), opActor, taskB.ID)
	require.ErrorIs(t, err, operationtask.ErrNotFound)
	_, err = svc.GetTask(context.Background(), opActor, tenantTask.ID)
	require.ErrorIs(t, err, operationtask.ErrNotFound)
	_, err = svc.ListDrafts(context.Background(), opActor, taskB.ID, 10)
	require.ErrorIs(t, err, operationtask.ErrNotFound)
	_, err = svc.ListAttempts(context.Background(), opActor, taskB.ID, 10, "")
	require.ErrorIs(t, err, operationtask.ErrNotFound)
	_, err = svc.ListEvents(context.Background(), opActor, taskB.ID, 10, 0)
	require.ErrorIs(t, err, operationtask.ErrNotFound)
	_, err = svc.GetTask(context.Background(), opActor, taskA.ID)
	require.NoError(t, err)

	// Readonly with no authorized shops sees an empty list and 404 details.
	roActor := operationtask.APIActor{TenantID: tenantID, ActorID: readonlyID, Role: "readonly", AllowedShopIDs: []uuid.UUID{}}
	roList, err := svc.ListTasks(context.Background(), roActor, operationtask.OperationTaskListParams{Limit: 10})
	require.NoError(t, err)
	require.Empty(t, roList.Items)
	_, err = svc.GetTask(context.Background(), roActor, taskA.ID)
	require.ErrorIs(t, err, operationtask.ErrNotFound)

	// Cross-tenant access surfaces as not found even for an admin role.
	crossActor := operationtask.APIActor{TenantID: 202, ActorID: crossAdminID, Role: admin.RoleAdmin}
	_, err = svc.GetTask(context.Background(), crossActor, taskA.ID)
	require.ErrorIs(t, err, operationtask.ErrNotFound)
	crossList, err := svc.ListTasks(context.Background(), crossActor, operationtask.OperationTaskListParams{Limit: 10})
	require.NoError(t, err)
	require.Empty(t, crossList.Items)
}

func TestOperationTaskShopScopeWritePaths(t *testing.T) {
	db := openOperationTaskTestDB(t)
	tenantID := int64(101)
	adminID := createAdminUser(t, db, tenantID, admin.RoleAdmin, admin.StatusActive)
	operatorID := createAdminUser(t, db, tenantID, "operator", admin.StatusActive)
	reviewerID := createAdminUser(t, db, tenantID, "reviewer", admin.StatusActive)
	svc := operationtask.NewAPIService(db)

	shopA := createScopeShop(t, db, tenantID)
	shopB := createScopeShop(t, db, tenantID)
	adminActor := operationtask.APIActor{TenantID: tenantID, ActorID: adminID, Role: admin.RoleAdmin}
	taskB := createScopedTask(t, svc, adminActor, &shopB, "wb")

	opActor := operationtask.APIActor{TenantID: tenantID, ActorID: operatorID, Role: "operator", AllowedShopIDs: []uuid.UUID{shopA}}
	_, err := svc.CreateInitialDraft(context.Background(), opActor, taskB.ID, operationtask.CreateDraftRequest{Payload: json.RawMessage(`{"draft":{"title":"safe"}}`)}, "req-w1", "idem-w1")
	require.ErrorIs(t, err, operationtask.ErrNotFound)
	_, err = svc.EditLatestDraft(context.Background(), opActor, taskB.ID, operationtask.EditDraftRequest{Payload: json.RawMessage(`{"draft":{"title":"safe"}}`)}, "req-w2", "idem-w2")
	require.ErrorIs(t, err, operationtask.ErrNotFound)
	_, err = svc.Cancel(context.Background(), opActor, taskB.ID, operationtask.CancelTaskRequest{Reason: "out of scope"}, "req-w3", "idem-w3")
	require.ErrorIs(t, err, operationtask.ErrNotFound)

	revActor := operationtask.APIActor{TenantID: tenantID, ActorID: reviewerID, Role: "reviewer", AllowedShopIDs: []uuid.UUID{shopA}}
	_, err = svc.Approve(context.Background(), revActor, taskB.ID, operationtask.ApprovalRequest{DraftVersion: 1, DraftPayloadHash: "0000000000000000000000000000000000000000000000000000000000000000"}, "req-w4", "idem-w4")
	require.ErrorIs(t, err, operationtask.ErrNotFound)
	_, err = svc.Reject(context.Background(), revActor, taskB.ID, operationtask.ApprovalRequest{DraftVersion: 1, DraftPayloadHash: "0000000000000000000000000000000000000000000000000000000000000000", Reason: "scope"}, "req-w5", "idem-w5")
	require.ErrorIs(t, err, operationtask.ErrNotFound)
	_, err = svc.Execute(context.Background(), revActor, taskB.ID, operationtask.ExecuteRequest{}, "req-w6", "idem-w6")
	require.ErrorIs(t, err, operationtask.ErrNotFound)
	_, err = svc.RetryExecution(context.Background(), revActor, taskB.ID, operationtask.RetryRequest{Reason: "scope retry"}, "req-w7", "idem-w7")
	require.ErrorIs(t, err, operationtask.ErrNotFound)
}

func TestOperationTaskCreateShopBinding(t *testing.T) {
	db := openOperationTaskTestDB(t)
	tenantID := int64(101)
	adminID := createAdminUser(t, db, tenantID, admin.RoleAdmin, admin.StatusActive)
	operatorID := createAdminUser(t, db, tenantID, "operator", admin.StatusActive)
	svc := operationtask.NewAPIService(db)
	shopA := createScopeShop(t, db, tenantID)
	foreignShop := createScopeShop(t, db, 202)

	baseReq := operationtask.CreateTaskRequest{
		SourceType: operationtask.OperationTaskSourceManual,
		TaskType:   operationtask.OperationTaskTypeProductContent,
		Platform:   operationtask.PlatformDouyin,
		Title:      "Create binding",
		Payload:    json.RawMessage(`{"title":"safe"}`),
		Priority:   operationtask.OperationTaskPriorityNormal,
	}

	// Scoped roles must bind an authorized shop.
	opActor := operationtask.APIActor{TenantID: tenantID, ActorID: operatorID, Role: "operator", AllowedShopIDs: []uuid.UUID{shopA}}
	_, err := svc.CreateTask(context.Background(), opActor, baseReq, "req-cb1", "idem-cb1")
	require.ErrorIs(t, err, operationtask.ErrValidation)

	unauthorized := baseReq
	unauthorized.ShopID = foreignShop.String()
	_, err = svc.CreateTask(context.Background(), opActor, unauthorized, "req-cb2", "idem-cb2")
	require.ErrorIs(t, err, operationtask.ErrNotFound)

	// Admin cannot bind a shop from another tenant.
	adminActor := operationtask.APIActor{TenantID: tenantID, ActorID: adminID, Role: admin.RoleAdmin}
	crossTenant := baseReq
	crossTenant.ShopID = foreignShop.String()
	_, err = svc.CreateTask(context.Background(), adminActor, crossTenant, "req-cb3", "idem-cb3")
	require.ErrorIs(t, err, operationtask.ErrNotFound)

	allowed := baseReq
	allowed.ShopID = shopA.String()
	created, err := svc.CreateTask(context.Background(), opActor, allowed, "req-cb4", "idem-cb4")
	require.NoError(t, err)
	require.NotNil(t, created.ShopID)
	require.Equal(t, shopA, *created.ShopID)

	// Admin may create tenant-level tasks without a shop binding.
	tenantLevel, err := svc.CreateTask(context.Background(), adminActor, baseReq, "req-cb5", "idem-cb5")
	require.NoError(t, err)
	require.Nil(t, tenantLevel.ShopID)
}

func TestOperationTaskShopBackfill(t *testing.T) {
	db := openOperationTaskTestDB(t)
	tenantID := int64(101)
	shopA := createScopeShop(t, db, tenantID)
	shopB := createScopeShop(t, db, tenantID)

	require.NoError(t, db.Exec(`CREATE TABLE IF NOT EXISTS products (id char(36) PRIMARY KEY, tenant_id bigint NOT NULL)`).Error)
	require.NoError(t, db.Exec(`CREATE TABLE IF NOT EXISTS product_platform_publish_configs (product_id char(36), shop_id char(36))`).Error)
	require.NoError(t, db.Exec(`CREATE TABLE IF NOT EXISTS product_publications (product_id char(36), shop_id char(36), deleted_at datetime)`).Error)

	singleShopProduct := uuid.New()
	multiShopProduct := uuid.New()
	require.NoError(t, db.Exec(`INSERT INTO products (id, tenant_id) VALUES (?, ?), (?, ?)`, singleShopProduct.String(), tenantID, multiShopProduct.String(), tenantID).Error)
	require.NoError(t, db.Exec(`INSERT INTO product_platform_publish_configs (product_id, shop_id) VALUES (?, ?)`, singleShopProduct.String(), shopA.String()).Error)
	require.NoError(t, db.Exec(`INSERT INTO product_publications (product_id, shop_id, deleted_at) VALUES (?, ?, NULL), (?, ?, NULL)`, multiShopProduct.String(), shopA.String(), multiShopProduct.String(), shopB.String()).Error)

	repo := operationtask.NewOperationTaskRepository(db)
	mk := func(ref, idem string) uuid.UUID {
		task := sampleTask(tenantID, idem)
		task.SourceReference = ref
		require.NoError(t, repo.Create(context.Background(), &task))
		return task.ID
	}
	shopRefTask := mk(shopA.String(), "idem-backfill-shop")
	productRefTask := mk(singleShopProduct.String(), "idem-backfill-product")
	ambiguousTask := mk(multiShopProduct.String(), "idem-backfill-ambiguous")
	unknownTask := mk("free-text-reference", "idem-backfill-unknown")

	require.NoError(t, operationtask.Migrate(db))

	shopOf := func(id uuid.UUID) *uuid.UUID {
		task, err := repo.GetByID(context.Background(), tenantID, id)
		require.NoError(t, err)
		return task.ShopID
	}
	require.NotNil(t, shopOf(shopRefTask))
	require.Equal(t, shopA, *shopOf(shopRefTask))
	require.NotNil(t, shopOf(productRefTask))
	require.Equal(t, shopA, *shopOf(productRefTask))
	require.Nil(t, shopOf(ambiguousTask))
	require.Nil(t, shopOf(unknownTask))
}

func TestExecuteAdapterValidationFailureReturns4xx(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db := openOperationTaskTestDB(t)
	task, _, _, _ := createApprovedExecutionTask(t, db)
	actorID := createAdminUser(t, db, task.TenantID, admin.RoleAdmin, admin.StatusActive)
	port := newRecordingExecutionPort()
	port.err = &operationtask.ExecutionDomainError{Category: operationtask.ExecutionErrorCategoryValidation, Code: "validation_rejected", SafeMessage: "Draft payload validation failed", Retryable: false}
	svc := operationtask.NewAPIService(db)
	svc.Executor = operationtask.NewExecutionOrchestrator(db, operationtask.NewRBACAuthorizer(db), port)
	svc.Retry = operationtask.NewManualRetryService(db, svc.Executor, 3)

	r := gin.New()
	r.Use(func(c *gin.Context) {
		c.Set(ctxkey.TenantID, task.TenantID)
		c.Set(ctxkey.AdminID, actorID.String())
		c.Set(ctxkey.TraceID, uuid.NewString())
		c.Next()
	})
	operationtask.Register(r.Group("/api/v1"), &operationtask.Handler{Svc: svc})

	req := httptest.NewRequest(http.MethodPost, fmt.Sprintf("/api/v1/operation-tasks/%s/execute", task.ID), bytes.NewReader([]byte(`{}`)))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Idempotency-Key", "idem-exec-validation-4xx")
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	require.Equal(t, http.StatusBadRequest, rec.Code)
	require.Contains(t, rec.Body.String(), operationtask.ErrCodeExecutionValidationFailed)
	require.NotContains(t, rec.Body.String(), "50000")
	require.NotContains(t, rec.Body.String(), "internal error")

	// The failed attempt must still be persisted.
	attempts, err := operationtask.NewExecutionAttemptRepository(db).ListByTask(context.Background(), task.TenantID, task.ID)
	require.NoError(t, err)
	require.Len(t, attempts, 1)
	require.Equal(t, operationtask.ExecutionAttemptStatusFailed, attempts[0].Status)
}
