package waybill_test

import (
	"fmt"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"github.com/google/uuid"
	"github.com/trademind-ai/trademind/backend/internal/modules/waybill"
	"github.com/trademind-ai/trademind/backend/internal/pkg/ctxkey"
	"gorm.io/gorm"
)

func openWaybillTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	dsn := fmt.Sprintf("file:waybill_%s?mode=memory&cache=shared&_pragma=busy_timeout(5000)", uuid.New().String())
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Skipf("sqlite unavailable: %v", err)
	}
	sqlDB, _ := db.DB()
	sqlDB.SetMaxOpenConns(1)
	if err := db.AutoMigrate(&waybill.Template{}, &waybill.ShippingRule{}); err != nil {
		t.Fatal(err)
	}
	return db
}

func waybillTestCtx(tenantID int64) *gin.Context {
	gin.SetMode(gin.TestMode)
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Request = httptest.NewRequest("GET", "/api/v1/waybill-templates", nil)
	c.Set(ctxkey.TenantID, tenantID)
	return c
}

func TestListTemplatesSeedsPresetsPerTenant(t *testing.T) {
	db := openWaybillTestDB(t)
	svc := &waybill.Service{DB: db}

	rows, err := svc.ListTemplates(waybillTestCtx(1))
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != len(waybill.TemplatePresets()) {
		t.Fatalf("expected %d presets, got %d", len(waybill.TemplatePresets()), len(rows))
	}
	defaults := 0
	sizes := map[string]bool{}
	for _, r := range rows {
		if r.IsDefault {
			defaults++
		}
		sizes[r.SizeCode] = true
	}
	if defaults != 1 {
		t.Fatalf("expected exactly one default template, got %d", defaults)
	}
	for _, s := range waybill.ValidSizes() {
		if !sizes[s] {
			t.Fatalf("missing preset size %s", s)
		}
	}
	// Idempotent on second call.
	rows2, err := svc.ListTemplates(waybillTestCtx(1))
	if err != nil || len(rows2) != len(rows) {
		t.Fatalf("expected idempotent seeding, got %d rows err=%v", len(rows2), err)
	}
}

func TestTemplateTenantIsolation(t *testing.T) {
	db := openWaybillTestDB(t)
	svc := &waybill.Service{DB: db}

	rowT1, err := svc.CreateTemplate(waybillTestCtx(1), waybill.TemplateBody{Name: "自定义模板", SizeCode: waybill.Size100x150}, nil)
	if err != nil {
		t.Fatal(err)
	}
	rows, err := svc.ListTemplates(waybillTestCtx(2))
	if err != nil {
		t.Fatal(err)
	}
	for _, r := range rows {
		if r.ID == rowT1.ID {
			t.Fatalf("tenant 2 sees tenant 1 template")
		}
	}
	if _, err := svc.UpdateTemplate(waybillTestCtx(2), rowT1.ID, waybill.TemplateBody{Name: "hijack"}, nil); err != waybill.ErrTemplateNotFound {
		t.Fatalf("expected ErrTemplateNotFound for cross-tenant update, got %v", err)
	}
	if err := svc.DeleteTemplate(waybillTestCtx(2), rowT1.ID, nil); err != waybill.ErrTemplateNotFound {
		t.Fatalf("expected ErrTemplateNotFound for cross-tenant delete, got %v", err)
	}
	if _, err := svc.GetTemplate(waybillTestCtx(2), rowT1.ID); err != waybill.ErrTemplateNotFound {
		t.Fatalf("expected ErrTemplateNotFound for cross-tenant get, got %v", err)
	}
}

func TestTemplateDefaultIsExclusive(t *testing.T) {
	db := openWaybillTestDB(t)
	svc := &waybill.Service{DB: db}

	if _, err := svc.ListTemplates(waybillTestCtx(1)); err != nil {
		t.Fatal(err)
	}
	yes := true
	row, err := svc.CreateTemplate(waybillTestCtx(1), waybill.TemplateBody{Name: "新默认", SizeCode: waybill.Size100x180, IsDefault: &yes}, nil)
	if err != nil {
		t.Fatal(err)
	}
	var defaults int64
	if err := db.Model(&waybill.Template{}).Where("tenant_id = 1 AND is_default = ?", true).Count(&defaults).Error; err != nil {
		t.Fatal(err)
	}
	if defaults != 1 {
		t.Fatalf("expected single default after switch, got %d", defaults)
	}
	got, err := svc.DefaultTemplate(waybillTestCtx(1))
	if err != nil || got.ID != row.ID {
		t.Fatalf("expected new default %s, got %+v err=%v", row.ID, got, err)
	}
}

func TestTemplatePresetCannotBeDeleted(t *testing.T) {
	db := openWaybillTestDB(t)
	svc := &waybill.Service{DB: db}

	rows, err := svc.ListTemplates(waybillTestCtx(1))
	if err != nil || len(rows) == 0 {
		t.Fatalf("seed failed: %v", err)
	}
	if err := svc.DeleteTemplate(waybillTestCtx(1), rows[0].ID, nil); err == nil {
		t.Fatal("expected preset delete to be rejected")
	}
}

func TestTemplateRejectsInvalidSize(t *testing.T) {
	db := openWaybillTestDB(t)
	svc := &waybill.Service{DB: db}
	if _, err := svc.CreateTemplate(waybillTestCtx(1), waybill.TemplateBody{Name: "坏尺寸", SizeCode: "a3"}, nil); err == nil {
		t.Fatal("expected invalid size to be rejected")
	}
}

func TestRuleTenantIsolation(t *testing.T) {
	db := openWaybillTestDB(t)
	svc := &waybill.Service{DB: db}

	rowT1, err := svc.CreateRule(waybillTestCtx(1), waybill.RuleBody{Name: "规则A", CarrierCode: "sf"}, nil)
	if err != nil {
		t.Fatal(err)
	}
	rows, err := svc.ListRules(waybillTestCtx(2))
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 0 {
		t.Fatalf("tenant 2 sees tenant 1 rules")
	}
	if _, err := svc.UpdateRule(waybillTestCtx(2), rowT1.ID, waybill.RuleBody{Name: "hijack"}, nil); err != waybill.ErrRuleNotFound {
		t.Fatalf("expected ErrRuleNotFound for cross-tenant update, got %v", err)
	}
	if err := svc.DeleteRule(waybillTestCtx(2), rowT1.ID, nil); err != waybill.ErrRuleNotFound {
		t.Fatalf("expected ErrRuleNotFound for cross-tenant delete, got %v", err)
	}
}

func TestRuleRangeValidation(t *testing.T) {
	db := openWaybillTestDB(t)
	svc := &waybill.Service{DB: db}
	minW, maxW := 5.0, 1.0
	if _, err := svc.CreateRule(waybillTestCtx(1), waybill.RuleBody{Name: "坏区间", CarrierCode: "sf", MinWeightKg: &minW, MaxWeightKg: &maxW}, nil); err == nil {
		t.Fatal("expected inverted weight range to be rejected")
	}
}

func f64(v float64) *float64 { return &v }

func TestRecommendPriorityAndConditions(t *testing.T) {
	db := openWaybillTestDB(t)
	svc := &waybill.Service{DB: db}
	c := waybillTestCtx(1)

	provinces := []string{"广东省"}
	platforms := []string{"douyin_shop"}
	p1, p2, p3 := 10, 20, 30
	if _, err := svc.CreateRule(c, waybill.RuleBody{Name: "广东走顺丰", CarrierCode: "sf", Priority: &p1, Provinces: &provinces}, nil); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.CreateRule(c, waybill.RuleBody{Name: "高金额走京东", CarrierCode: "jd", Priority: &p2, MinAmount: f64(500)}, nil); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.CreateRule(c, waybill.RuleBody{Name: "抖店默认韵达", CarrierCode: "yunda", Priority: &p3, Platforms: &platforms}, nil); err != nil {
		t.Fatal(err)
	}

	// Province match wins by priority even when the amount rule also matches.
	rec, err := svc.Recommend(c, waybill.MatchAttrs{Province: "广东", Amount: f64(800), Platform: "douyin_shop"})
	if err != nil || !rec.Matched || rec.CarrierCode != "sf" {
		t.Fatalf("expected sf by priority, got %+v err=%v", rec, err)
	}
	// Unknown province cannot satisfy a province condition; amount rule wins.
	rec, err = svc.Recommend(c, waybill.MatchAttrs{Amount: f64(800)})
	if err != nil || !rec.Matched || rec.CarrierCode != "jd" {
		t.Fatalf("expected jd for amount-only, got %+v err=%v", rec, err)
	}
	// Platform rule as fallback.
	rec, err = svc.Recommend(c, waybill.MatchAttrs{Platform: "douyin_shop", Amount: f64(100)})
	if err != nil || !rec.Matched || rec.CarrierCode != "yunda" {
		t.Fatalf("expected yunda for platform, got %+v err=%v", rec, err)
	}
	// No rule matches: explicit unmatched, no error.
	rec, err = svc.Recommend(c, waybill.MatchAttrs{Platform: "manual", Amount: f64(100)})
	if err != nil || rec.Matched {
		t.Fatalf("expected unmatched, got %+v err=%v", rec, err)
	}
	// Rules never leak across tenants.
	rec, err = svc.Recommend(waybillTestCtx(2), waybill.MatchAttrs{Province: "广东", Amount: f64(800)})
	if err != nil || rec.Matched {
		t.Fatalf("expected unmatched for tenant 2, got %+v err=%v", rec, err)
	}
}

func TestRecommendSkipsDisabledRule(t *testing.T) {
	db := openWaybillTestDB(t)
	svc := &waybill.Service{DB: db}
	c := waybillTestCtx(1)

	off := false
	p1, p2 := 10, 20
	if _, err := svc.CreateRule(c, waybill.RuleBody{Name: "停用规则", CarrierCode: "sf", Priority: &p1, Enabled: &off}, nil); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.CreateRule(c, waybill.RuleBody{Name: "启用规则", CarrierCode: "zto", Priority: &p2}, nil); err != nil {
		t.Fatal(err)
	}
	rec, err := svc.Recommend(c, waybill.MatchAttrs{Platform: "manual"})
	if err != nil || !rec.Matched || rec.CarrierCode != "zto" {
		t.Fatalf("expected zto (disabled rule skipped), got %+v err=%v", rec, err)
	}
}
