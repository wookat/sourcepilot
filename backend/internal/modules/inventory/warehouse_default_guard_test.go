package inventory_test

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"github.com/google/uuid"
	"gorm.io/gorm"

	"github.com/trademind-ai/trademind/backend/internal/modules/admin"
	"github.com/trademind-ai/trademind/backend/internal/modules/inventory"
	"github.com/trademind-ai/trademind/backend/internal/modules/product"
	"github.com/trademind-ai/trademind/backend/internal/pkg/ctxkey"
	"github.com/trademind-ai/trademind/backend/internal/pkg/model"
)

func openDefaultGuardTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	dsn := fmt.Sprintf("file:whdefault_%s?mode=memory&cache=shared&_pragma=busy_timeout(5000)", uuid.New().String())
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Skipf("sqlite unavailable: %v", err)
	}
	sqlDB, _ := db.DB()
	sqlDB.SetMaxOpenConns(1)
	if err := db.AutoMigrate(
		&admin.AdminUser{}, &admin.UserStorePermission{},
		&product.Product{}, &product.ProductSKU{},
		&inventory.Warehouse{}, &inventory.WarehouseStock{}, &inventory.InventoryChangeLog{},
	); err != nil {
		t.Fatal(err)
	}
	return db
}

func seedDefaultGuardUser(t *testing.T, db *gorm.DB, role string) uuid.UUID {
	t.Helper()
	uid := uuid.New()
	if err := db.Create(&admin.AdminUser{
		Base:         model.Base{ID: uid},
		Username:     "u-" + uid.String()[:12],
		Email:        "u-" + uid.String()[:12] + "@example.com",
		PasswordHash: "x",
		Role:         role,
		Status:       "active",
	}).Error; err != nil {
		t.Fatal(err)
	}
	return uid
}

func newDefaultGuardRouter(db *gorm.DB, actorID uuid.UUID) *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(func(c *gin.Context) {
		c.Set(ctxkey.AdminID, actorID.String())
		c.Set(ctxkey.TenantID, int64(1))
		c.Next()
	})
	inventory.Register(r.Group("/api/v1"), &inventory.Handler{Svc: &inventory.Service{DB: db}})
	return r
}

// Three-role contract for POST /inventory/warehouses/:id/set-default:
// readonly is rejected with 403 before any business logic; admin and
// operator (inventory.operate) succeed and the default flag actually moves.
func TestSetDefaultWarehouseRoles(t *testing.T) {
	db := openDefaultGuardTestDB(t)
	svc := &inventory.Service{DB: db}
	ctx := t.Context()
	def, err := svc.EnsureDefaultWarehouse(ctx, 1)
	if err != nil {
		t.Fatal(err)
	}
	south, err := svc.CreateWarehouse(ctx, 1, inventory.CreateWarehouseBody{Code: "south", Name: "华南仓"})
	if err != nil {
		t.Fatal(err)
	}

	readonlyRouter := newDefaultGuardRouter(db, seedDefaultGuardUser(t, db, "readonly"))
	w := httptest.NewRecorder()
	readonlyRouter.ServeHTTP(w, httptest.NewRequest(http.MethodPost, "/api/v1/inventory/warehouses/"+south.ID.String()+"/set-default", nil))
	if w.Code != http.StatusForbidden {
		t.Fatalf("readonly set-default: got %d body=%s, want 403", w.Code, w.Body.String())
	}
	if cur, err := svc.GetWarehouse(ctx, 1, south.ID); err != nil || cur.IsDefault {
		t.Fatalf("readonly must not switch default: %v %+v", err, cur)
	}

	operatorRouter := newDefaultGuardRouter(db, seedDefaultGuardUser(t, db, "operator"))
	w = httptest.NewRecorder()
	operatorRouter.ServeHTTP(w, httptest.NewRequest(http.MethodPost, "/api/v1/inventory/warehouses/"+south.ID.String()+"/set-default", nil))
	if w.Code != http.StatusOK {
		t.Fatalf("operator set-default: got %d body=%s, want 200", w.Code, w.Body.String())
	}
	if cur, err := svc.GetWarehouse(ctx, 1, south.ID); err != nil || !cur.IsDefault {
		t.Fatalf("operator switch missing: %v %+v", err, cur)
	}

	adminRouter := newDefaultGuardRouter(db, seedDefaultGuardUser(t, db, "admin"))
	w = httptest.NewRecorder()
	adminRouter.ServeHTTP(w, httptest.NewRequest(http.MethodPost, "/api/v1/inventory/warehouses/"+def.ID.String()+"/set-default", nil))
	if w.Code != http.StatusOK {
		t.Fatalf("admin set-default: got %d body=%s, want 200", w.Code, w.Body.String())
	}
	if cur, err := svc.GetWarehouse(ctx, 1, def.ID); err != nil || !cur.IsDefault {
		t.Fatalf("admin switch missing: %v %+v", err, cur)
	}
}
