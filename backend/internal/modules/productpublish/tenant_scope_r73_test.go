package productpublish

import (
	"strings"
	"testing"

	"github.com/google/uuid"

	"github.com/trademind-ai/trademind/backend/internal/modules/product"
	"github.com/trademind-ai/trademind/backend/internal/modules/shop"
)

// Regression (R73): publish-target endpoints were tenant-unscoped, so a foreign
// tenant could enumerate another tenant's shops, check readiness of its
// products and create publish drafts for them. Product lookups must fail and
// foreign shops must never be usable as targets.
func TestPublishTargetsScopedByTenant(t *testing.T) {
	db := newBatchIntegrationDB(t)
	svc := newBatchTestService(db)
	svc.Shops = &shop.Service{DB: db}
	pid, sid := seedBatchProduct(t, db)
	const owner int64 = 7
	if err := db.Model(&product.Product{}).Where("id = ?", pid).Update("tenant_id", owner).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Model(&shop.Shop{}).Where("id = ?", sid).Update("tenant_id", owner).Error; err != nil {
		t.Fatal(err)
	}
	foreign := testGinContextTenant(8)

	if _, err := svc.CheckPublishTargets(foreign, pid, PublishTargetsCheckRequest{
		Targets: []PublishTargetRef{{Platform: "douyin_shop", ShopID: ptrString(sid.String())}},
	}); err == nil {
		t.Fatal("cross-tenant check publish targets: want error, got nil")
	}

	adminID := uuid.New()
	if _, err := svc.CreateBatchTargetDrafts(foreign, batchCreateReq(pid, sid, map[string]any{"priceRule": "fixed"}), &adminID); err == nil {
		t.Fatal("cross-tenant batch drafts: want error, got nil")
	}
	var tasks int64
	if err := db.Model(&ProductPublishTask{}).Count(&tasks).Error; err != nil {
		t.Fatal(err)
	}
	if tasks != 0 {
		t.Fatalf("cross-tenant batch must not create tasks, got %d", tasks)
	}

	// owner's own product remains reachable
	if _, err := svc.CheckPublishTargets(testGinContextTenant(owner), pid, PublishTargetsCheckRequest{
		Targets: []PublishTargetRef{{Platform: "douyin_shop", ShopID: ptrString(sid.String())}},
	}); err != nil {
		t.Fatalf("owner check publish targets: %v", err)
	}

	// shop visibility itself is tenant-scoped (drives the target shop list)
	if !svc.shopBelongsToTenant(foreign.Request.Context(), owner, sid) {
		t.Fatal("owner shop must be visible to its tenant")
	}
	if svc.shopBelongsToTenant(foreign.Request.Context(), 8, sid) {
		t.Fatal("shop must not be visible to a foreign tenant")
	}

	otherProd := &product.Product{TenantID: 8, Source: "manual", Title: "p8", Status: "draft"}
	if err := db.Create(otherProd).Error; err != nil {
		t.Fatal(err)
	}

	// targeting a foreign shop is blocked without disclosing its name
	res, err := svc.CheckPublishTargets(testGinContextTenant(8), otherProd.ID, PublishTargetsCheckRequest{
		Targets: []PublishTargetRef{{Platform: "douyin_shop", ShopID: ptrString(sid.String())}},
	})
	if err != nil {
		t.Fatalf("check own product with foreign shop: %v", err)
	}
	if len(res.Targets) != 1 || res.Targets[0].Status != statusBlocked {
		t.Fatalf("foreign shop target must be blocked, got %+v", res.Targets)
	}
	if strings.TrimSpace(res.Targets[0].ShopName) != "" {
		t.Fatalf("foreign shop name leaked: %q", res.Targets[0].ShopName)
	}
}

// Regression (R73): CancelTask loaded the task by bare id, letting a foreign
// tenant cancel another tenant's pending publish task.
func TestCancelTaskScopedByTenant(t *testing.T) {
	db := newBatchIntegrationDB(t)
	svc := newBatchTestService(db)
	pid, sid := seedBatchProduct(t, db)
	const owner int64 = 7
	task := &ProductPublishTask{
		TenantID:  owner,
		ProductID: pid,
		ShopID:    sid,
		Platform:  "douyin_shop",
		Status:    TaskPending,
	}
	if err := db.Create(task).Error; err != nil {
		t.Fatal(err)
	}
	adminID := uuid.New()

	if _, err := svc.CancelTask(testGinContextTenant(8), task.ID, &adminID); err == nil {
		t.Fatal("cross-tenant cancel: want error, got nil")
	}
	var after ProductPublishTask
	if err := db.First(&after, "id = ?", task.ID).Error; err != nil {
		t.Fatal(err)
	}
	if after.Status != TaskPending {
		t.Fatalf("cross-tenant cancel must not change status, got %s", after.Status)
	}
	if _, err := svc.CancelTask(testGinContextTenant(owner), task.ID, &adminID); err != nil {
		t.Fatalf("owner cancel: %v", err)
	}
	if err := db.First(&after, "id = ?", task.ID).Error; err != nil {
		t.Fatal(err)
	}
	if after.Status != TaskCancelled {
		t.Fatalf("owner cancel must cancel, got %s", after.Status)
	}
}
