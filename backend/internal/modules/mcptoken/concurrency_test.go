package mcptoken_test

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"testing"

	"github.com/trademind-ai/trademind/backend/internal/modules/mcptoken"
	"github.com/trademind-ai/trademind/backend/internal/testing/postgrestest"
	"github.com/trademind-ai/trademind/backend/internal/testing/safeenv"
	"gorm.io/gorm"
)

// runConcurrentCreates fires n parallel Create calls for tenant 1 and
// asserts the active-token cap holds afterwards (the count→insert window
// must not admit extra tokens).
func runConcurrentCreates(t *testing.T, db *gorm.DB, n int) {
	t.Helper()
	svc := &mcptoken.Service{DB: db}
	var wg sync.WaitGroup
	errCh := make(chan error, n)
	start := make(chan struct{})
	for i := 0; i < n; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			<-start
			_, err := svc.Create(context.Background(), 1, fmt.Sprintf("conc-%d", i), nil, nil)
			errCh <- err
		}(i)
	}
	close(start)
	wg.Wait()
	close(errCh)
	var created, capped int
	for err := range errCh {
		switch {
		case err == nil:
			created++
		case errors.Is(err, mcptoken.ErrTooManyTokens):
			capped++
		default:
			t.Fatalf("unexpected create error: %v", err)
		}
	}
	var active int64
	if err := db.Model(&mcptoken.Token{}).
		Where("tenant_id = ? AND revoked_at IS NULL", 1).Count(&active).Error; err != nil {
		t.Fatal(err)
	}
	if active > mcptoken.MaxActiveTokensPerTenant {
		t.Fatalf("active tokens = %d, cap %d exceeded under concurrency (created=%d capped=%d)",
			active, mcptoken.MaxActiveTokensPerTenant, created, capped)
	}
	if created != mcptoken.MaxActiveTokensPerTenant {
		t.Fatalf("created = %d, want exactly %d", created, mcptoken.MaxActiveTokensPerTenant)
	}
}

// Concurrent creates on the in-process path (sqlite): the per-tenant mutex
// must close the count→insert race.
func TestCreateCapConcurrencySQLite(t *testing.T) {
	runConcurrentCreates(t, openTestDB(t), mcptoken.MaxActiveTokensPerTenant*2)
}

// Concurrent creates on PostgreSQL: the transaction-scoped advisory lock
// must close the race (also across replicas; here exercised in-process
// against a real PostgreSQL). Skips without a safe test database.
func TestCreateCapConcurrencyPostgres(t *testing.T) {
	if _, ok, _ := safeenv.TestDatabaseURLFromEnv(); !ok {
		t.Skip("TEST_DATABASE_URL is not set; skipping PostgreSQL concurrency regression")
	}
	h := postgrestest.Require(t)
	if err := h.DB.AutoMigrate(&mcptoken.Token{}); err != nil {
		t.Fatal(err)
	}
	runConcurrentCreates(t, h.DB, mcptoken.MaxActiveTokensPerTenant*2)
}
