package procurement

import (
	"context"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"github.com/google/uuid"
	"gorm.io/gorm"

	"github.com/trademind-ai/trademind/backend/internal/modules/admin"
	"github.com/trademind-ai/trademind/backend/internal/pkg/ctxkey"
	"github.com/trademind-ai/trademind/backend/internal/pkg/model"
)

func TestVoidOnlyAllowedFromTerminalStatuses(t *testing.T) {
	f := setupFixture(t)
	ctx := context.Background()
	po := generate(t, f, "key-void-status").Orders[0]

	// draft (non-terminal) must be rejected
	if _, err := f.svc.Void(ctx, po.ID, "test data", nil); err == nil {
		t.Fatalf("draft→voided must be rejected")
	}

	// delivered → voided is legal
	if err := f.svc.DB.Model(&PurchaseOrder{}).Where("id = ?", po.ID).
		Update("status", StatusDelivered).Error; err != nil {
		t.Fatal(err)
	}
	got, err := f.svc.Void(ctx, po.ID, "test data", nil)
	if err != nil {
		t.Fatalf("void delivered: %v", err)
	}
	if got.Status != StatusVoided {
		t.Fatalf("expected voided, got %s", got.Status)
	}

	// voided is terminal: no further transitions
	if _, err := f.svc.Void(ctx, po.ID, "again", nil); err == nil {
		t.Fatalf("voided→voided must be rejected")
	}
	if _, err := f.svc.Retry(ctx, po.ID, nil); err == nil {
		t.Fatalf("voided→placing must be rejected")
	}

	// audit event delivered→voided is preserved
	detail, err := f.svc.Detail(ctx, po.ID)
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, ev := range detail.Events {
		if ev.FromStatus == StatusDelivered && ev.ToStatus == StatusVoided {
			found = true
		}
	}
	if !found {
		t.Fatalf("expected delivered→voided audit event, got %+v", detail.Events)
	}
}

func TestVoidStateMachineTable(t *testing.T) {
	cases := []struct {
		from string
		ok   bool
	}{
		{StatusDelivered, true},
		{StatusFailed, true},
		{StatusCancelled, true},
		{StatusDraft, false},
		{StatusPendingConfirm, false},
		{StatusPlacing, false},
		{StatusPlaced, false},
		{StatusPaid, false},
		{StatusShipped, false},
		{StatusVoided, false},
	}
	for _, c := range cases {
		if got := CanTransition(c.from, StatusVoided); got != c.ok {
			t.Fatalf("%s→voided expected %v", c.from, c.ok)
		}
	}
}

// Voided purchase orders must stop covering sales-order lines so the
// lines become generatable again (same rule as cancelled/failed).
func TestVoidedOrderExcludedFromGenerateCoverage(t *testing.T) {
	f := setupFixture(t)
	first := generate(t, f, "key-void-cov-1")
	if len(first.Orders) != 1 {
		t.Fatalf("expected 1 purchase order, got %+v", first)
	}
	// while active the line stays covered
	second := generate(t, f, "key-void-cov-2")
	if len(second.Orders) != 0 {
		t.Fatalf("expected covered lines, got %+v", second.Orders)
	}
	// void the (delivered) order → coverage released
	if err := f.svc.DB.Model(&PurchaseOrder{}).Where("id = ?", first.Orders[0].ID).
		Update("status", StatusDelivered).Error; err != nil {
		t.Fatal(err)
	}
	if _, err := f.svc.Void(context.Background(), first.Orders[0].ID, "test data", nil); err != nil {
		t.Fatalf("void: %v", err)
	}
	third := generate(t, f, "key-void-cov-3")
	if len(third.Orders) != 1 {
		t.Fatalf("expected regeneration after void, got %+v", third)
	}
}

// readonly admins must get 403 on the void endpoint at the route level.
func TestVoidRouteRejectsReadonly(t *testing.T) {
	dsn := filepath.Join(t.TempDir(), "guard.db")
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Skipf("sqlite unavailable: %v", err)
	}
	if err := db.AutoMigrate(&admin.AdminUser{}, &admin.UserStorePermission{}, &PurchaseOrder{}); err != nil {
		t.Fatal(err)
	}
	uid := uuid.New()
	if err := db.Create(&admin.AdminUser{
		Base:         model.Base{ID: uid},
		Username:     "ro-" + uid.String()[:8],
		Email:        "ro-" + uid.String()[:8] + "@example.com",
		PasswordHash: "x",
		Role:         "readonly",
		Status:       "active",
	}).Error; err != nil {
		t.Fatalf("seed readonly user: %v", err)
	}

	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(func(c *gin.Context) {
		c.Set(ctxkey.AdminID, uid.String())
		c.Set(ctxkey.TenantID, int64(0))
		c.Next()
	})
	Register(r.Group("/api/v1"), &Handler{Svc: &Service{DB: db}})

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost,
		"/api/v1/procurement/orders/"+uuid.NewString()+"/void", strings.NewReader(`{"reason":"x"}`))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)
	if w.Code != http.StatusForbidden {
		t.Fatalf("readonly void: got %d, want 403", w.Code)
	}
}
