package aioperationbatch

import (
	"errors"
	"net/http/httptest"
	"path/filepath"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"github.com/google/uuid"
	"gorm.io/gorm"

	"github.com/trademind-ai/trademind/backend/internal/modules/admin"
	"github.com/trademind-ai/trademind/backend/internal/pkg/ctxkey"
	"github.com/trademind-ai/trademind/backend/internal/pkg/model"
)

func seedBatchAdmin(t *testing.T, db *gorm.DB, tenantID int64) uuid.UUID {
	t.Helper()
	uid := uuid.New()
	if err := db.Create(&admin.AdminUser{
		Base:         model.Base{ID: uid},
		TenantID:     tenantID,
		Username:     "u-" + uid.String()[:8],
		Email:        "u-" + uid.String()[:8] + "@example.com",
		PasswordHash: "x",
		Role:         "admin",
		Status:       "active",
	}).Error; err != nil {
		t.Fatal(err)
	}
	return uid
}

// Regression (R71): batch subresources (GET /:id, /:id/tasks, retry,
// apply-results) must verify the batch's tenant via its creator and return
// 404 for cross-tenant access.
func TestAIOperationBatchScopedByTenant(t *testing.T) {
	dsn := filepath.Join(t.TempDir(), "scope.db")
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Skipf("sqlite unavailable: %v", err)
	}
	if err := db.AutoMigrate(&admin.AdminUser{}, &AIOperationBatch{}); err != nil {
		t.Fatal(err)
	}

	creatorT1 := seedBatchAdmin(t, db, 1)
	batch := &AIOperationBatch{BatchNo: "AI202501010001", OperationType: OperationTitleOptimize, Status: StatusSuccess, CreatedBy: &creatorT1}
	if err := db.Create(batch).Error; err != nil {
		t.Fatal(err)
	}
	legacy := &AIOperationBatch{BatchNo: "AI202501010002", OperationType: OperationTitleOptimize, Status: StatusSuccess}
	if err := db.Create(legacy).Error; err != nil {
		t.Fatal(err)
	}

	gin.SetMode(gin.TestMode)
	svc := &Service{DB: db}
	ginCtx := func(tid int64) *gin.Context {
		c, _ := gin.CreateTestContext(httptest.NewRecorder())
		c.Request = httptest.NewRequest("GET", "/", nil)
		c.Set(ctxkey.TenantID, tid)
		return c
	}

	// 同租户 —— 正常读取
	if _, err := svc.GetScoped(ginCtx(1), batch.ID); err != nil {
		t.Fatalf("same-tenant get batch: %v", err)
	}
	// 跨租户 —— 404，不泄露存在性
	if _, err := svc.GetScoped(ginCtx(2), batch.ID); !errors.Is(err, gorm.ErrRecordNotFound) {
		t.Fatalf("cross-tenant get batch: want ErrRecordNotFound, got %v", err)
	}
	// 存量无创建人批次按租户 0 归属
	if _, err := svc.GetScoped(ginCtx(0), legacy.ID); err != nil {
		t.Fatalf("tenant-0 legacy batch: %v", err)
	}
	if _, err := svc.GetScoped(ginCtx(1), legacy.ID); !errors.Is(err, gorm.ErrRecordNotFound) {
		t.Fatalf("tenant-1 legacy batch: want ErrRecordNotFound, got %v", err)
	}
	// 跨租户 apply-results —— 404 且无副作用
	if _, err := svc.ApplyBatchResults(ginCtx(2), batch.ID, ApplyBatchResultsBody{Target: "ai_field"}, nil); !errors.Is(err, gorm.ErrRecordNotFound) {
		t.Fatalf("cross-tenant apply-results: want ErrRecordNotFound, got %v", err)
	}
}

// Regression (R72): ai_operation_batches now carries tenant_id; the list
// endpoint must filter by tenant and detail/subresources must scope by the
// column (with creator fallback for not-yet-backfilled tenant-0 rows).
func TestAIOperationBatchTenantColumnScope(t *testing.T) {
	dsn := filepath.Join(t.TempDir(), "scope72.db")
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Skipf("sqlite unavailable: %v", err)
	}
	if err := db.AutoMigrate(&admin.AdminUser{}, &AIOperationBatch{}); err != nil {
		t.Fatal(err)
	}

	creatorT1 := seedBatchAdmin(t, db, 1)
	operatorT1 := uuid.New()
	if err := db.Create(&admin.AdminUser{
		Base:         model.Base{ID: operatorT1},
		TenantID:     1,
		Username:     "op-" + operatorT1.String()[:8],
		Email:        "op-" + operatorT1.String()[:8] + "@example.com",
		PasswordHash: "x",
		Role:         "operator",
		Status:       "active",
	}).Error; err != nil {
		t.Fatal(err)
	}
	b1 := &AIOperationBatch{TenantID: 1, BatchNo: "AI202502010001", OperationType: OperationTitleOptimize, Status: StatusSuccess, CreatedBy: &creatorT1}
	b2 := &AIOperationBatch{TenantID: 2, BatchNo: "AI202502010002", OperationType: OperationTitleOptimize, Status: StatusSuccess}
	legacy := &AIOperationBatch{TenantID: 0, BatchNo: "AI202502010003", OperationType: OperationTitleOptimize, Status: StatusSuccess}
	for _, b := range []*AIOperationBatch{b1, b2, legacy} {
		if err := db.Create(b).Error; err != nil {
			t.Fatal(err)
		}
	}

	gin.SetMode(gin.TestMode)
	svc := &Service{DB: db}
	ginCtx := func(tid int64, adminID string) *gin.Context {
		c, _ := gin.CreateTestContext(httptest.NewRecorder())
		c.Request = httptest.NewRequest("GET", "/", nil)
		c.Set(ctxkey.TenantID, tid)
		if adminID != "" {
			c.Set(ctxkey.AdminID, adminID)
		}
		return c
	}

	// 角色1：同租户 admin —— 列表仅见本租户批次
	items, total, err := svc.ListBatches(ginCtx(1, creatorT1.String()), ListBatchesQuery{Page: 1, PageSize: 20})
	if err != nil || total != 1 || len(items) != 1 || items[0].ID != b1.ID {
		t.Fatalf("tenant-1 list: want only b1, got total=%d err=%v", total, err)
	}
	// 角色2：跨租户 admin —— 看不到租户 1 的批次
	items, total, err = svc.ListBatches(ginCtx(2, ""), ListBatchesQuery{Page: 1, PageSize: 20})
	if err != nil || total != 1 || len(items) != 1 || items[0].ID != b2.ID {
		t.Fatalf("tenant-2 list: want only b2, got total=%d err=%v", total, err)
	}
	// 角色3：同租户 operator —— 与 admin 同租户口径（批次无店铺维度）
	items, total, err = svc.ListBatches(ginCtx(1, operatorT1.String()), ListBatchesQuery{Page: 1, PageSize: 20})
	if err != nil || total != 1 || len(items) != 1 || items[0].ID != b1.ID {
		t.Fatalf("tenant-1 operator list: want only b1, got total=%d err=%v", total, err)
	}
	// 缺租户上下文 —— 报错而非放行
	badCtx, _ := gin.CreateTestContext(httptest.NewRecorder())
	badCtx.Request = httptest.NewRequest("GET", "/", nil)
	if _, _, err := svc.ListBatches(badCtx, ListBatchesQuery{Page: 1, PageSize: 20}); err == nil {
		t.Fatal("missing tenant context must fail list")
	}

	// 详情按 tenant_id 列 scope：跨租户 404
	if _, err := svc.GetScoped(ginCtx(2, ""), b1.ID); !errors.Is(err, gorm.ErrRecordNotFound) {
		t.Fatalf("cross-tenant get b1: want ErrRecordNotFound, got %v", err)
	}
	if _, err := svc.GetScoped(ginCtx(1, ""), b1.ID); err != nil {
		t.Fatalf("same-tenant get b1: %v", err)
	}
	// 无创建人的存量批次按租户 0 归属
	if _, err := svc.GetScoped(ginCtx(0, ""), legacy.ID); err != nil {
		t.Fatalf("tenant-0 legacy batch: %v", err)
	}
	if _, err := svc.GetScoped(ginCtx(1, ""), legacy.ID); !errors.Is(err, gorm.ErrRecordNotFound) {
		t.Fatalf("tenant-1 legacy batch: want ErrRecordNotFound, got %v", err)
	}

	// backfill 口径：tenant_id=0 且有创建人 → 归属创建人租户
	unfilled := &AIOperationBatch{TenantID: 0, BatchNo: "AI202502010004", OperationType: OperationTitleOptimize, Status: StatusSuccess, CreatedBy: &creatorT1}
	if err := db.Create(unfilled).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Exec(`UPDATE ai_operation_batches SET tenant_id = (SELECT u.tenant_id FROM admin_users u WHERE u.id = ai_operation_batches.created_by) WHERE created_by IS NOT NULL AND (tenant_id IS NULL OR tenant_id = 0)`).Error; err != nil {
		t.Fatalf("backfill: %v", err)
	}
	var got AIOperationBatch
	if err := db.First(&got, "id = ?", unfilled.ID).Error; err != nil || got.TenantID != 1 {
		t.Fatalf("backfill result: want tenant 1, got %d err=%v", got.TenantID, err)
	}
	// 未 backfill 的 tenant-0 行（创建人在）走创建人租户回退口径
	unfilled2 := &AIOperationBatch{TenantID: 0, BatchNo: "AI202502010005", OperationType: OperationTitleOptimize, Status: StatusSuccess, CreatedBy: &creatorT1}
	if err := db.Create(unfilled2).Error; err != nil {
		t.Fatal(err)
	}
	if _, err := svc.GetScoped(ginCtx(1, ""), unfilled2.ID); err != nil {
		t.Fatalf("fallback same-tenant get: %v", err)
	}
	if _, err := svc.GetScoped(ginCtx(2, ""), unfilled2.ID); !errors.Is(err, gorm.ErrRecordNotFound) {
		t.Fatalf("fallback cross-tenant get: want ErrRecordNotFound, got %v", err)
	}
}
