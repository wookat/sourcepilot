package integration

import (
	"context"
	"fmt"
	"os"
	"strconv"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"github.com/trademind-ai/trademind/backend/internal/database"
	"github.com/trademind-ai/trademind/backend/internal/modules/order"
	"github.com/trademind-ai/trademind/backend/internal/testing/safeenv"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

// Load/idempotency harness for the自动化订单规则 engine (R122): fires the
// order_created event for a batch of orders against a confirm_payment +
// mark_printed rule set, measures throughput, then replays every event and
// asserts the dedup guard leaves the log table unchanged (no duplicate
// executions, no duplicate rows).
//
// Batch size defaults to 500 so the suite stays fast; a万级 run is driven by
// PERF_AUTOMATION_ORDERS (e.g. 10000) for performance audits.
func TestAutomationEngineLoadIdempotency(t *testing.T) {
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
	sqlDB.SetMaxOpenConns(16)
	t.Cleanup(func() { sqlDB.Close() })

	require.NoError(t, database.AutoMigrate(db))

	const tenantID int64 = 990122
	cleanup := func() {
		db.Exec(`DELETE FROM order_automation_logs WHERE tenant_id = ?`, tenantID)
		db.Exec(`DELETE FROM order_automation_rules WHERE tenant_id = ?`, tenantID)
		db.Exec(`DELETE FROM order_items WHERE order_id IN (SELECT id FROM orders WHERE tenant_id = ?)`, tenantID)
		db.Exec(`DELETE FROM orders WHERE tenant_id = ?`, tenantID)
	}
	cleanup()
	t.Cleanup(cleanup)

	n := 500
	if v := os.Getenv("PERF_AUTOMATION_ORDERS"); v != "" {
		parsed, perr := strconv.Atoi(v)
		require.NoError(t, perr)
		n = parsed
	}

	maxAmount := 500.0
	rules := []order.OrderAutomationRule{
		{TenantID: tenantID, Name: "r122-confirm-payment", Priority: 1, Enabled: true,
			TriggerEvent: order.AutomationEventOrderCreated, Action: order.AutomationActionConfirmPayment,
			MaxAmount: &maxAmount},
		{TenantID: tenantID, Name: "r122-mark-printed", Priority: 2, Enabled: true,
			TriggerEvent: order.AutomationEventOrderPaid, Action: order.AutomationActionMarkPrinted},
	}
	require.NoError(t, db.Create(&rules).Error)

	orders := make([]order.Order, 0, n)
	for i := 0; i < n; i++ {
		orders = append(orders, order.Order{
			TenantID:          tenantID,
			Platform:          "manual",
			OrderNo:           fmt.Sprintf("R122-LOAD-%06d", i),
			CustomerName:      "r122-load",
			Status:            order.StatusPending,
			PaymentStatus:     order.PaymentUnpaid,
			FulfillmentStatus: order.FulfillmentUnfulfilled,
			Currency:          "CNY",
			TotalAmount:       float64(50 + i%400),
		})
	}
	require.NoError(t, db.CreateInBatches(&orders, 500).Error)

	svc := &order.Service{DB: db}
	ctx := context.Background()

	fireAll := func() time.Duration {
		start := time.Now()
		for _, o := range orders {
			svc.FireOrderEvent(ctx, tenantID, o.ID, order.AutomationEventOrderCreated)
		}
		return time.Since(start)
	}

	elapsed := fireAll()
	t.Logf("first pass: %d order_created events in %s (%.1f events/s)",
		n, elapsed, float64(n)/elapsed.Seconds())

	countLogs := func(status string) int64 {
		var c int64
		q := db.Model(&order.OrderAutomationLog{}).Where("tenant_id = ?", tenantID)
		if status != "" {
			q = q.Where("status = ?", status)
		}
		require.NoError(t, q.Count(&c).Error)
		return c
	}
	// confirm_payment succeeds for every order, chaining order_paid so the
	// mark_printed rule also runs: 2 success logs per order, no failures.
	require.Equal(t, int64(2*n), countLogs(order.AutomationLogSuccess))
	require.Equal(t, int64(0), countLogs(order.AutomationLogFailed))
	totalAfterFirst := countLogs("")

	var paid int64
	require.NoError(t, db.Model(&order.Order{}).
		Where("tenant_id = ? AND payment_status = ?", tenantID, order.PaymentPaid).
		Count(&paid).Error)
	require.Equal(t, int64(n), paid)

	// Replay every event: the dedup guard must skip all executed rules and
	// write no additional log rows (idempotency under duplicate delivery).
	replay := fireAll()
	t.Logf("replay pass: %d duplicate events in %s (%.1f events/s)",
		n, replay, float64(n)/replay.Seconds())
	require.Equal(t, totalAfterFirst, countLogs(""))
	require.Equal(t, int64(2*n), countLogs(order.AutomationLogSuccess))

	// The dedup key is unique per rule+order+event: assert at the SQL level
	// that no duplicate executions slipped through.
	var dups int64
	require.NoError(t, db.Raw(`SELECT COUNT(*) FROM (
		SELECT dedup_key FROM order_automation_logs WHERE tenant_id = ?
		GROUP BY dedup_key HAVING COUNT(*) > 1) d`, tenantID).Scan(&dups).Error)
	require.Equal(t, int64(0), dups)
}
