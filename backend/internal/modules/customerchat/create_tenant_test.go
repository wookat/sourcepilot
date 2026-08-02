package customerchat

import (
	"net/http/httptest"
	"path/filepath"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"gorm.io/gorm"

	"github.com/trademind-ai/trademind/backend/internal/pkg/ctxkey"
)

// Regression: conversations created from the UI must carry the caller's
// tenant id; they were previously persisted with tenant_id=0 and thus
// invisible to the tenant-scoped list query.
func TestCreateConversationSetsTenantID(t *testing.T) {
	dsn := filepath.Join(t.TempDir(), "tenant.db")
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Skipf("sqlite unavailable: %v", err)
	}
	if err := db.AutoMigrate(&CustomerConversation{}); err != nil {
		t.Fatal(err)
	}

	gin.SetMode(gin.TestMode)
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Request = httptest.NewRequest("POST", "/api/v1/customer/conversations", nil)
	c.Set(ctxkey.TenantID, int64(7))

	svc := &Service{DB: db}
	row, err := svc.CreateConversation(c, CreateConversationBody{CustomerName: "e2e-客户"}, nil)
	if err != nil {
		t.Fatalf("CreateConversation: %v", err)
	}
	if row.TenantID != 7 {
		t.Fatalf("tenant id: got %d, want 7", row.TenantID)
	}

	var stored CustomerConversation
	if err := db.First(&stored, "id = ?", row.ID).Error; err != nil {
		t.Fatal(err)
	}
	if stored.TenantID != 7 {
		t.Fatalf("stored tenant id: got %d, want 7", stored.TenantID)
	}
}
