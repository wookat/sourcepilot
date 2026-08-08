package mcpaudit_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/trademind-ai/trademind/backend/internal/modules/admin"
	"github.com/trademind-ai/trademind/backend/internal/modules/mcpaudit"
	"github.com/trademind-ai/trademind/backend/internal/pkg/ctxkey"
	"github.com/trademind-ai/trademind/backend/internal/testing/postgrestest"
	"github.com/trademind-ai/trademind/backend/internal/testing/safeenv"
	"gorm.io/gorm"
)

// R184: write-action audit rows are least-exposure — only accounts holding
// settings.manage (admins, same axis as write-token governance) see them.
// operator / readonly accounts only see read-tool rows.

var testWriteTools = []string{"orders_add_tag", "procurement_mark_paid"}

func seedAdminUser(t *testing.T, db *gorm.DB, role string) uuid.UUID {
	t.Helper()
	if err := db.AutoMigrate(&admin.AdminUser{}); err != nil {
		t.Fatal(err)
	}
	row := admin.AdminUser{Username: role + "-" + uuid.NewString()[:8], Role: role, Status: "active", TenantID: 1}
	if err := db.Create(&row).Error; err != nil {
		t.Fatal(err)
	}
	return row.ID
}

func writeRow(t *testing.T, svc *mcpaudit.Service, tool, mode string) {
	t.Helper()
	if err := svc.Write(context.Background(), mcpaudit.WriteOpts{
		TenantID:      1,
		TokenID:       uuid.New(),
		TokenName:     "tok",
		TokenMasked:   "sp_mcp_ro_ab12…cd34",
		Tool:          tool,
		Status:        mcpaudit.StatusSuccess,
		Mode:          mode,
		ParamsSummary: "orderNo=SO-1",
		ConfirmHash:   "abc123",
	}); err != nil {
		t.Fatal(err)
	}
}

func getAuditList(t *testing.T, db *gorm.DB, adminID uuid.UUID, query string) *httptest.ResponseRecorder {
	t.Helper()
	gin.SetMode(gin.TestMode)
	h := &mcpaudit.Handler{Svc: &mcpaudit.Service{DB: db}, WriteTools: testWriteTools}
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodGet, "/api/v1/mcp/audit-logs"+query, nil)
	c.Set(ctxkey.TenantID, int64(1))
	c.Set(ctxkey.AdminID, adminID.String())
	h.List(c)
	return w
}

func listedTools(t *testing.T, w *httptest.ResponseRecorder) []string {
	t.Helper()
	var body struct {
		Data struct {
			Total int64 `json:"total"`
			Items []struct {
				Tool string `json:"tool"`
				Mode string `json:"mode"`
			} `json:"items"`
		} `json:"data"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatalf("bad body %s: %v", w.Body.String(), err)
	}
	tools := make([]string, 0, len(body.Data.Items))
	for _, it := range body.Data.Items {
		tools = append(tools, it.Tool)
	}
	return tools
}

// TestAuditListWriteRowsAdminOnlyPostgres runs the same regression on a real
// PostgreSQL database (Docker compose) and additionally verifies tenant
// isolation: another tenant's rows never appear regardless of role.
func TestAuditListWriteRowsAdminOnlyPostgres(t *testing.T) {
	if _, ok, _ := safeenv.TestDatabaseURLFromEnv(); !ok {
		t.Skip("TEST_DATABASE_URL is not set; skipping PostgreSQL audit visibility regression")
	}
	h := postgrestest.Require(t)
	db := h.DB
	if err := db.AutoMigrate(&mcpaudit.ToolCallLog{}, &admin.AdminUser{}); err != nil {
		t.Fatal(err)
	}
	svc := &mcpaudit.Service{DB: db}
	writeRow(t, svc, "orders_query", "")
	writeRow(t, svc, "orders_add_tag", mcpaudit.ModeDryRun)
	writeRow(t, svc, "procurement_mark_paid", mcpaudit.ModeExecute)
	// Tenant 2 rows must never leak into tenant 1's list.
	if err := svc.Write(context.Background(), mcpaudit.WriteOpts{
		TenantID: 2, TokenID: uuid.New(), TokenName: "tok2", TokenMasked: "sp_mcp_ro_zz99…yy88",
		Tool: "procurement_mark_paid", Status: mcpaudit.StatusSuccess, Mode: mcpaudit.ModeExecute,
	}); err != nil {
		t.Fatal(err)
	}

	adminID := seedAdminUser(t, db, "admin")
	w := getAuditList(t, db, adminID, "")
	if w.Code != http.StatusOK {
		t.Fatalf("admin list: status = %d (body %s)", w.Code, w.Body.String())
	}
	if got := listedTools(t, w); len(got) != 3 {
		t.Fatalf("tenant-1 admin should see exactly its 3 rows, got %v", got)
	}

	for _, role := range []string{"operator", "readonly"} {
		id := seedAdminUser(t, db, role)
		w := getAuditList(t, db, id, "")
		if w.Code != http.StatusOK {
			t.Fatalf("%s list: status = %d (body %s)", role, w.Code, w.Body.String())
		}
		if got := listedTools(t, w); len(got) != 1 || got[0] != "orders_query" {
			t.Fatalf("%s should only see the read-tool row, got %v", role, got)
		}
	}
}

func TestAuditListWriteRowsAdminOnly(t *testing.T) {
	db := openTestDB(t)
	svc := &mcpaudit.Service{DB: db}
	writeRow(t, svc, "orders_query", "")                    // read tool row
	writeRow(t, svc, "orders_add_tag", mcpaudit.ModeDryRun) // write pipeline rows
	writeRow(t, svc, "procurement_mark_paid", mcpaudit.ModeExecute)

	adminID := seedAdminUser(t, db, "admin")
	w := getAuditList(t, db, adminID, "")
	if w.Code != http.StatusOK {
		t.Fatalf("admin list: status = %d (body %s)", w.Code, w.Body.String())
	}
	if got := listedTools(t, w); len(got) != 3 {
		t.Fatalf("admin should see all 3 rows, got %v", got)
	}

	for _, role := range []string{"operator", "readonly"} {
		id := seedAdminUser(t, db, role)
		w := getAuditList(t, db, id, "")
		if w.Code != http.StatusOK {
			t.Fatalf("%s list: status = %d (body %s)", role, w.Code, w.Body.String())
		}
		got := listedTools(t, w)
		if len(got) != 1 || got[0] != "orders_query" {
			t.Fatalf("%s should only see the read-tool row, got %v", role, got)
		}
		// Requesting write modes explicitly must not leak write rows either.
		for _, mode := range []string{mcpaudit.ModeDryRun, mcpaudit.ModeExecute} {
			w := getAuditList(t, db, id, "?mode="+mode)
			if w.Code != http.StatusOK {
				t.Fatalf("%s mode=%s: status = %d", role, mode, w.Code)
			}
			if got := listedTools(t, w); len(got) != 0 {
				t.Fatalf("%s mode=%s leaked write rows: %v", role, mode, got)
			}
		}
	}
}
