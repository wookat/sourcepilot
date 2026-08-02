package orderexception

import (
	"context"
	"fmt"
	"testing"

	"github.com/glebarez/sqlite"
	"github.com/google/uuid"
	"github.com/trademind-ai/trademind/backend/internal/modules/ordersync"
	"github.com/trademind-ai/trademind/backend/internal/modules/shop"
	"gorm.io/gorm"
)

func openOrderSyncMarkTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	dsn := fmt.Sprintf("file:ordersyncmark_%s?mode=memory&cache=shared&_pragma=busy_timeout(5000)", uuid.New().String())
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Skipf("sqlite unavailable: %v", err)
	}
	sqlDB, err := db.DB()
	if err != nil {
		t.Fatal(err)
	}
	sqlDB.SetMaxOpenConns(1)
	if err := db.AutoMigrate(
		&shop.Shop{},
		&ordersync.OrderSyncTask{},
		&OrderExceptionMark{},
	); err != nil {
		t.Fatal(err)
	}
	return db
}

// Regression: marking a 页级同步失败 (order_sync_partial_failed) row as
// handled/ignored must succeed; UpsertMark previously rejected
// sourceType=order_sync_task with "unsupported sourceType".
func TestUpsertMarkOrderSyncTask(t *testing.T) {
	db := openOrderSyncMarkTestDB(t)
	svc := &Service{DB: db}

	task := ordersync.OrderSyncTask{Platform: "douyin_shop", Status: ordersync.StatusPartialSuccess}
	if err := db.Create(&task).Error; err != nil {
		t.Fatal(err)
	}

	if err := svc.UpsertMark(context.Background(), TypeOrderSyncPartialFailed, SourceOrderSyncTask, task.ID.String(), MarkHandled, "已重试失败页", nil); err != nil {
		t.Fatalf("UpsertMark(order_sync_task, handled): %v", err)
	}

	res, err := svc.ListOrderExceptions(context.Background(), ListOrderExceptionsRequest{
		ExceptionType: TypeOrderSyncPartialFailed,
		All:           boolPtr(true),
		Page:          1,
		PageSize:      10,
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(res.List) != 1 {
		t.Fatalf("want 1 row, got %d", len(res.List))
	}
	if !res.List[0].Handled || res.List[0].Status != StatusHandled {
		t.Fatalf("row not marked handled: %+v", res.List[0])
	}

	// switching to ignored replaces the handled mark
	if err := svc.UpsertMark(context.Background(), TypeOrderSyncPartialFailed, SourceOrderSyncTask, task.ID.String(), MarkIgnored, "", nil); err != nil {
		t.Fatalf("UpsertMark(order_sync_task, ignored): %v", err)
	}
	res, err = svc.ListOrderExceptions(context.Background(), ListOrderExceptionsRequest{
		ExceptionType: TypeOrderSyncPartialFailed,
		All:           boolPtr(true),
		Page:          1,
		PageSize:      10,
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(res.List) != 1 || !res.List[0].Ignored || res.List[0].Handled {
		t.Fatalf("row not switched to ignored: %+v", res.List[0])
	}
}
