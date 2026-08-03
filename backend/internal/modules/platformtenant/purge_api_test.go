package platformtenant_test

import (
	"encoding/json"
	"fmt"
	"net/http"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"github.com/google/uuid"
	"github.com/trademind-ai/trademind/backend/internal/modules/admin"
	"github.com/trademind-ai/trademind/backend/internal/modules/operationlog"
	"github.com/trademind-ai/trademind/backend/internal/modules/platformtenant"
	"github.com/trademind-ai/trademind/backend/internal/modules/product"
	"github.com/trademind-ai/trademind/backend/internal/modules/shop"
	"github.com/trademind-ai/trademind/backend/internal/pkg/ctxkey"
	"github.com/trademind-ai/trademind/backend/internal/pkg/model"
	"gorm.io/gorm"
)

func openPurgeTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	dsn := fmt.Sprintf("file:tenantpurge_%s?mode=memory&cache=shared&_pragma=busy_timeout(5000)", uuid.New().String())
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Skipf("sqlite unavailable: %v", err)
	}
	sqlDB, _ := db.DB()
	sqlDB.SetMaxOpenConns(1)
	if err := db.AutoMigrate(
		&admin.AdminUser{},
		&admin.UserStorePermission{},
		&platformtenant.Tenant{},
		&platformtenant.TenantPurgeTask{},
		&operationlog.OperationLog{},
		&product.Product{},
		&product.ProductSKU{},
		&shop.Shop{},
	); err != nil {
		t.Fatal(err)
	}
	return db
}

func newPurgeRouter(db *gorm.DB, actorID uuid.UUID, tenantID int64) *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(func(c *gin.Context) {
		c.Set(ctxkey.AdminID, actorID.String())
		c.Set(ctxkey.TenantID, tenantID)
		c.Next()
	})
	svc := &platformtenant.Service{DB: db, OpLog: &operationlog.Service{DB: db}, PurgeSync: true}
	platformtenant.Register(r.Group("/api/v1"), &platformtenant.Handler{Svc: svc})
	return r
}

func seedTenantBusinessData(t *testing.T, db *gorm.DB, tenantID int64) {
	t.Helper()
	u := &admin.AdminUser{
		Base:         model.Base{ID: uuid.New()},
		TenantID:     tenantID,
		Username:     fmt.Sprintf("purge-u-%d", tenantID),
		Email:        fmt.Sprintf("purge-u-%d@example.com", tenantID),
		PasswordHash: "x",
		Role:         "admin",
		Status:       "active",
	}
	if err := db.Create(u).Error; err != nil {
		t.Fatal(err)
	}
	sh := &shop.Shop{TenantID: tenantID, ShopName: "PURGE-shop", ShopCode: fmt.Sprintf("PURGE-%d", tenantID), Platform: "douyin_shop"}
	if err := db.Create(sh).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Create(&admin.UserStorePermission{ID: uuid.New(), UserID: u.ID, StoreID: sh.ID, Platform: "douyin_shop", PermissionScope: "operate"}).Error; err != nil {
		t.Fatal(err)
	}
	p := &product.Product{TenantID: tenantID, Title: "PURGE-product", Status: "draft"}
	if err := db.Create(p).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Create(&product.ProductSKU{ProductID: p.ID, SKUCode: fmt.Sprintf("PURGE-SKU-%d", tenantID)}).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Create(&operationlog.OperationLog{TenantID: tenantID, Action: "product.create", Resource: "product", Status: "success"}).Error; err != nil {
		t.Fatal(err)
	}
	// Soft-deleted row must be purged too.
	p2 := &product.Product{TenantID: tenantID, Title: "PURGE-archived", Status: "draft"}
	if err := db.Create(p2).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Delete(p2).Error; err != nil {
		t.Fatal(err)
	}
}

func countRows(t *testing.T, db *gorm.DB, query string, args ...any) int64 {
	t.Helper()
	var n int64
	if err := db.Raw(query, args...).Scan(&n).Error; err != nil {
		t.Fatal(err)
	}
	return n
}

// Full purge flow: disabled tenant with business data is purged in a
// background task, every table reaches zero residual rows, and the platform
// audit trail (tenant 0) is retained.
func TestPurgeDisabledTenantZeroResidual(t *testing.T) {
	db := openPurgeTestDB(t)
	actor := seedActor(t, db, 0, "admin")
	r := newPurgeRouter(db, actor, 0)
	id := createTenantForTest(t, r, "e2e-purge")
	seedTenantBusinessData(t, db, id)

	if w := doJSON(r, http.MethodPost, fmt.Sprintf("/api/v1/platform/tenants/%d/purge", id), `{"confirmName":"e2e-purge"}`); w.Code != http.StatusBadRequest {
		t.Fatalf("purge active tenant: got %d, want 400", w.Code)
	}
	if w := doJSON(r, http.MethodPost, fmt.Sprintf("/api/v1/platform/tenants/%d/disable", id), ""); w.Code != http.StatusOK {
		t.Fatalf("disable: got %d body=%s", w.Code, w.Body.String())
	}
	if w := doJSON(r, http.MethodPost, fmt.Sprintf("/api/v1/platform/tenants/%d/purge", id), `{"confirmName":"wrong-name"}`); w.Code != http.StatusBadRequest {
		t.Fatalf("purge wrong confirm: got %d, want 400", w.Code)
	}

	w := doJSON(r, http.MethodPost, fmt.Sprintf("/api/v1/platform/tenants/%d/purge", id), `{"confirmName":"e2e-purge"}`)
	if w.Code != http.StatusOK {
		t.Fatalf("purge: got %d body=%s", w.Code, w.Body.String())
	}

	w = doJSON(r, http.MethodGet, fmt.Sprintf("/api/v1/platform/tenants/%d/purge", id), "")
	if w.Code != http.StatusOK {
		t.Fatalf("purge status: got %d body=%s", w.Code, w.Body.String())
	}
	var payload struct {
		Data struct {
			Status string `json:"status"`
			Error  string `json:"error"`
			Report *struct {
				Tables map[string]int64 `json:"tables"`
				Total  int64            `json:"total"`
			} `json:"report"`
		} `json:"data"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &payload); err != nil {
		t.Fatal(err)
	}
	if payload.Data.Status != platformtenant.PurgeStatusSucceeded {
		t.Fatalf("purge status=%s error=%s, want succeeded", payload.Data.Status, payload.Data.Error)
	}
	if payload.Data.Report == nil || payload.Data.Report.Total != 0 {
		t.Fatalf("purge report should be zero residual: %+v", payload.Data.Report)
	}
	for table, n := range payload.Data.Report.Tables {
		if n != 0 {
			t.Errorf("residual rows in %s: %d", table, n)
		}
	}

	for _, q := range []string{
		"SELECT COUNT(*) FROM tenants WHERE id = ?",
		"SELECT COUNT(*) FROM admin_users WHERE tenant_id = ?",
		"SELECT COUNT(*) FROM shops WHERE tenant_id = ?",
		"SELECT COUNT(*) FROM products WHERE tenant_id = ?",
		"SELECT COUNT(*) FROM operation_logs WHERE tenant_id = ?",
	} {
		if n := countRows(t, db, q, id); n != 0 {
			t.Errorf("%s: %d rows remain", q, n)
		}
	}
	if n := countRows(t, db, "SELECT COUNT(*) FROM product_skus WHERE sku_code LIKE 'PURGE-SKU-%'"); n != 0 {
		t.Errorf("product_skus: %d rows remain", n)
	}

	// Platform audit (tenant 0) must retain provisioning + purge trail.
	for _, action := range []string{"tenant.create", "tenant.purge.start", "tenant.purge.done"} {
		if n := countRows(t, db,
			"SELECT COUNT(*) FROM operation_logs WHERE tenant_id = 0 AND action = ? AND resource_id = ?",
			action, fmt.Sprintf("%d", id)); n != 1 {
			t.Errorf("platform audit %s: got %d rows, want 1", action, n)
		}
	}
	// Purge task record itself survives.
	if n := countRows(t, db, "SELECT COUNT(*) FROM tenant_purge_tasks WHERE tenant_id = ?", id); n != 1 {
		t.Errorf("tenant_purge_tasks rows = %d, want 1", n)
	}
}

// Tenant 0 can never be purged; unknown tenants are 404.
func TestPurgeGuards(t *testing.T) {
	db := openPurgeTestDB(t)
	actor := seedActor(t, db, 0, "admin")
	r := newPurgeRouter(db, actor, 0)

	if w := doJSON(r, http.MethodPost, "/api/v1/platform/tenants/0/purge", `{"confirmName":"平台租户（默认）"}`); w.Code != http.StatusBadRequest {
		t.Fatalf("purge tenant 0: got %d, want 400", w.Code)
	}
	if w := doJSON(r, http.MethodPost, "/api/v1/platform/tenants/999999/purge", `{"confirmName":"x"}`); w.Code != http.StatusNotFound {
		t.Fatalf("purge missing tenant: got %d, want 404", w.Code)
	}
	if w := doJSON(r, http.MethodGet, "/api/v1/platform/tenants/0/purge", ""); w.Code != http.StatusBadRequest {
		t.Fatalf("purge status tenant 0: got %d, want 400", w.Code)
	}
	if n := countRows(t, db, "SELECT COUNT(*) FROM admin_users WHERE tenant_id = 0"); n != 1 {
		t.Fatalf("tenant 0 data must be untouched, admin_users=%d", n)
	}
}

// Non-platform personas get 403 on purge routes with no side effects.
func TestPurgeForbiddenPersonas(t *testing.T) {
	db := openPurgeTestDB(t)
	platformActor := seedActor(t, db, 0, "admin")
	pr := newPurgeRouter(db, platformActor, 0)
	id := createTenantForTest(t, pr, "e2e-purge-perm")
	if w := doJSON(pr, http.MethodPost, fmt.Sprintf("/api/v1/platform/tenants/%d/disable", id), ""); w.Code != http.StatusOK {
		t.Fatalf("disable: got %d", w.Code)
	}

	cases := []struct {
		name   string
		tenant int64
		role   string
	}{
		{"tenant1-admin", 1, "admin"},
		{"tenant0-operator", 0, "operator"},
		{"tenant0-readonly", 0, "readonly"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			actor := seedActor(t, db, tc.tenant, tc.role)
			r := newPurgeRouter(db, actor, tc.tenant)
			if w := doJSON(r, http.MethodPost, fmt.Sprintf("/api/v1/platform/tenants/%d/purge", id), `{"confirmName":"e2e-purge-perm"}`); w.Code != http.StatusForbidden {
				t.Fatalf("purge: got %d, want 403", w.Code)
			}
			if w := doJSON(r, http.MethodGet, fmt.Sprintf("/api/v1/platform/tenants/%d/purge", id), ""); w.Code != http.StatusForbidden {
				t.Fatalf("purge status: got %d, want 403", w.Code)
			}
		})
	}
	if n := countRows(t, db, "SELECT COUNT(*) FROM tenants WHERE id = ?", id); n != 1 {
		t.Fatalf("forbidden purge must have no side effect, tenants=%d", n)
	}
	if n := countRows(t, db, "SELECT COUNT(*) FROM tenant_purge_tasks WHERE tenant_id = ?", id); n != 0 {
		t.Fatalf("forbidden purge must not create tasks, got %d", n)
	}
}
