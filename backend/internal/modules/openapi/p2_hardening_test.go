package openapi_test

import (
	"context"
	"fmt"
	"net/http"
	"testing"

	"github.com/trademind-ai/trademind/backend/internal/modules/mcpaudit"
	"github.com/trademind-ai/trademind/backend/internal/modules/mcptoken"
	"gorm.io/gorm"
)

// R154 P2 regression: invalid pagination parameters answer 400 instead of
// being silently normalized, matching the invalid-date policy.
func TestPaginationInvalidValuesAnswer400(t *testing.T) {
	db := openTestDB(t)
	seedOrders(t, db)
	srv, tokens := newTestServer(t, db, 100, 100)
	tok, err := tokens.Create(context.Background(), 1, "t1", mcptoken.PurposeOpenAPI, nil, nil)
	if err != nil {
		t.Fatal(err)
	}

	bad := []string{
		"/api/open/v1/orders?page=abc",
		"/api/open/v1/orders?pageSize=-1",
		"/api/open/v1/orders?page=0",
		"/api/open/v1/inventory?pageSize=abc",
		"/api/open/v1/exceptions?page=-5",
	}
	for _, path := range bad {
		res, body := get(t, srv, path, tok.Plaintext)
		if res.StatusCode != http.StatusBadRequest {
			t.Fatalf("%s: want 400 got %d", path, res.StatusCode)
		}
		if code, _ := body["code"].(float64); code != 40001 {
			t.Fatalf("%s: want business code 40001, got %v", path, body["code"])
		}
	}

	// Absent and valid values still work; oversized pageSize stays clamped.
	for _, path := range []string{
		"/api/open/v1/orders",
		"/api/open/v1/orders?page=1&pageSize=5",
		"/api/open/v1/orders?pageSize=500",
	} {
		res, _ := get(t, srv, path, tok.Plaintext)
		if res.StatusCode != http.StatusOK {
			t.Fatalf("%s: want 200 got %d", path, res.StatusCode)
		}
	}
}

func countAudits(t *testing.T, db *gorm.DB, tool, status string, tenantID int64) int64 {
	t.Helper()
	var n int64
	if err := db.Model(&mcpaudit.ToolCallLog{}).
		Where("tool = ? AND status = ? AND tenant_id = ?", tool, status, tenantID).
		Count(&n).Error; err != nil {
		t.Fatal(err)
	}
	return n
}

// R154 P2 regression: rejected credentials leave an auth_failed audit row
// (tenant 0, throttled per source), so brute-force attempts are visible in
// the audit table and not only in logs.
func TestAuthFailureWritesAuditRow(t *testing.T) {
	db := openTestDB(t)
	srv, _ := newTestServer(t, db, 100, 100)

	res, _ := get(t, srv, "/api/open/v1/orders", "")
	if res.StatusCode != http.StatusUnauthorized {
		t.Fatalf("want 401 got %d", res.StatusCode)
	}
	if n := countAudits(t, db, "openapi:auth", mcpaudit.StatusAuthFailed, 0); n != 1 {
		t.Fatalf("want 1 auth_failed audit row, got %d", n)
	}

	// Repeats within the throttle window do not grow the table.
	badToken := mcptoken.TokenPrefix + fmt.Sprintf("%064d", 0)
	for i := 0; i < 3; i++ {
		res, _ = get(t, srv, "/api/open/v1/orders", badToken)
		if res.StatusCode != http.StatusUnauthorized {
			t.Fatalf("want 401 got %d", res.StatusCode)
		}
	}
	if n := countAudits(t, db, "openapi:auth", mcpaudit.StatusAuthFailed, 0); n != 1 {
		t.Fatalf("throttle: want 1 auth_failed audit row, got %d", n)
	}
}

// R154 P2 regression: a rate-limited authenticated call leaves a rate_limited
// audit row attributed to the token's tenant.
func TestRateLimitWritesAuditRow(t *testing.T) {
	db := openTestDB(t)
	seedOrders(t, db)
	srv, tokens := newTestServer(t, db, 1, 1)
	tok, err := tokens.Create(context.Background(), 1, "t1", mcptoken.PurposeOpenAPI, nil, nil)
	if err != nil {
		t.Fatal(err)
	}

	got429 := false
	for i := 0; i < 5; i++ {
		res, _ := get(t, srv, "/api/open/v1/orders", tok.Plaintext)
		if res.StatusCode == http.StatusTooManyRequests {
			got429 = true
			break
		}
	}
	if !got429 {
		t.Fatal("expected a 429 within 5 rapid calls at burst 1")
	}
	if n := countAudits(t, db, "openapi:auth", mcpaudit.StatusRateLimited, 1); n != 1 {
		t.Fatalf("want 1 rate_limited audit row for tenant 1, got %d", n)
	}
}
