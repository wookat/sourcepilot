package inventory

import (
	"context"
	"strings"
	"testing"

	"github.com/glebarez/sqlite"
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

// 库存告警/批量库存设置的共享基础查询必须按 products.tenant_id 限定租户。
func TestBuildSKUAlertBaseTXTenantScope(t *testing.T) {
	db := newDryRunDB(t)
	s := &Service{DB: db}
	tid := int64(2)

	q := s.buildSKUAlertBaseTX(context.Background(), skuAlertBaseQuery{TenantID: &tid})
	sql := q.Session(&gorm.Session{DryRun: true}).Find(&[]alertSKUScan{}).Statement.SQL.String()
	if !strings.Contains(sql, "p.tenant_id = ?") {
		t.Fatalf("expected tenant predicate on products join, got: %s", sql)
	}

	q0 := s.buildSKUAlertBaseTX(context.Background(), skuAlertBaseQuery{})
	sql0 := q0.Session(&gorm.Session{DryRun: true}).Find(&[]alertSKUScan{}).Statement.SQL.String()
	if strings.Contains(sql0, "tenant_id") {
		t.Fatalf("nil TenantID must not add tenant predicate: %s", sql0)
	}
}
