package taskcenter

import (
	"strings"
	"testing"

	"github.com/glebarez/sqlite"
	"github.com/trademind-ai/trademind/backend/internal/modules/imagetask"
	"gorm.io/gorm"
)

func newDryRunDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open("file::memory:?cache=shared"), &gorm.Config{DryRun: true})
	if err != nil {
		t.Skipf("sqlite unavailable: %v", err)
	}
	return db
}

// 源表无 tenant_id 列的失败源必须经由关联表限定租户，而不是直接 WHERE tenant_id
func TestApplyTenantListFilterVia(t *testing.T) {
	db := newDryRunDB(t)
	s := &Service{DB: db}
	cond := "(product_id IS NULL OR product_id IN (SELECT id FROM products WHERE tenant_id = ?))"

	q := db.Session(&gorm.Session{DryRun: true}).Model(&imagetask.ImageTask{})
	q = s.applyTenantListFilterVia(q, ListFailureParams{TenantID: 1}, cond)
	stmt := q.Find(&[]imagetask.ImageTask{}).Statement
	sql := stmt.SQL.String()
	if !strings.Contains(sql, "SELECT id FROM products WHERE tenant_id = ?") {
		t.Fatalf("expected via-subquery tenant condition, got: %s", sql)
	}
	if strings.Contains(sql, `"image_tasks"."tenant_id"`) {
		t.Fatalf("must not filter on missing image_tasks.tenant_id column: %s", sql)
	}

	q0 := db.Session(&gorm.Session{DryRun: true}).Model(&imagetask.ImageTask{})
	q0 = s.applyTenantListFilterVia(q0, ListFailureParams{TenantID: 0}, cond)
	sql0 := q0.Find(&[]imagetask.ImageTask{}).Statement.SQL.String()
	if strings.Contains(sql0, "tenant_id") {
		t.Fatalf("tenant 0 must not apply tenant condition: %s", sql0)
	}
}
