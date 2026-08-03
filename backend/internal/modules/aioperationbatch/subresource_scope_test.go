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
