package operationtask_test

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
	"github.com/trademind-ai/trademind/backend/internal/modules/admin"
	"github.com/trademind-ai/trademind/backend/internal/modules/operationtask"
	"github.com/trademind-ai/trademind/backend/internal/pkg/adminperm"
	"github.com/trademind-ai/trademind/backend/internal/pkg/ctxkey"
	"gorm.io/gorm"
)

// 回归（R95 UX v4 P1-3）：demo/dev 环境 bootstrap 与 seed 数据落在 tenant 0，
// 运营任务中心开箱即 403。口径：
//   - 读接口按 tenant_id 严格圈定，tenant 0 一律放行（生产环境 tenant 0 无业务数据，仅得空列表）；
//   - 写接口保留 R84 #185 生产闸门（tenant 0 平台管理员不建业务数据），
//     仅 demo/dev 部署（AllowTenantZeroWrites）放开。
func newTenantZeroRouter(t *testing.T, db *gorm.DB, tenantID int64, actorID string, allowWrites bool) *gin.Engine {
	t.Helper()
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(func(c *gin.Context) {
		c.Set(ctxkey.TenantID, tenantID)
		c.Set(ctxkey.AdminID, actorID)
		c.Set(ctxkey.TraceID, "trace-tenant0-gate-test")
		c.Next()
	})
	operationtask.Register(r.Group("/api/v1"), &operationtask.Handler{
		Svc:                   operationtask.NewAPIService(db),
		AllowTenantZeroWrites: allowWrites,
	})
	return r
}

func TestOperationTaskTenantZeroReadAllowedAndScoped(t *testing.T) {
	db := openOperationTaskTestDB(t)
	zeroAdmin := createAdminUser(t, db, 0, admin.RoleAdmin, admin.StatusActive)
	otherAdmin := createAdminUser(t, db, 7, admin.RoleAdmin, admin.StatusActive)

	seeded := sampleTask(0, "tenant0-demo-task")
	require.NoError(t, db.Create(&seeded).Error)
	foreign := sampleTask(7, "tenant7-task")
	foreign.Title = "Foreign tenant task"
	require.NoError(t, db.Create(&foreign).Error)

	// 生产口径（写闸门开启）下读接口仍放行 tenant 0，且仅见 tenant 0 数据。
	r := newTenantZeroRouter(t, db, 0, zeroAdmin.String(), false)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/v1/operation-tasks?limit=50", nil))
	require.Equal(t, http.StatusOK, rec.Code)
	require.Contains(t, rec.Body.String(), seeded.ID.String())
	require.NotContains(t, rec.Body.String(), foreign.ID.String())

	rec = httptest.NewRecorder()
	r.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/v1/operation-tasks/"+seeded.ID.String(), nil))
	require.Equal(t, http.StatusOK, rec.Code)

	// tenant 0 无法越权读取其他租户的任务。
	rec = httptest.NewRecorder()
	r.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/v1/operation-tasks/"+foreign.ID.String(), nil))
	require.Equal(t, http.StatusNotFound, rec.Code)

	// 其他租户也读不到 tenant 0 的任务。
	r7 := newTenantZeroRouter(t, db, 7, otherAdmin.String(), false)
	rec = httptest.NewRecorder()
	r7.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/v1/operation-tasks/"+seeded.ID.String(), nil))
	require.Equal(t, http.StatusNotFound, rec.Code)
}

func TestOperationTaskTenantZeroReadAllowedForAllRoles(t *testing.T) {
	db := openOperationTaskTestDB(t)
	seeded := sampleTask(0, "tenant0-roles-task")
	require.NoError(t, db.Create(&seeded).Error)
	for _, role := range []string{admin.RoleAdmin, adminperm.RoleOperator, adminperm.RoleReadonly} {
		actor := createAdminUser(t, db, 0, role, admin.StatusActive)
		r := newTenantZeroRouter(t, db, 0, actor.String(), false)
		rec := httptest.NewRecorder()
		r.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/v1/operation-tasks?limit=10", nil))
		// 三角色都不再被 tenant 0 闸门 403；店铺范围收敛仍由既有 RBAC 决定
		//（无授权店铺的 operator/readonly 得到空列表而非拒绝）。
		require.Equalf(t, http.StatusOK, rec.Code, "role %s list: %s", role, rec.Body.String())
		if role == admin.RoleAdmin {
			require.Contains(t, rec.Body.String(), seeded.ID.String())
		}
	}
}

func TestOperationTaskTenantZeroWriteGate(t *testing.T) {
	db := openOperationTaskTestDB(t)
	zeroAdmin := createAdminUser(t, db, 0, admin.RoleAdmin, admin.StatusActive)
	body := []byte(`{"sourceType":"manual","taskType":"product_content","platform":"douyin","title":"Tenant zero create","payload":{"title":"safe"},"priority":"normal"}`)

	// 生产口径：tenant 0 写接口统一 403，且不落库。
	prod := newTenantZeroRouter(t, db, 0, zeroAdmin.String(), false)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/operation-tasks", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Idempotency-Key", "tenant0-create-prod")
	rec := httptest.NewRecorder()
	prod.ServeHTTP(rec, req)
	require.Equal(t, http.StatusForbidden, rec.Code)
	var n int64
	require.NoError(t, db.Model(&operationtask.OperationTask{}).Count(&n).Error)
	require.Zero(t, n)

	// demo/dev 口径：tenant 0 写接口放开，任务创建成功。
	demo := newTenantZeroRouter(t, db, 0, zeroAdmin.String(), true)
	req = httptest.NewRequest(http.MethodPost, "/api/v1/operation-tasks", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Idempotency-Key", "tenant0-create-demo")
	rec = httptest.NewRecorder()
	demo.ServeHTTP(rec, req)
	require.Equalf(t, http.StatusCreated, rec.Code, "demo create: %s", rec.Body.String())
	require.NoError(t, db.Model(&operationtask.OperationTask{}).Count(&n).Error)
	require.Equal(t, int64(1), n)
}

func TestOperationTaskTenantZeroServiceStillScopes(t *testing.T) {
	db := openOperationTaskTestDB(t)
	zeroAdmin := createAdminUser(t, db, 0, admin.RoleAdmin, admin.StatusActive)
	svc := operationtask.NewAPIService(db)
	actor := operationtask.APIActor{TenantID: 0, ActorID: zeroAdmin, Role: admin.RoleAdmin}
	created, err := svc.CreateTask(context.Background(), actor, operationtask.CreateTaskRequest{
		SourceType: operationtask.OperationTaskSourceManual,
		TaskType:   operationtask.OperationTaskTypeProductContent,
		Platform:   operationtask.PlatformDouyin,
		Title:      "Tenant zero service task",
		Payload:    json.RawMessage(`{"title":"safe"}`),
		Priority:   operationtask.OperationTaskPriorityNormal,
	}, "req-tenant0-svc", "idem-tenant0-svc")
	require.NoError(t, err)
	require.NotNil(t, created)

	list, err := svc.ListTasks(context.Background(), actor, operationtask.OperationTaskListParams{Limit: 10})
	require.NoError(t, err)
	require.Len(t, list.Items, 1)
}
