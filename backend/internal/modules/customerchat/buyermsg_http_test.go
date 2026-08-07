package customerchat

import (
	"encoding/json"
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

type buyerMsgHTTPFixture struct {
	db      *gorm.DB
	router  *gin.Engine
	adminT1 uuid.UUID
	adminT2 uuid.UUID
	roT1    uuid.UUID
	tplID   uuid.UUID
	ruleID  uuid.UUID
	draftID uuid.UUID
}

func setupBuyerMsgHTTPFixture(t *testing.T) *buyerMsgHTTPFixture {
	t.Helper()
	dsn := filepath.Join(t.TempDir(), "buyermsg-http.db")
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Skipf("sqlite unavailable: %v", err)
	}
	if err := db.AutoMigrate(
		&admin.AdminUser{}, &admin.UserStorePermission{},
		&CustomerConversation{}, &CustomerMessage{}, &CustomerReplyTemplate{}, &CustomerReplyTemplateVariant{},
		&BuyerMessageRule{}, &BuyerMessageDraft{},
	); err != nil {
		t.Fatal(err)
	}

	f := &buyerMsgHTTPFixture{db: db}
	f.adminT1 = seedScopeUser(t, db, "admin")
	f.adminT2 = seedScopeUser(t, db, "admin")

	roID := uuid.New()
	if err := db.Create(&admin.AdminUser{
		Base:         model.Base{ID: roID},
		Username:     "ro-" + roID.String()[:8],
		Email:        "ro-" + roID.String()[:8] + "@example.com",
		PasswordHash: "x",
		Role:         "readonly",
		Status:       "active",
	}).Error; err != nil {
		t.Fatalf("seed readonly user: %v", err)
	}
	f.roT1 = roID

	tpl := &CustomerReplyTemplate{TenantID: 1, GroupKey: TemplateGroupLogistics, Name: "物流通知", Content: "您好{买家昵称}", Enabled: true}
	if err := db.Create(tpl).Error; err != nil {
		t.Fatal(err)
	}
	f.tplID = tpl.ID

	rule := &BuyerMessageRule{TenantID: 1, Name: "发货通知", Node: BuyerMsgNodeShipped, TemplateID: tpl.ID, Enabled: true}
	if err := db.Create(rule).Error; err != nil {
		t.Fatal(err)
	}
	f.ruleID = rule.ID

	draft := &BuyerMessageDraft{
		TenantID: 1, OrderID: uuid.New(), Node: BuyerMsgNodeShipped,
		RuleID: rule.ID, TemplateID: tpl.ID, TemplateName: tpl.Name,
		Platform: "manual", OrderNo: "SO-1", CustomerName: "买家A",
		Content: "您好买家A", Status: BuyerMsgDraftPending,
	}
	if err := db.Create(draft).Error; err != nil {
		t.Fatal(err)
	}
	f.draftID = draft.ID

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

func (f *buyerMsgHTTPFixture) do(t *testing.T, method, path string, adminID uuid.UUID, tenant string, body string) *httptest.ResponseRecorder {
	t.Helper()
	w := httptest.NewRecorder()
	req := httptest.NewRequest(method, path, strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Test-Admin", adminID.String())
	req.Header.Set("X-Test-Tenant", tenant)
	f.router.ServeHTTP(w, req)
	return w
}

// 三角色：可写 admin 全链路可用；readonly 读 200 / 写 403；跨租户一律 404。
func TestBuyerMsgRoutesRolesAndTenantIsolation(t *testing.T) {
	f := setupBuyerMsgHTTPFixture(t)
	rid := f.ruleID.String()
	did := f.draftID.String()

	// 可写 admin：列表读取
	for _, p := range []string{
		"/api/v1/customer/buyer-message-rules",
		"/api/v1/customer/buyer-messages/drafts",
	} {
		w := f.do(t, http.MethodGet, p, f.adminT1, "1", "")
		if w.Code != http.StatusOK {
			t.Fatalf("admin GET %s: got %d (%s)", p, w.Code, w.Body.String())
		}
		var envelope struct {
			Data struct {
				CanWrite bool `json:"canWrite"`
			} `json:"data"`
		}
		if err := json.Unmarshal(w.Body.Bytes(), &envelope); err != nil {
			t.Fatal(err)
		}
		if !envelope.Data.CanWrite {
			t.Fatalf("admin GET %s: canWrite should be true", p)
		}
	}

	// 可写 admin：完整写链路 创建规则 → 编辑草稿 → 标记已发送
	w := f.do(t, http.MethodPost, "/api/v1/customer/buyer-message-rules", f.adminT1, "1",
		`{"name":"签收关怀","node":"delivered","templateId":"`+f.tplID.String()+`"}`)
	if w.Code != http.StatusOK {
		t.Fatalf("admin create rule: got %d (%s)", w.Code, w.Body.String())
	}
	w = f.do(t, http.MethodPut, "/api/v1/customer/buyer-messages/drafts/"+did, f.adminT1, "1", `{"content":"改后内容"}`)
	if w.Code != http.StatusOK {
		t.Fatalf("admin edit draft: got %d (%s)", w.Code, w.Body.String())
	}
	w = f.do(t, http.MethodPost, "/api/v1/customer/buyer-messages/drafts/"+did+"/mark-sent", f.adminT1, "1", `{}`)
	if w.Code != http.StatusOK {
		t.Fatalf("admin mark sent: got %d (%s)", w.Code, w.Body.String())
	}

	// readonly：读 200（canWrite=false），写 403
	w = f.do(t, http.MethodGet, "/api/v1/customer/buyer-messages/drafts", f.roT1, "1", "")
	if w.Code != http.StatusOK {
		t.Fatalf("readonly GET drafts: got %d (%s)", w.Code, w.Body.String())
	}
	if !strings.Contains(w.Body.String(), `"canWrite":false`) {
		t.Fatalf("readonly GET drafts: canWrite should be false (%s)", w.Body.String())
	}
	writeCases := []struct{ method, path, body string }{
		{http.MethodPost, "/api/v1/customer/buyer-message-rules", `{"name":"x","node":"paid","templateId":"` + f.tplID.String() + `"}`},
		{http.MethodPut, "/api/v1/customer/buyer-message-rules/" + rid, `{"enabled":false}`},
		{http.MethodDelete, "/api/v1/customer/buyer-message-rules/" + rid, ""},
		{http.MethodPost, "/api/v1/customer/buyer-messages/generate", `{}`},
		{http.MethodPut, "/api/v1/customer/buyer-messages/drafts/" + did, `{"content":"x"}`},
		{http.MethodPost, "/api/v1/customer/buyer-messages/drafts/" + did + "/mark-sent", `{}`},
		{http.MethodPost, "/api/v1/customer/buyer-messages/drafts/" + did + "/ignore", `{}`},
		{http.MethodPost, "/api/v1/customer/buyer-messages/drafts/batch-mark-sent", `{"ids":["` + did + `"]}`},
	}
	for _, tc := range writeCases {
		if w := f.do(t, tc.method, tc.path, f.roT1, "1", tc.body); w.Code != http.StatusForbidden {
			t.Fatalf("readonly %s %s: got %d want 403 (%s)", tc.method, tc.path, w.Code, w.Body.String())
		}
	}

	// 跨租户：对象级操作一律 404，不泄露存在性
	crossCases := []struct{ method, path, body string }{
		{http.MethodPut, "/api/v1/customer/buyer-message-rules/" + rid, `{"enabled":false}`},
		{http.MethodDelete, "/api/v1/customer/buyer-message-rules/" + rid, ""},
		{http.MethodPut, "/api/v1/customer/buyer-messages/drafts/" + did, `{"content":"x"}`},
		{http.MethodPost, "/api/v1/customer/buyer-messages/drafts/" + did + "/mark-sent", `{}`},
		{http.MethodPost, "/api/v1/customer/buyer-messages/drafts/" + did + "/ignore", `{}`},
	}
	for _, tc := range crossCases {
		if w := f.do(t, tc.method, tc.path, f.adminT2, "2", tc.body); w.Code != http.StatusNotFound {
			t.Fatalf("cross-tenant %s %s: got %d want 404 (%s)", tc.method, tc.path, w.Code, w.Body.String())
		}
	}

	// 跨租户列表：不返回他租户数据
	w = f.do(t, http.MethodGet, "/api/v1/customer/buyer-messages/drafts", f.adminT2, "2", "")
	if w.Code != http.StatusOK {
		t.Fatalf("cross-tenant GET drafts: got %d", w.Code)
	}
	if strings.Contains(w.Body.String(), "SO-1") {
		t.Fatalf("cross-tenant GET drafts leaked data: %s", w.Body.String())
	}
}
