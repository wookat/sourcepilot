package customerchat

import (
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"github.com/google/uuid"
	"gorm.io/gorm"

	"github.com/trademind-ai/trademind/backend/internal/modules/admin"
	"github.com/trademind-ai/trademind/backend/internal/pkg/ctxkey"
	"github.com/trademind-ai/trademind/backend/internal/pkg/model"
)

type scopeFixture struct {
	db      *gorm.DB
	router  *gin.Engine
	convID  uuid.UUID
	suggID  uuid.UUID
	shopID  uuid.UUID
	adminT1 uuid.UUID
	adminT2 uuid.UUID
	opOther uuid.UUID
	opShopA uuid.UUID
}

func seedScopeUser(t *testing.T, db *gorm.DB, role string) uuid.UUID {
	t.Helper()
	uid := uuid.New()
	if err := db.Create(&admin.AdminUser{
		Base:         model.Base{ID: uid},
		Username:     role + "-" + uid.String()[:8],
		Email:        role + "-" + uid.String()[:8] + "@example.com",
		PasswordHash: "x",
		Role:         role,
		Status:       "active",
	}).Error; err != nil {
		t.Fatalf("seed %s user: %v", role, err)
	}
	return uid
}

func setupScopeFixture(t *testing.T) *scopeFixture {
	t.Helper()
	dsn := filepath.Join(t.TempDir(), "scope.db")
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Skipf("sqlite unavailable: %v", err)
	}
	if err := db.AutoMigrate(
		&admin.AdminUser{}, &admin.UserStorePermission{},
		&CustomerConversation{}, &CustomerMessage{}, &CustomerReplySuggestion{}, &CustomerFailureEvent{},
	); err != nil {
		t.Fatal(err)
	}

	f := &scopeFixture{db: db, shopID: uuid.New()}
	conv := &CustomerConversation{TenantID: 1, ShopID: &f.shopID, CustomerName: "客户A", CustomerLanguage: "zh", Platform: "manual", Status: StatusOpen}
	if err := db.Create(conv).Error; err != nil {
		t.Fatal(err)
	}
	f.convID = conv.ID
	sugg := &CustomerReplySuggestion{ConversationID: conv.ID, Status: SuggestionGenerated, SuggestedReply: "hi"}
	if err := db.Create(sugg).Error; err != nil {
		t.Fatal(err)
	}
	f.suggID = sugg.ID

	f.adminT1 = seedScopeUser(t, db, "admin")
	f.adminT2 = seedScopeUser(t, db, "admin")
	f.opOther = seedScopeUser(t, db, "operator")
	f.opShopA = seedScopeUser(t, db, "operator")
	otherShop := uuid.New()
	if err := db.Create(&admin.UserStorePermission{UserID: f.opOther, StoreID: otherShop, PermissionScope: "operate"}).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Create(&admin.UserStorePermission{UserID: f.opShopA, StoreID: f.shopID, PermissionScope: "operate"}).Error; err != nil {
		t.Fatal(err)
	}

	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(func(c *gin.Context) {
		c.Set(ctxkey.AdminID, c.GetHeader("X-Test-Admin"))
		tid := int64(1)
		if c.GetHeader("X-Test-Tenant") == "2" {
			tid = 2
		}
		c.Set(ctxkey.TenantID, tid)
		c.Next()
	})
	Register(r.Group("/api/v1"), &Handler{Svc: &Service{DB: db}})
	f.router = r
	return f
}

func (f *scopeFixture) do(t *testing.T, method, path string, adminID uuid.UUID, tenant string, body string) *httptest.ResponseRecorder {
	t.Helper()
	w := httptest.NewRecorder()
	var rd *strings.Reader
	if body == "" {
		rd = strings.NewReader("")
	} else {
		rd = strings.NewReader(body)
	}
	req := httptest.NewRequest(method, path, rd)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Test-Admin", adminID.String())
	req.Header.Set("X-Test-Tenant", tenant)
	f.router.ServeHTTP(w, req)
	return w
}

// Regression (R70): conversation subresources must apply the same tenant +
// store scope as the detail endpoint. Unauthorized / cross-tenant access must
// return 404 (never 200 with empty data) to avoid leaking existence.
func TestConversationSubresourcesScopedByTenantAndStore(t *testing.T) {
	f := setupScopeFixture(t)
	cid := f.convID.String()
	sid := f.suggID.String()

	readPaths := []string{
		"/api/v1/customer/conversations/" + cid,
		"/api/v1/customer/conversations/" + cid + "/messages",
		"/api/v1/customer/conversations/" + cid + "/ai-suggestions",
	}

	// 角色1：同租户 admin —— 全部可读
	for _, p := range readPaths {
		if w := f.do(t, http.MethodGet, p, f.adminT1, "1", ""); w.Code != http.StatusOK {
			t.Fatalf("same-tenant admin GET %s: got %d want 200 (%s)", p, w.Code, w.Body.String())
		}
	}
	// 角色2：跨租户 admin —— 一律 404，不泄露存在性
	for _, p := range readPaths {
		if w := f.do(t, http.MethodGet, p, f.adminT2, "2", ""); w.Code != http.StatusNotFound {
			t.Fatalf("cross-tenant admin GET %s: got %d want 404 (%s)", p, w.Code, w.Body.String())
		}
	}
	// 角色3：operator 只授权其他店铺 —— 一律 404
	for _, p := range readPaths {
		if w := f.do(t, http.MethodGet, p, f.opOther, "1", ""); w.Code != http.StatusNotFound {
			t.Fatalf("foreign-shop operator GET %s: got %d want 404 (%s)", p, w.Code, w.Body.String())
		}
	}
	// operator 授权了会话所在店铺 —— 可读
	for _, p := range readPaths {
		if w := f.do(t, http.MethodGet, p, f.opShopA, "1", ""); w.Code != http.StatusOK {
			t.Fatalf("granted operator GET %s: got %d want 200 (%s)", p, w.Code, w.Body.String())
		}
	}

	// 写路径：越权/跨租户同样 404（先于业务校验，且不落库）
	writeCases := []struct{ method, path, body string }{
		{http.MethodPost, "/api/v1/customer/conversations/" + cid + "/messages", `{"role":"agent","content":"hi"}`},
		{http.MethodPost, "/api/v1/customer/conversations/" + cid + "/mark-replied", `{"reply":"done"}`},
		{http.MethodPut, "/api/v1/customer/conversations/" + cid, `{"customerName":"x"}`},
		{http.MethodDelete, "/api/v1/customer/conversations/" + cid, ""},
		{http.MethodPut, "/api/v1/customer/reply-suggestions/" + sid, `{"editedReply":"x"}`},
		{http.MethodPost, "/api/v1/customer/reply-suggestions/" + sid + "/accept", `{"finalReply":"x"}`},
		{http.MethodPost, "/api/v1/customer/reply-suggestions/" + sid + "/discard", `{}`},
		{http.MethodPost, "/api/v1/customer/ai-suggestions/" + sid + "/apply", `{"finalReply":"x"}`},
		{http.MethodPost, "/api/v1/customer/ai-suggestions/" + sid + "/reject", `{"reason":"x"}`},
	}
	for _, tc := range writeCases {
		if w := f.do(t, tc.method, tc.path, f.adminT2, "2", tc.body); w.Code != http.StatusNotFound {
			t.Fatalf("cross-tenant %s %s: got %d want 404 (%s)", tc.method, tc.path, w.Code, w.Body.String())
		}
		if w := f.do(t, tc.method, tc.path, f.opOther, "1", tc.body); w.Code != http.StatusNotFound {
			t.Fatalf("foreign-shop %s %s: got %d want 404 (%s)", tc.method, tc.path, w.Code, w.Body.String())
		}
	}

	// 越权写不得产生副作用
	var msgCount int64
	if err := f.db.Model(&CustomerMessage{}).Count(&msgCount).Error; err != nil {
		t.Fatal(err)
	}
	if msgCount != 0 {
		t.Fatalf("out-of-scope writes must not persist messages, got %d rows", msgCount)
	}
	var conv CustomerConversation
	if err := f.db.First(&conv, "id = ?", f.convID).Error; err != nil {
		t.Fatal(err)
	}
	if conv.CustomerName != "客户A" || conv.Status != StatusOpen {
		t.Fatalf("out-of-scope writes must not mutate conversation, got %+v", conv)
	}
}
