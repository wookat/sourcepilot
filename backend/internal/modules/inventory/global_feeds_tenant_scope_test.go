package inventory

import (
	"context"
	"fmt"
	"testing"

	"github.com/glebarez/sqlite"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

func setupGlobalFeedFixture(t *testing.T) *Service {
	t.Helper()
	dsn := fmt.Sprintf("file:globalfeed_%s?mode=memory&cache=shared", uuid.NewString())
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Skipf("sqlite unavailable: %v", err)
	}
	if err := db.AutoMigrate(&orderMirror{}, &InventoryChangeLog{}, &OrderInventoryEffect{}); err != nil {
		t.Fatal(err)
	}
	return &Service{DB: db}
}

// 全局库存流水必须按调用方租户过滤，不得跨租户返回其它租户的流水。
func TestListGlobalLogsTenantScope(t *testing.T) {
	s := setupGlobalFeedFixture(t)
	for _, tid := range []int64{0, 2} {
		log := InventoryChangeLog{
			TenantID:         tid,
			ProductID:        uuid.New(),
			ProductSKUID:     uuid.New(),
			ChangeType:       "manual_adjust",
			BeforeStock:      1,
			AfterStock:       2,
			Delta:            1,
			BusinessEventKey: fmt.Sprintf("evt-%d-%s", tid, uuid.NewString()),
		}
		if err := s.DB.Create(&log).Error; err != nil {
			t.Fatal(err)
		}
	}

	res, err := s.ListGlobalLogs(context.Background(), GlobalLogsQuery{TenantID: 2})
	if err != nil {
		t.Fatal(err)
	}
	if res.Total != 1 {
		t.Fatalf("tenant 2 should only see its own log, got total=%d", res.Total)
	}

	res0, err := s.ListGlobalLogs(context.Background(), GlobalLogsQuery{TenantID: 0})
	if err != nil {
		t.Fatal(err)
	}
	if res0.Total != 1 {
		t.Fatalf("tenant 0 should only see tenant-0 logs, got total=%d", res0.Total)
	}
}

// 全局订单库存联动列表必须经 orders.tenant_id 限定，不得跨租户返回。
func TestListOrderEffectsGlobalTenantScope(t *testing.T) {
	s := setupGlobalFeedFixture(t)
	for _, tid := range []int64{0, 2} {
		o := orderMirror{TenantID: tid, OrderNo: fmt.Sprintf("ORD-%d", tid), Status: "paid", PaymentStatus: "paid"}
		if err := s.DB.Create(&o).Error; err != nil {
			t.Fatal(err)
		}
		eff := OrderInventoryEffect{
			OrderID:      o.ID,
			OrderItemID:  uuid.New(),
			ProductSKUID: uuid.New(),
			EffectType:   "deduct",
			Quantity:     1,
			Status:       "applied",
		}
		if err := s.DB.Create(&eff).Error; err != nil {
			t.Fatal(err)
		}
	}

	res, err := s.ListOrderEffectsGlobal(context.Background(), OrderEffectsQuery{TenantID: 2})
	if err != nil {
		t.Fatal(err)
	}
	if res.Total != 1 {
		t.Fatalf("tenant 2 should only see effects of its own orders, got total=%d", res.Total)
	}
}
