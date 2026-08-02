package orderexception

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"github.com/google/uuid"
	"github.com/trademind-ai/trademind/backend/internal/modules/order"
	"gorm.io/gorm"
)

func openMarkNotFoundTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	dsn := fmt.Sprintf("file:marknotfound_%s?mode=memory&cache=shared&_pragma=busy_timeout(5000)", uuid.New().String())
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Skipf("sqlite unavailable: %v", err)
	}
	sqlDB, err := db.DB()
	if err != nil {
		t.Fatal(err)
	}
	sqlDB.SetMaxOpenConns(1)
	if err := db.AutoMigrate(
		&order.Order{},
		&order.OrderItem{},
		&OrderExceptionMark{},
	); err != nil {
		t.Fatal(err)
	}
	return db
}

// Marking a non-existent source must return a localized 404 envelope instead
// of the raw gorm "record not found" string.
func TestMarkHandleNotFoundLocalized(t *testing.T) {
	db := openMarkNotFoundTestDB(t)
	gin.SetMode(gin.TestMode)
	r := gin.New()
	h := &Handler{Svc: &Service{DB: db}}
	r.POST("/orders/exceptions/:sourceType/:sourceId/handle", h.Handle)
	r.POST("/orders/exceptions/:sourceType/:sourceId/ignore", h.Ignore)

	for _, route := range []string{"handle", "ignore"} {
		body := strings.NewReader(`{"exceptionType":"sku_unmatched"}`)
		req := httptest.NewRequest(http.MethodPost,
			"/orders/exceptions/order_item/"+uuid.NewString()+"/"+route, body)
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)

		if w.Code != http.StatusNotFound {
			t.Fatalf("%s: expected 404, got %d body=%s", route, w.Code, w.Body.String())
		}
		var env struct {
			Code    int    `json:"code"`
			Message string `json:"message"`
		}
		if err := json.Unmarshal(w.Body.Bytes(), &env); err != nil {
			t.Fatalf("%s: bad envelope: %v", route, err)
		}
		if strings.Contains(env.Message, "record not found") {
			t.Fatalf("%s: raw english error leaked: %q", route, env.Message)
		}
		if env.Message != "记录不存在或已被删除" {
			t.Fatalf("%s: expected localized message, got %q", route, env.Message)
		}
	}
}
