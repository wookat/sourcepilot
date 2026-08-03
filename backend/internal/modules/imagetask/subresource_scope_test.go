package imagetask

import (
	"errors"
	"net/http/httptest"
	"path/filepath"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"gorm.io/gorm"

	"github.com/trademind-ai/trademind/backend/internal/modules/product"
	"github.com/trademind-ai/trademind/backend/internal/pkg/ctxkey"
)

// Regression (R71): image task items must verify the parent task's product
// tenant scope and return 404 for cross-tenant access.
func TestImageTaskItemsScopedByTenant(t *testing.T) {
	dsn := filepath.Join(t.TempDir(), "scope.db")
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Skipf("sqlite unavailable: %v", err)
	}
	if err := db.AutoMigrate(&product.Product{}, &ImageTask{}, &ImageTaskItem{}); err != nil {
		t.Fatal(err)
	}

	prod := &product.Product{TenantID: 1, Source: "manual", Title: "p1", Status: "draft"}
	if err := db.Create(prod).Error; err != nil {
		t.Fatal(err)
	}
	pid := prod.ID
	task := &ImageTask{TaskType: "remove_background", Provider: "local", Status: "success", ProductID: &pid}
	if err := db.Create(task).Error; err != nil {
		t.Fatal(err)
	}
	item := &ImageTaskItem{TaskID: task.ID, Status: "success"}
	if err := db.Create(item).Error; err != nil {
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
	if items, err := svc.ListTaskItems(ginCtx(1), task.ID); err != nil || len(items) != 1 {
		t.Fatalf("same-tenant list items: items=%d err=%v", len(items), err)
	}
	// 跨租户 —— 404，不泄露存在性
	if _, err := svc.ListTaskItems(ginCtx(2), task.ID); !errors.Is(err, gorm.ErrRecordNotFound) {
		t.Fatalf("cross-tenant list items: want ErrRecordNotFound, got %v", err)
	}
	// 跨租户删除 —— 404 且不落库
	if err := svc.DeleteTaskItem(ginCtx(2), task.ID, item.ID); !errors.Is(err, gorm.ErrRecordNotFound) {
		t.Fatalf("cross-tenant delete item: want ErrRecordNotFound, got %v", err)
	}
	var cnt int64
	if err := db.Model(&ImageTaskItem{}).Count(&cnt).Error; err != nil || cnt != 1 {
		t.Fatalf("cross-tenant delete must not persist, cnt=%d err=%v", cnt, err)
	}
	// 无商品关联的存量任务（无 tenant 归属）保持可见
	legacy := &ImageTask{TaskType: "remove_background", Provider: "local", Status: "success"}
	if err := db.Create(legacy).Error; err != nil {
		t.Fatal(err)
	}
	if _, err := svc.ListTaskItems(ginCtx(2), legacy.ID); err != nil {
		t.Fatalf("legacy task without product: %v", err)
	}
}
