package productpublish

import (
	"errors"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"gorm.io/gorm"

	"github.com/trademind-ai/trademind/backend/internal/modules/product"
	"github.com/trademind-ai/trademind/backend/internal/modules/shop"
	"github.com/trademind-ai/trademind/backend/internal/pkg/ctxkey"
)

func seedTenantBatch(t *testing.T, db *gorm.DB, svc *Service, tenantID int64) uuid.UUID {
	t.Helper()
	pid, sid := seedBatchProduct(t, db)
	if err := db.Model(&product.Product{}).Where("id = ?", pid).Update("tenant_id", tenantID).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Model(&shop.Shop{}).Where("id = ?", sid).Update("tenant_id", tenantID).Error; err != nil {
		t.Fatal(err)
	}
	adminID := uuid.New()
	out, err := svc.CreateBatchTargetDrafts(testGinContextTenant(tenantID), batchCreateReq(pid, sid, map[string]any{"priceRule": "fixed"}), &adminID)
	if err != nil {
		t.Fatal(err)
	}
	bid, err := uuid.Parse(out.BatchID)
	if err != nil {
		t.Fatal(err)
	}
	return bid
}

// Regression (R81): product_publish_batches had no tenant column, so batch
// detail/list/retry/cancel were reachable across tenants. Creation must stamp
// the request tenant and every batch entrypoint must hide foreign batches
// behind record-not-found.
func TestPublishBatchScopedByTenant(t *testing.T) {
	db := newBatchIntegrationDB(t)
	svc := newBatchTestService(db)
	const owner int64 = 7
	bid := seedTenantBatch(t, db, svc, owner)

	var row ProductPublishBatch
	if err := db.First(&row, "id = ?", bid).Error; err != nil {
		t.Fatal(err)
	}
	if row.TenantID != owner {
		t.Fatalf("batch tenant_id = %d, want %d", row.TenantID, owner)
	}

	foreign := testGinContextTenant(8)
	adminID := uuid.New()

	if _, err := svc.GetPublishBatchDetail(foreign, bid, &adminID); !errors.Is(err, gorm.ErrRecordNotFound) {
		t.Fatalf("cross-tenant batch detail: want ErrRecordNotFound, got %v", err)
	}
	if _, err := svc.RetryFailedBatchTasks(foreign, bid, &adminID); !errors.Is(err, gorm.ErrRecordNotFound) {
		t.Fatalf("cross-tenant batch retry: want ErrRecordNotFound, got %v", err)
	}
	if _, err := svc.CancelPendingBatchTasks(foreign, bid, &adminID); !errors.Is(err, gorm.ErrRecordNotFound) {
		t.Fatalf("cross-tenant batch cancel: want ErrRecordNotFound, got %v", err)
	}

	// owner keeps full access
	if _, err := svc.GetPublishBatchDetail(testGinContextTenant(owner), bid, nil); err != nil {
		t.Fatalf("owner batch detail: %v", err)
	}

	// list is tenant-filtered
	ownerList, _, err := svc.ListPublishBatches(testGinContextTenant(owner), 1, 20)
	if err != nil {
		t.Fatal(err)
	}
	if len(ownerList) != 1 {
		t.Fatalf("owner batch list len = %d, want 1", len(ownerList))
	}
	foreignList, total, err := svc.ListPublishBatches(testGinContextTenant(8), 1, 20)
	if err != nil {
		t.Fatal(err)
	}
	if len(foreignList) != 0 || total != 0 {
		t.Fatalf("foreign batch list must be empty, got len=%d total=%d", len(foreignList), total)
	}
}

// Regression (R81): RetryFailedOnly replay resolved the source batch by bare
// id, letting a foreign tenant read another tenant's failed targets.
func TestFailedTargetsFromBatchScopedByTenant(t *testing.T) {
	db := newBatchIntegrationDB(t)
	svc := newBatchTestService(db)
	const owner int64 = 7
	bid := seedTenantBatch(t, db, svc, owner)
	ctx := testGinContextTenant(owner).Request.Context()

	if _, err := svc.failedTargetsFromBatch(ctx, 8, bid.String()); !errors.Is(err, gorm.ErrRecordNotFound) {
		t.Fatalf("cross-tenant failed-targets replay: want ErrRecordNotFound, got %v", err)
	}
	if _, err := svc.failedTargetsFromBatch(ctx, owner, bid.String()); err != nil {
		t.Fatalf("owner failed-targets replay: %v", err)
	}
}

func performTenantRequest(t *testing.T, h *Handler, tenantID int64, id uuid.UUID, fn func(*gin.Context)) int {
	t.Helper()
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("POST", "/test", nil)
	c.Set(ctxkey.TenantID, tenantID)
	c.Params = gin.Params{{Key: "id", Value: id.String()}}
	fn(c)
	return w.Code
}

// Regression (R81): cross-tenant (record-not-found) errors on publish task /
// batch mutation endpoints surfaced as 400 with an error message, leaking
// existence. They must be indistinguishable from missing resources: 404.
func TestPublishMutationEndpointsCrossTenant404(t *testing.T) {
	db := newBatchIntegrationDB(t)
	svc := newBatchTestService(db)
	h := &Handler{Svc: svc}
	const owner int64 = 7

	pid, sid := seedBatchProduct(t, db)
	task := &ProductPublishTask{TenantID: owner, ProductID: pid, ShopID: sid, Platform: "douyin_shop", Status: TaskFailed}
	if err := db.Create(task).Error; err != nil {
		t.Fatal(err)
	}
	bid := seedTenantBatch(t, db, svc, owner)

	cases := []struct {
		name string
		id   uuid.UUID
		fn   func(*gin.Context)
	}{
		{"task retry", task.ID, h.RetryTask},
		{"task cancel", task.ID, h.CancelTask},
		{"task recover", task.ID, h.RecoverDouyinDraftTask},
		{"batch retry", bid, h.RetryFailedBatch},
		{"batch cancel", bid, h.CancelPendingBatch},
		{"batch detail", bid, h.GetPublishBatch},
	}
	for _, tc := range cases {
		if code := performTenantRequest(t, h, 8, tc.id, tc.fn); code != 404 {
			t.Fatalf("%s cross-tenant status = %d, want 404", tc.name, code)
		}
	}
}
