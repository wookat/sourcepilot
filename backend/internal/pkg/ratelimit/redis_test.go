package ratelimit_test

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/redis/go-redis/v9"
	"github.com/trademind-ai/trademind/backend/internal/pkg/ratelimit"
	"github.com/trademind-ai/trademind/backend/internal/testing/safeenv"
	"golang.org/x/time/rate"
)

func testRedisClient(t *testing.T) *redis.Client {
	t.Helper()
	cfg, ok, err := safeenv.TestRedisURLFromEnv()
	if err != nil {
		t.Fatalf("unsafe TEST_REDIS_URL: %v", err)
	}
	if !ok {
		t.Skip("TEST_REDIS_URL not set; skipping Redis limiter integration test")
	}
	opts, err := redis.ParseURL(cfg.URL)
	if err != nil {
		t.Fatal(err)
	}
	client := redis.NewClient(opts)
	t.Cleanup(func() { _ = client.Close() })
	if err := client.Ping(context.Background()).Err(); err != nil {
		t.Skipf("test redis unreachable: %v", err)
	}
	return client
}

func testPolicy(t *testing.T, burst int) ratelimit.Policy {
	return ratelimit.Policy{
		ID:        fmt.Sprintf("test_%s_%d", t.Name(), time.Now().UnixNano()),
		Rate:      rate.Limit(1),
		Burst:     burst,
		TTL:       time.Minute,
		RetryHint: time.Second,
	}
}

func TestRedisLimiterSharedAcrossInstances(t *testing.T) {
	client := testRedisClient(t)
	p := testPolicy(t, 4)
	// Two limiter instances simulate two backend replicas sharing one budget.
	a := ratelimit.NewRedisLimiter(client, p)
	b := ratelimit.NewRedisLimiter(client, p)

	allowed := 0
	for i := 0; i < 8; i++ {
		l := a
		if i%2 == 1 {
			l = b
		}
		if l.Allow(context.Background(), "k").Allowed {
			allowed++
		}
	}
	if allowed != 4 {
		t.Fatalf("shared budget: allowed %d of 8, want burst=4", allowed)
	}
}

func TestRedisLimiterHasBudgetDoesNotConsume(t *testing.T) {
	client := testRedisClient(t)
	l := ratelimit.NewRedisLimiter(client, testPolicy(t, 2))
	for i := 0; i < 5; i++ {
		if !l.HasBudget(context.Background(), "k") {
			t.Fatalf("peek %d consumed budget", i)
		}
	}
	if !l.Allow(context.Background(), "k").Allowed || !l.Allow(context.Background(), "k").Allowed {
		t.Fatal("burst should still be intact after peeks")
	}
	if l.Allow(context.Background(), "k").Allowed {
		t.Fatal("third call should be denied")
	}
	if l.HasBudget(context.Background(), "k") {
		t.Fatal("peek should report empty budget")
	}
}

func TestRedisLimiterFallsBackWhenRedisDown(t *testing.T) {
	// Point at a closed port: every script call fails, the limiter must keep
	// limiting through its in-process fallback instead of failing open.
	client := redis.NewClient(&redis.Options{
		Addr:        "127.0.0.1:1",
		DialTimeout: 50 * time.Millisecond,
		MaxRetries:  -1,
	})
	t.Cleanup(func() { _ = client.Close() })
	p := testPolicy(t, 2)
	// Near-zero refill so dial-retry latency cannot add tokens mid-test.
	p.Rate = rate.Limit(0.001)
	l := ratelimit.NewRedisLimiter(client, p)

	allowed := 0
	for i := 0; i < 5; i++ {
		if l.Allow(context.Background(), "k").Allowed {
			allowed++
		}
	}
	if allowed != 2 {
		t.Fatalf("fallback budget: allowed %d of 5, want burst=2", allowed)
	}
}
