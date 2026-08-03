package productpublish

import (
	"testing"

	"github.com/google/uuid"

	"github.com/trademind-ai/trademind/backend/internal/modules/product"
)

// Round76 regression: batch-created publish sub-tasks must carry the owning
// product's tenant, otherwise the tenant-scoped ListTasks (子任务 tab) shows
// nothing while the batch detail (batch_id lookup) shows the tasks.
func TestBatchLocalDraftTaskCarriesProductTenant(t *testing.T) {
	db := newBatchIntegrationDB(t)
	svc := newBatchTestService(db)
	pid, sid := seedBatchProduct(t, db)
	const tenantID int64 = 7
	if err := db.Model(&product.Product{}).Where("id = ?", pid).
		Update("tenant_id", tenantID).Error; err != nil {
		t.Fatal(err)
	}
	adminID := uuid.New()
	c := testGinContext()
	req := batchCreateReq(pid, sid, map[string]any{"priceRule": "fixed"})
	if _, err := svc.CreateBatchTargetDrafts(c, req, &adminID); err != nil {
		t.Fatal(err)
	}
	var tasks []ProductPublishTask
	if err := db.Find(&tasks).Error; err != nil {
		t.Fatal(err)
	}
	if len(tasks) == 0 {
		t.Fatal("expected batch sub-tasks to be created")
	}
	for _, task := range tasks {
		if task.TenantID != tenantID {
			t.Fatalf("task %s tenant_id = %d, want %d", task.ID, task.TenantID, tenantID)
		}
	}
}
