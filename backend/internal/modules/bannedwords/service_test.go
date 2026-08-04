package bannedwords_test

import (
	"context"
	"fmt"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"github.com/google/uuid"
	"github.com/trademind-ai/trademind/backend/internal/modules/bannedwords"
	"github.com/trademind-ai/trademind/backend/internal/modules/product"
	"github.com/trademind-ai/trademind/backend/internal/pkg/ctxkey"
	"gorm.io/gorm"
)

func openTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	dsn := fmt.Sprintf("file:bannedwords_%s?mode=memory&cache=shared&_pragma=busy_timeout(5000)", uuid.New().String())
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Skipf("sqlite unavailable: %v", err)
	}
	sqlDB, _ := db.DB()
	sqlDB.SetMaxOpenConns(1)
	if err := db.AutoMigrate(&bannedwords.BannedWord{}, &bannedwords.BannedWordCategoryState{}, &product.Product{}); err != nil {
		t.Fatal(err)
	}
	return db
}

func testCtx(tenantID int64) *gin.Context {
	gin.SetMode(gin.TestMode)
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Request = httptest.NewRequest("GET", "/api/v1/banned-words", nil)
	c.Set(ctxkey.TenantID, tenantID)
	return c
}

func TestListSeedsPresetsPerTenant(t *testing.T) {
	db := openTestDB(t)
	svc := &bannedwords.Service{DB: db}

	rows, err := svc.List(testCtx(1), bannedwords.ListQuery{})
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != len(bannedwords.Presets()) {
		t.Fatalf("expected %d presets, got %d", len(bannedwords.Presets()), len(rows))
	}
	rows2, err := svc.List(testCtx(1), bannedwords.ListQuery{})
	if err != nil || len(rows2) != len(rows) {
		t.Fatalf("expected idempotent seeding, got %d rows err=%v", len(rows2), err)
	}
}

func TestTenantIsolationAndNotFound(t *testing.T) {
	db := openTestDB(t)
	svc := &bannedwords.Service{DB: db}

	row, err := svc.Create(testCtx(1), bannedwords.CreateBody{Word: "跨租户词", Level: "warning"}, nil)
	if err != nil {
		t.Fatal(err)
	}
	// Another tenant cannot see / mutate it (越权 → ErrNotFound → 404).
	enabled := false
	if _, err := svc.Update(testCtx(2), row.ID, bannedwords.UpdateBody{Enabled: &enabled}, nil); err != bannedwords.ErrNotFound {
		t.Fatalf("expected ErrNotFound for cross tenant update, got %v", err)
	}
	if err := svc.Delete(testCtx(2), row.ID, nil); err != bannedwords.ErrNotFound {
		t.Fatalf("expected ErrNotFound for cross tenant delete, got %v", err)
	}
}

func TestPresetReadonlySemantics(t *testing.T) {
	db := openTestDB(t)
	svc := &bannedwords.Service{DB: db}

	rows, err := svc.List(testCtx(1), bannedwords.ListQuery{Keyword: "最佳"})
	if err != nil || len(rows) == 0 {
		t.Fatalf("expected preset 最佳, err=%v", err)
	}
	preset := rows[0]
	if !preset.IsPreset {
		t.Fatal("expected preset row")
	}
	// Presets cannot be deleted.
	if err := svc.Delete(testCtx(1), preset.ID, nil); err == nil {
		t.Fatal("expected delete preset to fail")
	}
	// Presets cannot change level/category/suggestion.
	lv := "warning"
	if _, err := svc.Update(testCtx(1), preset.ID, bannedwords.UpdateBody{Level: &lv}, nil); err == nil {
		t.Fatal("expected preset level edit to fail")
	}
	// Presets can be disabled.
	off := false
	upd, err := svc.Update(testCtx(1), preset.ID, bannedwords.UpdateBody{Enabled: &off}, nil)
	if err != nil || upd.Enabled {
		t.Fatalf("expected preset disable to succeed, err=%v", err)
	}
}

func TestCreateValidatesAndRejectsDuplicates(t *testing.T) {
	db := openTestDB(t)
	svc := &bannedwords.Service{DB: db}

	if _, err := svc.Create(testCtx(1), bannedwords.CreateBody{Word: "  "}, nil); err == nil {
		t.Fatal("expected empty word to fail")
	}
	if _, err := svc.Create(testCtx(1), bannedwords.CreateBody{Word: "自定义词", Level: "bogus"}, nil); err == nil {
		t.Fatal("expected invalid level to fail")
	}
	if _, err := svc.Create(testCtx(1), bannedwords.CreateBody{Word: "自定义词", Level: "forbidden"}, nil); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.Create(testCtx(1), bannedwords.CreateBody{Word: "自定义词"}, nil); err == nil {
		t.Fatal("expected duplicate word to fail")
	}
	// Same word in another tenant is fine.
	if _, err := svc.Create(testCtx(2), bannedwords.CreateBody{Word: "自定义词"}, nil); err != nil {
		t.Fatal(err)
	}
}

func TestCategoryToggleAffectsActiveWords(t *testing.T) {
	db := openTestDB(t)
	svc := &bannedwords.Service{DB: db}

	cats, err := svc.ListCategories(testCtx(1))
	if err != nil || len(cats) < 4 {
		t.Fatalf("expected builtin categories, err=%v got=%d", err, len(cats))
	}
	before, err := svc.ActiveWords(context.Background(), 1)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := svc.ToggleCategory(testCtx(1), bannedwords.CategoryMedical, false, nil); err != nil {
		t.Fatal(err)
	}
	after, err := svc.ActiveWords(context.Background(), 1)
	if err != nil {
		t.Fatal(err)
	}
	if len(after) >= len(before) {
		t.Fatalf("expected fewer active words after disabling medical: %d -> %d", len(before), len(after))
	}
	for _, w := range after {
		if w.Category == bannedwords.CategoryMedical {
			t.Fatalf("disabled category word still active: %s", w.Word)
		}
	}
	// Unknown empty category with no words → not found.
	if _, err := svc.ToggleCategory(testCtx(1), "nonexistent", false, nil); err != bannedwords.ErrNotFound {
		t.Fatalf("expected ErrNotFound, got %v", err)
	}
}

func TestScanProductStatuses(t *testing.T) {
	db := openTestDB(t)
	svc := &bannedwords.Service{DB: db}

	blocked := product.Product{TenantID: 1, Source: "manual", Status: "draft", Title: "全网最低价好物", Description: "治疗失眠"}
	if err := db.Create(&blocked).Error; err != nil {
		t.Fatal(err)
	}
	res, err := svc.ScanProduct(context.Background(), blocked)
	if err != nil {
		t.Fatal(err)
	}
	if res.Status != "blocked" || res.ForbiddenCount == 0 {
		t.Fatalf("expected blocked scan, got %+v", res)
	}

	warn := product.Product{TenantID: 1, Source: "manual", Status: "draft", Title: "祖传工艺茶杯"}
	if err := db.Create(&warn).Error; err != nil {
		t.Fatal(err)
	}
	res, err = svc.ScanProduct(context.Background(), warn)
	if err != nil {
		t.Fatal(err)
	}
	if res.Status != "warning" || res.WarningCount == 0 || res.ForbiddenCount != 0 {
		t.Fatalf("expected warning scan, got %+v", res)
	}

	clean := product.Product{TenantID: 1, Source: "manual", Status: "draft", Title: "简约陶瓷杯", Description: "日常家用"}
	if err := db.Create(&clean).Error; err != nil {
		t.Fatal(err)
	}
	res, err = svc.ScanProduct(context.Background(), clean)
	if err != nil {
		t.Fatal(err)
	}
	if res.Status != "passed" || len(res.Hits) != 0 {
		t.Fatalf("expected passed scan, got %+v", res)
	}
}

func TestFindProductScopedCrossTenant404(t *testing.T) {
	db := openTestDB(t)
	svc := &bannedwords.Service{DB: db}

	p := product.Product{TenantID: 1, Source: "manual", Status: "draft", Title: "普通商品"}
	if err := db.Create(&p).Error; err != nil {
		t.Fatal(err)
	}
	if _, err := svc.FindProductScoped(testCtx(1), p.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.FindProductScoped(testCtx(2), p.ID); err != bannedwords.ErrNotFound {
		t.Fatalf("expected ErrNotFound for cross tenant product, got %v", err)
	}
}
