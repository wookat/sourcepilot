package taskcenter

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"github.com/google/uuid"
	"gorm.io/gorm"

	"github.com/trademind-ai/trademind/backend/internal/pkg/ctxkey"
)

// DELETE /task-center/alerts/:id/mark 必须走告警取消标记（租户 scoped），
// 不得误接失败任务的 Unmark（会因缺少 taskType 直接 400）。
func TestUnmarkAlertRouteIsTenantScoped(t *testing.T) {
	dsn := fmt.Sprintf("file:alerts_unmark_%s?mode=memory&cache=shared", uuid.NewString())
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Skipf("sqlite unavailable: %v", err)
	}
	if err := db.AutoMigrate(&TaskAlert{}, &TaskAlertNotification{}); err != nil {
		t.Fatal(err)
	}
	now := time.Now().UTC()
	own := TaskAlert{
		ID: uuid.New(), TenantID: 0, TaskType: TaskTypeCollect, SourceID: "src-0",
		FailureCategory: "network", Severity: "high", Title: "t",
		Status: TaskAlertStatusHandled, AlertCount: 1,
		FirstSeenAt: now, LastSeenAt: now, CreatedAt: now, UpdatedAt: now,
	}
	other := own
	other.ID = uuid.New()
	other.TenantID = 2
	other.SourceID = "src-2"
	if err := db.Create(&own).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Create(&other).Error; err != nil {
		t.Fatal(err)
	}

	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(func(c *gin.Context) {
		c.Set(ctxkey.TenantID, int64(0))
		c.Next()
	})
	Register(r.Group("/api/v1"), &Handler{Svc: &Service{DB: db}})

	do := func(id uuid.UUID) int {
		w := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodDelete, "/api/v1/task-center/alerts/"+id.String()+"/mark", nil)
		r.ServeHTTP(w, req)
		return w.Code
	}

	if code := do(own.ID); code != http.StatusOK {
		t.Fatalf("own alert unmark should succeed, got %d", code)
	}
	var got TaskAlert
	if err := db.First(&got, "id = ?", own.ID).Error; err != nil {
		t.Fatal(err)
	}
	if got.Status != TaskAlertStatusOpen {
		t.Fatalf("alert should return to open, got %s", got.Status)
	}
	if code := do(other.ID); code != http.StatusNotFound {
		t.Fatalf("cross-tenant alert unmark should 404, got %d", code)
	}
}
