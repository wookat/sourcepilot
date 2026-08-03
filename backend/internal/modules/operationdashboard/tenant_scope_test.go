package operationdashboard

import (
	"strings"
	"testing"

	"github.com/glebarez/sqlite"
	"github.com/trademind-ai/trademind/backend/internal/modules/order"
	"github.com/trademind-ai/trademind/backend/internal/modules/sourcing"
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

// 仪表盘聚合必须按可信租户列过滤；TenantID 为 nil 时保持 legacy 行为。
func TestScopeApplyTenantColumn(t *testing.T) {
	db := newDryRunDB(t)
	tid := int64(2)
	sc := Scope{IsAdmin: true, TenantID: &tid}

	q := sc.applyTenantColumn(db.Session(&gorm.Session{DryRun: true}).Model(&order.Order{}), "tenant_id")
	sql := q.Find(&[]order.Order{}).Statement.SQL.String()
	if !strings.Contains(sql, "tenant_id = ?") {
		t.Fatalf("expected tenant predicate, got: %s", sql)
	}

	q0 := Scope{IsAdmin: true}.applyTenantColumn(db.Session(&gorm.Session{DryRun: true}).Model(&order.Order{}), "tenant_id")
	sql0 := q0.Find(&[]order.Order{}).Statement.SQL.String()
	if strings.Contains(sql0, "tenant_id") {
		t.Fatalf("nil TenantID must not add tenant predicate: %s", sql0)
	}
}

func TestScopeApplyTenantViaProductAndShop(t *testing.T) {
	db := newDryRunDB(t)
	tid := int64(2)
	sc := Scope{IsAdmin: true, TenantID: &tid}

	q := sc.applyTenantViaProduct(db.Session(&gorm.Session{DryRun: true}).Model(&sourcing.ProductSource{}), "product_id")
	sql := q.Find(&[]sourcing.ProductSource{}).Statement.SQL.String()
	if !strings.Contains(sql, "SELECT id FROM products WHERE tenant_id = ?") {
		t.Fatalf("expected via-product tenant condition, got: %s", sql)
	}

	q2 := sc.applyTenantViaShop(db.Session(&gorm.Session{DryRun: true}).Model(&sourcing.ProductSource{}), "shop_id")
	sql2 := q2.Find(&[]sourcing.ProductSource{}).Statement.SQL.String()
	if !strings.Contains(sql2, "SELECT id FROM shops WHERE tenant_id = ?") {
		t.Fatalf("expected via-shop tenant condition, got: %s", sql2)
	}
}

func TestTenantValue(t *testing.T) {
	if (Scope{}).tenantValue() != 0 {
		t.Fatalf("nil TenantID must map to 0")
	}
	tid := int64(7)
	if (Scope{TenantID: &tid}).tenantValue() != 7 {
		t.Fatalf("tenantValue must return trusted tenant id")
	}
}
