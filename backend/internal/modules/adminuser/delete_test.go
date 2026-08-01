package adminuser_test

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"github.com/google/uuid"
	"github.com/trademind-ai/trademind/backend/internal/modules/admin"
	"github.com/trademind-ai/trademind/backend/internal/modules/adminuser"
	"github.com/trademind-ai/trademind/backend/internal/modules/shop"
	"github.com/trademind-ai/trademind/backend/internal/pkg/ctxkey"
	"github.com/trademind-ai/trademind/backend/internal/pkg/model"
	"gorm.io/gorm"
)

func openUserTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	dsn := fmt.Sprintf("file:adminuser_%s?mode=memory&cache=shared&_pragma=busy_timeout(5000)", uuid.New().String())
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Skipf("sqlite unavailable: %v", err)
	}
	sqlDB, _ := db.DB()
	sqlDB.SetMaxOpenConns(1)
	if err := db.AutoMigrate(&admin.AdminUser{}, &admin.UserStorePermission{}, &shop.Shop{}); err != nil {
		t.Fatal(err)
	}
	return db
}

func seedUser(t *testing.T, db *gorm.DB, role string) uuid.UUID {
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
		t.Fatalf("seed %s user: %v", role, err)
	}
	return uid
}

func newUserRouter(db *gorm.DB, actorID uuid.UUID) *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(func(c *gin.Context) {
		c.Set(ctxkey.AdminID, actorID.String())
		c.Set(ctxkey.TenantID, int64(0))
		c.Next()
	})
	adminuser.Register(r.Group("/api/v1"), &adminuser.Handler{Svc: &adminuser.Service{DB: db}})
	return r
}

func doDelete(r *gin.Engine, targetID string) *httptest.ResponseRecorder {
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodDelete, "/api/v1/admin/users/"+targetID, nil)
	r.ServeHTTP(w, req)
	return w
}

// admin can soft-delete another user; row keeps deleted_at and grants are revoked.
func TestDeleteUserSoftDeletesAndRevokesGrants(t *testing.T) {
	db := openUserTestDB(t)
	actor := seedUser(t, db, "admin")
	target := seedUser(t, db, "operator")
	storeID := uuid.New()
	if err := db.Create(&admin.UserStorePermission{
		ID:              uuid.New(),
		UserID:          target,
		StoreID:         storeID,
		Platform:        "douyin",
		PermissionScope: "operate",
	}).Error; err != nil {
		t.Fatalf("seed grant: %v", err)
	}

	r := newUserRouter(db, actor)
	if w := doDelete(r, target.String()); w.Code != http.StatusOK {
		t.Fatalf("delete: got %d body=%s, want 200", w.Code, w.Body.String())
	}

	var visible int64
	if err := db.Model(&admin.AdminUser{}).Where("id = ?", target).Count(&visible).Error; err != nil {
		t.Fatal(err)
	}
	if visible != 0 {
		t.Fatalf("deleted user still visible in default scope")
	}
	var kept admin.AdminUser
	if err := db.Unscoped().First(&kept, "id = ?", target).Error; err != nil {
		t.Fatalf("soft-deleted row must remain: %v", err)
	}
	if !kept.DeletedAt.Valid {
		t.Fatalf("deleted_at must be set (soft delete, not hard delete)")
	}
	var grants int64
	if err := db.Model(&admin.UserStorePermission{}).Where("user_id = ?", target).Count(&grants).Error; err != nil {
		t.Fatal(err)
	}
	if grants != 0 {
		t.Fatalf("store grants must be revoked, got %d", grants)
	}

	// deleting again returns 404 (already soft-deleted).
	if w := doDelete(r, target.String()); w.Code != http.StatusNotFound {
		t.Fatalf("repeat delete: got %d, want 404", w.Code)
	}
}

// admin cannot delete their own account.
func TestDeleteUserRejectsSelf(t *testing.T) {
	db := openUserTestDB(t)
	actor := seedUser(t, db, "admin")

	r := newUserRouter(db, actor)
	w := doDelete(r, actor.String())
	if w.Code != http.StatusBadRequest {
		t.Fatalf("self delete: got %d body=%s, want 400", w.Code, w.Body.String())
	}
	var visible int64
	if err := db.Model(&admin.AdminUser{}).Where("id = ?", actor).Count(&visible).Error; err != nil {
		t.Fatal(err)
	}
	if visible != 1 {
		t.Fatalf("actor must not be deleted")
	}
}

// readonly and operator roles are both rejected with 403.
func TestDeleteUserRequiresAdmin(t *testing.T) {
	db := openUserTestDB(t)
	target := seedUser(t, db, "operator")
	for _, role := range []string{"readonly", "operator"} {
		actor := seedUser(t, db, role)
		r := newUserRouter(db, actor)
		if w := doDelete(r, target.String()); w.Code != http.StatusForbidden {
			t.Fatalf("%s delete: got %d body=%s, want 403", role, w.Code, w.Body.String())
		}
	}
	var visible int64
	if err := db.Model(&admin.AdminUser{}).Where("id = ?", target).Count(&visible).Error; err != nil {
		t.Fatal(err)
	}
	if visible != 1 {
		t.Fatalf("target must not be deleted by non-admin roles")
	}
}

// invalid id returns 400, unknown id returns 404.
func TestDeleteUserInvalidAndMissing(t *testing.T) {
	db := openUserTestDB(t)
	actor := seedUser(t, db, "admin")
	r := newUserRouter(db, actor)
	if w := doDelete(r, "not-a-uuid"); w.Code != http.StatusBadRequest {
		t.Fatalf("invalid id: got %d, want 400", w.Code)
	}
	if w := doDelete(r, uuid.New().String()); w.Code != http.StatusNotFound {
		t.Fatalf("missing id: got %d, want 404", w.Code)
	}
}
