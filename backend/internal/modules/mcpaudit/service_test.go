package mcpaudit_test

import (
	"context"
	"fmt"
	"testing"

	"github.com/glebarez/sqlite"
	"github.com/google/uuid"
	"github.com/trademind-ai/trademind/backend/internal/modules/mcpaudit"
	"gorm.io/gorm"
)

func openTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	dsn := fmt.Sprintf("file:mcpaudit_%s?mode=memory&cache=shared&_pragma=busy_timeout(5000)", uuid.New().String())
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Skipf("sqlite unavailable: %v", err)
	}
	sqlDB, _ := db.DB()
	sqlDB.SetMaxOpenConns(1)
	if err := db.AutoMigrate(&mcpaudit.ToolCallLog{}); err != nil {
		t.Fatal(err)
	}
	return db
}

func write(t *testing.T, svc *mcpaudit.Service, tenant int64, tool, status string) {
	t.Helper()
	if err := svc.Write(context.Background(), mcpaudit.WriteOpts{
		TenantID:    tenant,
		TokenID:     uuid.New(),
		TokenName:   "tok",
		TokenMasked: "sp_mcp_ro_ab12…cd34",
		Tool:        tool,
		Status:      status,
		DurationMs:  3,
	}); err != nil {
		t.Fatal(err)
	}
}

func TestWriteAndListTenantScoped(t *testing.T) {
	svc := &mcpaudit.Service{DB: openTestDB(t)}
	write(t, svc, 1, "orders_query", mcpaudit.StatusSuccess)
	write(t, svc, 1, "inventory_query", mcpaudit.StatusError)
	write(t, svc, 2, "orders_query", mcpaudit.StatusSuccess)

	res, err := svc.List(context.Background(), 1, mcpaudit.ListFilter{})
	if err != nil {
		t.Fatal(err)
	}
	if res.Total != 2 || len(res.Items) != 2 {
		t.Fatalf("tenant 1 expected 2 rows, got total=%d items=%d", res.Total, len(res.Items))
	}
	for _, r := range res.Items {
		if r.TenantID != 1 {
			t.Fatalf("cross-tenant leak: %+v", r)
		}
	}
}

func TestListFilters(t *testing.T) {
	svc := &mcpaudit.Service{DB: openTestDB(t)}
	write(t, svc, 1, "orders_query", mcpaudit.StatusSuccess)
	write(t, svc, 1, "orders_query", mcpaudit.StatusError)
	write(t, svc, 1, "report_summary", mcpaudit.StatusSuccess)

	byTool, err := svc.List(context.Background(), 1, mcpaudit.ListFilter{Tool: "orders_query"})
	if err != nil {
		t.Fatal(err)
	}
	if byTool.Total != 2 {
		t.Fatalf("tool filter expected 2, got %d", byTool.Total)
	}
	byStatus, err := svc.List(context.Background(), 1, mcpaudit.ListFilter{Status: mcpaudit.StatusError})
	if err != nil {
		t.Fatal(err)
	}
	if byStatus.Total != 1 || byStatus.Items[0].Tool != "orders_query" {
		t.Fatalf("status filter mismatch: %+v", byStatus)
	}
}

func TestListFilterByMode(t *testing.T) {
	svc := &mcpaudit.Service{DB: openTestDB(t)}
	for _, mode := range []string{mcpaudit.ModeDryRun, mcpaudit.ModeExecute, ""} {
		if err := svc.Write(context.Background(), mcpaudit.WriteOpts{
			TenantID: 1, TokenID: uuid.New(), Tool: "orders_add_tag",
			Status: mcpaudit.StatusSuccess, Mode: mode,
		}); err != nil {
			t.Fatal(err)
		}
	}
	byMode, err := svc.List(context.Background(), 1, mcpaudit.ListFilter{Mode: mcpaudit.ModeExecute})
	if err != nil {
		t.Fatal(err)
	}
	if byMode.Total != 1 || byMode.Items[0].Mode != mcpaudit.ModeExecute {
		t.Fatalf("mode filter mismatch: %+v", byMode)
	}
}

func TestWriteNormalizesUnknownStatus(t *testing.T) {
	svc := &mcpaudit.Service{DB: openTestDB(t)}
	write(t, svc, 1, "orders_query", "weird")
	res, err := svc.List(context.Background(), 1, mcpaudit.ListFilter{})
	if err != nil {
		t.Fatal(err)
	}
	if res.Items[0].Status != mcpaudit.StatusError {
		t.Fatalf("unknown status must normalize to error, got %q", res.Items[0].Status)
	}
}
