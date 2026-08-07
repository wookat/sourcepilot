package mcpaudit_test

import (
	"context"
	"testing"

	"github.com/trademind-ai/trademind/backend/internal/modules/mcpaudit"
)

func countAll(t *testing.T, svc *mcpaudit.Service, tenant int64) int64 {
	t.Helper()
	res, err := svc.List(context.Background(), tenant, mcpaudit.ListFilter{})
	if err != nil {
		t.Fatal(err)
	}
	return res.Total
}

func TestWriteAcceptsRejectionStatuses(t *testing.T) {
	svc := &mcpaudit.Service{DB: openTestDB(t)}
	write(t, svc, 1, "mcp:auth", mcpaudit.StatusAuthFailed)
	write(t, svc, 1, "mcp:auth", mcpaudit.StatusRateLimited)
	res, err := svc.List(context.Background(), 1, mcpaudit.ListFilter{Status: mcpaudit.StatusAuthFailed})
	if err != nil {
		t.Fatal(err)
	}
	if res.Total != 1 {
		t.Fatalf("auth_failed filter expected 1, got %d", res.Total)
	}
	res, err = svc.List(context.Background(), 1, mcpaudit.ListFilter{Status: mcpaudit.StatusRateLimited})
	if err != nil {
		t.Fatal(err)
	}
	if res.Total != 1 {
		t.Fatalf("rate_limited filter expected 1, got %d", res.Total)
	}
}

func TestWriteThrottledBoundsPerKeyVolume(t *testing.T) {
	svc := &mcpaudit.Service{DB: openTestDB(t)}
	opts := mcpaudit.WriteOpts{TenantID: 0, Tool: "openapi:auth", Status: mcpaudit.StatusAuthFailed}
	for i := 0; i < 5; i++ {
		if err := svc.WriteThrottled(context.Background(), "ip:10.0.0.1", opts); err != nil {
			t.Fatal(err)
		}
	}
	if n := countAll(t, svc, 0); n != 1 {
		t.Fatalf("same key within a minute must write once, got %d", n)
	}

	// A different key writes its own row.
	if err := svc.WriteThrottled(context.Background(), "ip:10.0.0.2", opts); err != nil {
		t.Fatal(err)
	}
	if n := countAll(t, svc, 0); n != 2 {
		t.Fatalf("distinct key must write, got %d rows", n)
	}

	// A different status under the same key writes too.
	limited := opts
	limited.Status = mcpaudit.StatusRateLimited
	if err := svc.WriteThrottled(context.Background(), "ip:10.0.0.1", limited); err != nil {
		t.Fatal(err)
	}
	if n := countAll(t, svc, 0); n != 3 {
		t.Fatalf("distinct status must write, got %d rows", n)
	}
}
