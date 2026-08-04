package customerchat

import (
	"net/http/httptest"
	"path/filepath"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"gorm.io/gorm"

	"github.com/trademind-ai/trademind/backend/internal/pkg/ctxkey"
)

func newTemplateTestCtx(t *testing.T, tenantID int64) (*gin.Context, *Service) {
	t.Helper()
	dsn := filepath.Join(t.TempDir(), "tpl.db")
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Skipf("sqlite unavailable: %v", err)
	}
	if err := db.AutoMigrate(&CustomerReplyTemplate{}); err != nil {
		t.Fatal(err)
	}
	gin.SetMode(gin.TestMode)
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Request = httptest.NewRequest("POST", "/api/v1/customer/reply-templates", nil)
	c.Set(ctxkey.TenantID, tenantID)
	return c, &Service{DB: db}
}

func ctxWithTenant(t *testing.T, svc *Service, tenantID int64) *gin.Context {
	t.Helper()
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Request = httptest.NewRequest("GET", "/api/v1/customer/reply-templates", nil)
	c.Set(ctxkey.TenantID, tenantID)
	return c
}

func TestTemplateCRUDStampsTenantAndValidates(t *testing.T) {
	c, svc := newTemplateTestCtx(t, 7)

	// invalid group rejected
	if _, err := svc.CreateTemplate(c, TemplateUpsertBody{GroupKey: "bogus", Name: "x", Content: "y"}, nil); err == nil {
		t.Fatal("invalid group must be rejected")
	}
	// empty content rejected
	if _, err := svc.CreateTemplate(c, TemplateUpsertBody{GroupKey: TemplateGroupPresale, Name: "x", Content: "  "}, nil); err == nil {
		t.Fatal("empty content must be rejected")
	}

	row, err := svc.CreateTemplate(c, TemplateUpsertBody{
		GroupKey: TemplateGroupPresale, Name: "e2e-售前-欢迎", Content: "您好{买家昵称}，欢迎光临！",
	}, nil)
	if err != nil {
		t.Fatalf("CreateTemplate: %v", err)
	}
	if row.SortOrder != 1 || !row.Enabled {
		t.Fatalf("defaults: sortOrder=%d enabled=%v", row.SortOrder, row.Enabled)
	}
	var stored CustomerReplyTemplate
	if err := svc.DB.First(&stored, "id = ?", row.ID).Error; err != nil {
		t.Fatal(err)
	}
	if stored.TenantID != 7 {
		t.Fatalf("tenant id: got %d, want 7", stored.TenantID)
	}

	// second create appends after the first
	row2, err := svc.CreateTemplate(c, TemplateUpsertBody{
		GroupKey: TemplateGroupPresale, Name: "e2e-售前-库存", Content: "有货",
	}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if row2.SortOrder != 2 {
		t.Fatalf("append sortOrder: got %d, want 2", row2.SortOrder)
	}

	// create with enabled=false must persist false（回归：GORM default tag 会吞 bool 零值）
	offCreate := false
	rowOff, err := svc.CreateTemplate(c, TemplateUpsertBody{
		GroupKey: TemplateGroupOther, Name: "e2e-停用样例", Content: "停用内容", Enabled: &offCreate,
	}, nil)
	if err != nil {
		t.Fatalf("CreateTemplate(enabled=false): %v", err)
	}
	var storedOff CustomerReplyTemplate
	if err := svc.DB.First(&storedOff, "id = ?", rowOff.ID).Error; err != nil {
		t.Fatal(err)
	}
	if rowOff.Enabled || storedOff.Enabled {
		t.Fatalf("enabled=false must persist: row=%v stored=%v", rowOff.Enabled, storedOff.Enabled)
	}
	if err := svc.DeleteTemplate(c, rowOff.ID, nil); err != nil {
		t.Fatal(err)
	}

	// update toggles enabled and renames
	off := false
	upd, err := svc.UpdateTemplate(c, row.ID, TemplateUpsertBody{Name: "e2e-售前-欢迎V2", Enabled: &off}, nil)
	if err != nil {
		t.Fatalf("UpdateTemplate: %v", err)
	}
	if upd.Name != "e2e-售前-欢迎V2" || upd.Enabled {
		t.Fatalf("update result: %+v", upd)
	}

	// delete removes from list
	if err := svc.DeleteTemplate(c, row2.ID, nil); err != nil {
		t.Fatalf("DeleteTemplate: %v", err)
	}
	rows, err := svc.ListTemplates(c, TemplateListQuery{})
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 1 || rows[0].ID != row.ID {
		t.Fatalf("list after delete: %+v", rows)
	}
}

func TestTemplateTenantIsolationAndFilters(t *testing.T) {
	c7, svc := newTemplateTestCtx(t, 7)
	if _, err := svc.CreateTemplate(c7, TemplateUpsertBody{
		GroupKey: TemplateGroupRefund, Name: "e2e-退款-说明", Content: "退款 1-3 个工作日到账",
	}, nil); err != nil {
		t.Fatal(err)
	}

	c8 := ctxWithTenant(t, svc, 8)
	rows, err := svc.ListTemplates(c8, TemplateListQuery{})
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 0 {
		t.Fatalf("tenant 8 must not see tenant 7 templates, got %d", len(rows))
	}

	// keyword + group filters
	if _, err := svc.CreateTemplate(c7, TemplateUpsertBody{
		GroupKey: TemplateGroupLogistics, Name: "e2e-物流-查询", Content: "物流单号 {物流单号}",
	}, nil); err != nil {
		t.Fatal(err)
	}
	got, err := svc.ListTemplates(c7, TemplateListQuery{GroupKey: TemplateGroupLogistics})
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || got[0].GroupKey != TemplateGroupLogistics {
		t.Fatalf("group filter: %+v", got)
	}
	got, err = svc.ListTemplates(c7, TemplateListQuery{Keyword: "退款"})
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || got[0].GroupKey != TemplateGroupRefund {
		t.Fatalf("keyword filter: %+v", got)
	}
}

func TestTemplateReorder(t *testing.T) {
	c, svc := newTemplateTestCtx(t, 7)
	a, err := svc.CreateTemplate(c, TemplateUpsertBody{GroupKey: TemplateGroupPresale, Name: "e2e-A", Content: "a"}, nil)
	if err != nil {
		t.Fatal(err)
	}
	b, err := svc.CreateTemplate(c, TemplateUpsertBody{GroupKey: TemplateGroupPresale, Name: "e2e-B", Content: "b"}, nil)
	if err != nil {
		t.Fatal(err)
	}

	if err := svc.ReorderTemplates(c, ReorderTemplatesBody{
		GroupKey: TemplateGroupPresale, IDs: []string{b.ID.String(), a.ID.String()},
	}); err != nil {
		t.Fatalf("ReorderTemplates: %v", err)
	}
	rows, err := svc.ListTemplates(c, TemplateListQuery{GroupKey: TemplateGroupPresale})
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 2 || rows[0].ID != b.ID || rows[1].ID != a.ID {
		t.Fatalf("reorder result: %+v", rows)
	}

	// unknown id inside another tenant/group must fail atomically
	if err := svc.ReorderTemplates(c, ReorderTemplatesBody{
		GroupKey: TemplateGroupRefund, IDs: []string{a.ID.String()},
	}); err == nil {
		t.Fatal("reorder with wrong group must fail")
	}
}
