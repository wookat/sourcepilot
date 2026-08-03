package carrier_test

import (
	"fmt"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"github.com/google/uuid"
	"github.com/trademind-ai/trademind/backend/internal/modules/carrier"
	"github.com/trademind-ai/trademind/backend/internal/pkg/ctxkey"
	"gorm.io/gorm"
)

func openCarrierTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	dsn := fmt.Sprintf("file:carrier_%s?mode=memory&cache=shared&_pragma=busy_timeout(5000)", uuid.New().String())
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Skipf("sqlite unavailable: %v", err)
	}
	sqlDB, _ := db.DB()
	sqlDB.SetMaxOpenConns(1)
	if err := db.AutoMigrate(&carrier.Carrier{}); err != nil {
		t.Fatal(err)
	}
	return db
}

func carrierTestCtx(tenantID int64) *gin.Context {
	gin.SetMode(gin.TestMode)
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Request = httptest.NewRequest("GET", "/api/v1/carriers", nil)
	c.Set(ctxkey.TenantID, tenantID)
	return c
}

func TestListSeedsPresetsPerTenant(t *testing.T) {
	db := openCarrierTestDB(t)
	svc := &carrier.Service{DB: db}

	rows, err := svc.List(carrierTestCtx(1), carrier.ListQuery{})
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != len(carrier.Presets()) {
		t.Fatalf("expected %d presets, got %d", len(carrier.Presets()), len(rows))
	}
	// Idempotent on second call.
	rows2, err := svc.List(carrierTestCtx(1), carrier.ListQuery{})
	if err != nil || len(rows2) != len(rows) {
		t.Fatalf("expected idempotent seeding, got %d rows err=%v", len(rows2), err)
	}
}

func TestCarrierTenantIsolation(t *testing.T) {
	db := openCarrierTestDB(t)
	svc := &carrier.Service{DB: db}

	rowT1, err := svc.Create(carrierTestCtx(1), carrier.CreateBody{Code: "yunta", Name: "云途物流"}, nil)
	if err != nil {
		t.Fatal(err)
	}
	// Tenant 2 must not see or mutate tenant 1's custom carrier.
	rows, err := svc.List(carrierTestCtx(2), carrier.ListQuery{})
	if err != nil {
		t.Fatal(err)
	}
	for _, r := range rows {
		if r.Code == "yunta" {
			t.Fatalf("tenant 2 sees tenant 1 carrier")
		}
	}
	enabled := false
	if _, err := svc.Update(carrierTestCtx(2), rowT1.ID, carrier.UpdateBody{Enabled: &enabled}, nil); err != carrier.ErrNotFound {
		t.Fatalf("expected ErrNotFound for cross-tenant update, got %v", err)
	}
	if err := svc.Delete(carrierTestCtx(2), rowT1.ID, nil); err != carrier.ErrNotFound {
		t.Fatalf("expected ErrNotFound for cross-tenant delete, got %v", err)
	}
}

func TestCarrierCRUDRules(t *testing.T) {
	db := openCarrierTestDB(t)
	svc := &carrier.Service{DB: db}
	c := carrierTestCtx(1)

	if _, err := svc.Create(c, carrier.CreateBody{Code: "BAD CODE", Name: "x"}, nil); err == nil {
		t.Fatal("expected invalid code rejection")
	}
	if _, err := svc.Create(c, carrier.CreateBody{Code: "custom1", Name: ""}, nil); err == nil {
		t.Fatal("expected empty name rejection")
	}
	row, err := svc.Create(c, carrier.CreateBody{Code: "custom1", Name: "自定义快递"}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := svc.Create(c, carrier.CreateBody{Code: "custom1", Name: "重复"}, nil); err == nil {
		t.Fatal("expected duplicate code rejection")
	}
	enabled := false
	upd, err := svc.Update(c, row.ID, carrier.UpdateBody{Enabled: &enabled}, nil)
	if err != nil || upd.Enabled {
		t.Fatalf("expected disable to work, got %+v err=%v", upd, err)
	}
	if err := svc.Delete(c, row.ID, nil); err != nil {
		t.Fatal(err)
	}
	// Presets cannot be deleted.
	rows, err := svc.List(c, carrier.ListQuery{Keyword: "顺丰"})
	if err != nil || len(rows) == 0 {
		t.Fatalf("expected sf preset, got %v err=%v", rows, err)
	}
	if err := svc.Delete(c, rows[0].ID, nil); err == nil {
		t.Fatal("expected preset delete rejection")
	}
}

func TestResolveEnabled(t *testing.T) {
	db := openCarrierTestDB(t)
	svc := &carrier.Service{DB: db}
	c := carrierTestCtx(1)

	if cr, err := svc.ResolveEnabled(c, "sf"); err != nil || cr.Code != "sf" {
		t.Fatalf("resolve by code failed: %v %v", cr, err)
	}
	if cr, err := svc.ResolveEnabled(c, "顺丰速运"); err != nil || cr.Code != "sf" {
		t.Fatalf("resolve by exact name failed: %v %v", cr, err)
	}
	if cr, err := svc.ResolveEnabled(c, "顺丰"); err != nil || cr.Code != "sf" {
		t.Fatalf("resolve by name prefix failed: %v %v", cr, err)
	}
	if _, err := svc.ResolveEnabled(c, "不存在的快递"); err != carrier.ErrNotFound {
		t.Fatalf("expected ErrNotFound, got %v", err)
	}
	// Disabled carriers do not resolve.
	rows, _ := svc.List(c, carrier.ListQuery{Keyword: "顺丰"})
	enabled := false
	if _, err := svc.Update(c, rows[0].ID, carrier.UpdateBody{Enabled: &enabled}, nil); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.ResolveEnabled(c, "sf"); err != carrier.ErrNotFound {
		t.Fatalf("expected disabled carrier not to resolve, got %v", err)
	}
}

func TestValidateTrackingNo(t *testing.T) {
	cases := []struct {
		code, tn string
		ok       bool
	}{
		{"sf", "SF1234567890123", true},
		{"sf", "sf1234567890123", true}, // case-insensitive
		{"sf", "YT123456789012", false},
		{"jd", "JD0123456789012", true},
		{"jd", "0123456789", false},
		{"ems", "EA123456789CN", true},
		{"ems", "EMS-123", false},
		{"zto", "78901234567890", true}, // generic rule
		{"other", "ABC-123456", true},
		{"other", "bad tracking!", false},
		{"custom1", "12345", false}, // too short for generic rule
		{"sf", "", true},            // empty allowed (pending)
	}
	for _, tc := range cases {
		err := carrier.ValidateTrackingNo(tc.code, tc.tn)
		if tc.ok && err != nil {
			t.Errorf("%s/%s: unexpected error %v", tc.code, tc.tn, err)
		}
		if !tc.ok && err == nil {
			t.Errorf("%s/%s: expected error", tc.code, tc.tn)
		}
	}
}
