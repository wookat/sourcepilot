package integration

import (
	"context"
	"fmt"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"github.com/trademind-ai/trademind/backend/internal/database"
	"github.com/trademind-ai/trademind/backend/internal/modules/inventory"
	"github.com/trademind-ai/trademind/backend/internal/modules/order"
	"github.com/trademind-ai/trademind/backend/internal/modules/orderexception"
	"github.com/trademind-ai/trademind/backend/internal/modules/ordersync"
	"github.com/trademind-ai/trademind/backend/internal/testing/safeenv"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

// TestExceptionSummaryParity verifies the SQL-aggregated summary
// (SummaryOpenExceptions, R188) returns exactly the same numbers as the
// in-memory summary computed by ListOrderExceptions, across tenants, store
// scopes, platform filters and handled/ignored marks.
func TestExceptionSummaryParity(t *testing.T) {
	t.Setenv("APP_ENV", "test")
	cfg, ok, err := safeenv.TestDatabaseURLFromEnv()
	require.NoError(t, err)
	if !ok {
		t.Skip("TEST_DATABASE_URL is not set; skipping PostgreSQL integration test")
	}

	db, err := gorm.Open(postgres.Open(cfg.URL), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	require.NoError(t, err)
	sqlDB, err := db.DB()
	require.NoError(t, err)
	t.Cleanup(func() { sqlDB.Close() })

	require.NoError(t, database.AutoMigrate(db))

	tenants := []int64{990188, 990189}
	cleanup := func() {
		for _, tid := range tenants {
			db.Exec(`DELETE FROM order_exception_marks WHERE order_id IN (SELECT id FROM orders WHERE tenant_id = ?)`, tid)
			db.Exec(`DELETE FROM order_inventory_effects WHERE order_id IN (SELECT id FROM orders WHERE tenant_id = ?)`, tid)
			db.Exec(`DELETE FROM order_item_sku_matches WHERE order_id IN (SELECT id FROM orders WHERE tenant_id = ?)`, tid)
			db.Exec(`DELETE FROM order_items WHERE order_id IN (SELECT id FROM orders WHERE tenant_id = ?)`, tid)
			db.Exec(`DELETE FROM orders WHERE tenant_id = ?`, tid)
			db.Exec(`DELETE FROM inventory_sync_tasks WHERE tenant_id = ?`, tid)
			db.Exec(`DELETE FROM order_sync_tasks WHERE tenant_id = ?`, tid)
		}
	}
	cleanup()
	t.Cleanup(cleanup)

	svc := &orderexception.Service{DB: db}
	ctx := context.Background()

	type shopPair struct{ a, b uuid.UUID }
	shops := map[int64]shopPair{}

	for _, tid := range tenants {
		sp := shopPair{a: uuid.New(), b: uuid.New()}
		shops[tid] = sp
		for i := 0; i < 6; i++ {
			shop := sp.a
			platform := "douyin"
			if i%2 == 1 {
				shop = sp.b
				platform = "tiktok"
			}
			o := order.Order{
				TenantID:          tid,
				Platform:          platform,
				ShopID:            &shop,
				OrderNo:           fmt.Sprintf("R188-%d-%d", tid, i),
				Status:            order.StatusPending,
				PaymentStatus:     order.PaymentPaid,
				FulfillmentStatus: order.FulfillmentUnfulfilled,
			}
			require.NoError(t, db.Create(&o).Error)

			// Order item without a bound SKU → sku_unmatched (source order_item).
			oi := order.OrderItem{OrderID: o.ID, Quantity: 1, SKUCode: fmt.Sprintf("SKU-%d-%d", tid, i)}
			require.NoError(t, db.Create(&oi).Error)

			// Second item carrying an ambiguous match → sku_ambiguous.
			oi2 := order.OrderItem{OrderID: o.ID, Quantity: 2, SKUCode: fmt.Sprintf("SKU2-%d-%d", tid, i)}
			require.NoError(t, db.Create(&oi2).Error)
			m := order.OrderItemSKUMatch{
				OrderID:     o.ID,
				OrderItemID: oi2.ID,
				Platform:    platform,
				MatchType:   order.MatchTypeNone,
				MatchStatus: order.MatchStatusAmbiguous,
			}
			require.NoError(t, db.Create(&m).Error)

			// Failed deduct effects: insufficient vs generic failure.
			msg := "insufficient stock"
			if i%3 == 0 {
				msg = "deduct backend error"
			}
			eff := inventory.OrderInventoryEffect{
				OrderID:      o.ID,
				OrderItemID:  oi.ID,
				ProductSKUID: inventory.NilInventorySKUUID,
				EffectType:   inventory.EffectTypeDeduct,
				Quantity:     1,
				Status:       inventory.InventoryEffectFailed,
				ErrorMessage: msg,
			}
			require.NoError(t, db.Create(&eff).Error)

			// Failed restore effect on every second order.
			if i%2 == 0 {
				eff2 := inventory.OrderInventoryEffect{
					OrderID:      o.ID,
					OrderItemID:  oi2.ID,
					ProductSKUID: inventory.NilInventorySKUUID,
					EffectType:   inventory.EffectTypeRestore,
					Quantity:     1,
					Status:       inventory.InventoryEffectFailed,
					ErrorMessage: "restore failed",
				}
				require.NoError(t, db.Create(&eff2).Error)
			}

			// Mark one sku_unmatched row handled and one ignored per tenant.
			if i == 0 || i == 1 {
				markType := orderexception.MarkHandled
				if i == 1 {
					markType = orderexception.MarkIgnored
				}
				oid := o.ID
				mark := orderexception.OrderExceptionMark{
					ExceptionType: orderexception.TypeSKUUnmatched,
					SourceType:    orderexception.SourceOrderItem,
					SourceID:      oi.ID.String(),
					MarkType:      markType,
					OrderID:       &oid,
				}
				require.NoError(t, db.Create(&mark).Error)
			}
		}

		// Sync-task exceptions on shop A only.
		ist := inventory.InventorySyncTask{
			TenantID:  tid,
			ProductID: uuid.New(),
			ShopID:    sp.a,
			Platform:  "douyin",
			TaskType:  "stock_push",
			Status:    inventory.StatusFailed,
			Mode:      "auto",
		}
		require.NoError(t, db.Create(&ist).Error)
		ost := ordersync.OrderSyncTask{
			TenantID: tid,
			ShopID:   sp.a,
			Platform: "douyin",
			TaskType: "orders",
			Status:   ordersync.StatusPartialSuccess,
			Mode:     "auto",
		}
		require.NoError(t, db.Create(&ost).Error)
	}

	assertParity := func(t *testing.T, req orderexception.ListOrderExceptionsRequest) {
		t.Helper()
		req.Page = 1
		req.PageSize = 1
		listRes, err := svc.ListOrderExceptions(ctx, req)
		require.NoError(t, err)
		sqlSum, err := svc.SummaryOpenExceptions(ctx, req)
		require.NoError(t, err)
		require.Equal(t, listRes.Summary, sqlSum)
		if req.AllowedShopIDs == nil || len(req.AllowedShopIDs) > 0 {
			require.Positive(t, sqlSum.TotalOpen, "scenario should produce open exceptions")
		} else {
			require.Zero(t, sqlSum.TotalOpen, "empty store scope must hide everything")
		}
	}

	for _, tid := range tenants {
		tid := tid
		sp := shops[tid]
		t.Run(fmt.Sprintf("tenant_%d_unrestricted", tid), func(t *testing.T) {
			assertParity(t, orderexception.ListOrderExceptionsRequest{TenantID: &tid})
		})
		t.Run(fmt.Sprintf("tenant_%d_shop_scope", tid), func(t *testing.T) {
			assertParity(t, orderexception.ListOrderExceptionsRequest{
				TenantID: &tid, AllowedShopIDs: []uuid.UUID{sp.a},
			})
		})
		t.Run(fmt.Sprintf("tenant_%d_empty_scope", tid), func(t *testing.T) {
			assertParity(t, orderexception.ListOrderExceptionsRequest{
				TenantID: &tid, AllowedShopIDs: []uuid.UUID{},
			})
		})
		t.Run(fmt.Sprintf("tenant_%d_platform_filter", tid), func(t *testing.T) {
			assertParity(t, orderexception.ListOrderExceptionsRequest{
				TenantID: &tid, Platform: "douyin",
			})
		})
		t.Run(fmt.Sprintf("tenant_%d_shop_filter", tid), func(t *testing.T) {
			assertParity(t, orderexception.ListOrderExceptionsRequest{
				TenantID: &tid, ShopID: sp.b.String(),
			})
		})
		t.Run(fmt.Sprintf("tenant_%d_type_filter", tid), func(t *testing.T) {
			assertParity(t, orderexception.ListOrderExceptionsRequest{
				TenantID: &tid, ExceptionType: orderexception.TypeSKUUnmatched,
			})
		})
	}
}
