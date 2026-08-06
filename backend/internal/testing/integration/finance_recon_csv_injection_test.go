package integration

import (
	"fmt"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"github.com/trademind-ai/trademind/backend/internal/api"
	"github.com/trademind-ai/trademind/backend/internal/config"
	"github.com/trademind-ai/trademind/backend/internal/database"
	"github.com/trademind-ai/trademind/backend/internal/middleware"
	"github.com/trademind-ai/trademind/backend/internal/modules/admin"
	"github.com/trademind-ai/trademind/backend/internal/modules/auth"
	"github.com/trademind-ai/trademind/backend/internal/modules/finance"
	"github.com/trademind-ai/trademind/backend/internal/modules/order"
	"github.com/trademind-ai/trademind/backend/internal/modules/shop"
	"github.com/trademind-ai/trademind/backend/internal/pkg/model"
	"github.com/trademind-ai/trademind/backend/internal/testing/safeenv"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

const reconCSVTenant int64 = 913601

// E2E regression (R136, closes v20 P2-3): a hostile order currency such as
// "=1+2" must come back formula-escaped ("'=1+2") from the real reconciliation
// CSV export route — full production router, auth middleware, service and CSV
// writer, against a real PostgreSQL database. Complements the finance unit
// test TestReconciliationCSVCurrencyEscaped which exercises the service only.
func TestRound136ReconciliationCSVCurrencyInjectionE2E(t *testing.T) {
	t.Setenv("APP_ENV", "test")
	dbCfg, ok, err := safeenv.TestDatabaseURLFromEnv()
	require.NoError(t, err)
	if !ok {
		t.Skip("TEST_DATABASE_URL is not set; skipping reconciliation CSV injection E2E test")
	}

	gin.SetMode(gin.TestMode)
	db, err := gorm.Open(postgres.Open(dbCfg.URL), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	require.NoError(t, err)
	sqlDB, err := db.DB()
	require.NoError(t, err)
	t.Cleanup(func() { sqlDB.Close() })
	require.NoError(t, database.AutoMigrate(db))

	cleanup := func() {
		db.Exec("DELETE FROM finance_payment_records WHERE tenant_id = ?", reconCSVTenant)
		db.Exec("DELETE FROM order_items WHERE order_id IN (SELECT id FROM orders WHERE tenant_id = ?)", reconCSVTenant)
		db.Exec("DELETE FROM orders WHERE tenant_id = ?", reconCSVTenant)
		db.Exec("DELETE FROM shops WHERE tenant_id = ?", reconCSVTenant)
		db.Exec("DELETE FROM admin_users WHERE tenant_id = ?", reconCSVTenant)
	}
	cleanup()
	t.Cleanup(cleanup)

	cfg := &config.Config{
		AppEnv:                  "test",
		JWTSecret:               "recon-csv-e2e-jwt-secret-with-safe-length",
		JWTExpHrs:               1,
		MasterKey:               "recon-csv-e2e-master-key-0123456789abcd",
		CollectorTimeoutSeconds: 1,
		CollectorBaseURL:        "http://127.0.0.1:59322", // intentionally unreachable
	}
	keys, err := auth.BuildKeySet(cfg)
	require.NoError(t, err)

	engine := gin.New()
	engine.Use(middleware.RequestID())
	api.Register(engine, &api.Deps{Config: cfg, DB: db, MigrationsReady: true})

	adminID := uuid.New()
	require.NoError(t, db.Create(&admin.AdminUser{
		Base:         model.Base{ID: adminID},
		TenantID:     reconCSVTenant,
		Username:     admin.NewInternalUsername(),
		PasswordHash: "recon-csv-e2e-fixture",
		Role:         "admin",
		Status:       admin.StatusActive,
	}).Error)
	token, _, err := auth.MintAccessToken(cfg, keys, auth.MintAccessInput{
		UserID: adminID, Username: "recon-csv-e2e", TenantID: reconCSVTenant, TokenVersion: 1,
	})
	require.NoError(t, err)

	sh := shop.Shop{TenantID: reconCSVTenant, Platform: "manual", ShopName: "recon-csv-e2e-shop",
		Status: "active", AuthStatus: "authorized", Currency: "CNY"}
	require.NoError(t, db.Create(&sh).Error)

	// Hostile currency straight in the database, as an attacker-controlled
	// upstream value would land (UI offers no such input path).
	now := time.Now()
	o := order.Order{TenantID: reconCSVTenant, Platform: sh.Platform, ShopID: &sh.ID,
		OrderNo: "RECON-CSV-E2E-0001", CustomerName: "recon-csv-e2e",
		Status: order.StatusPaid, PaymentStatus: order.PaymentPaid,
		FulfillmentStatus: order.FulfillmentUnfulfilled,
		Currency:          "=1+2", TotalAmount: 100, OrderedAt: &now}
	require.NoError(t, db.Create(&o).Error)
	require.NoError(t, db.Create(&finance.PaymentRecord{
		TenantID: reconCSVTenant, OrderID: o.ID, ShopID: o.ShopID,
		Amount: 100, Currency: "=1+2", ReceivedAt: now, Source: finance.SourceManual,
	}).Error)

	day := now.Format("2006-01-02")
	req := httptest.NewRequest("GET",
		fmt.Sprintf("/api/v1/finance/reconciliation/export.csv?start=%s&end=%s", day, day), nil)
	req.Header.Set("Authorization", "Bearer "+token)
	w := httptest.NewRecorder()
	engine.ServeHTTP(w, req)

	require.Equal(t, 200, w.Code)
	require.Contains(t, w.Header().Get("Content-Type"), "text/csv")
	body := w.Body.String()
	require.Contains(t, body, "RECON-CSV-E2E-0001", "exported CSV must include the seeded order row")

	for _, line := range strings.Split(body, "\n") {
		if !strings.Contains(line, "RECON-CSV-E2E-0001") {
			continue
		}
		require.Contains(t, line, "'=1+2", "currency cell must be formula-escaped")
		require.NotContains(t, line, ",=1+2", "raw formula trigger must not survive in the row")
	}
}
