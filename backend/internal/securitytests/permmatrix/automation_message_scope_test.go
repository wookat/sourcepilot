package permmatrix

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"github.com/trademind-ai/trademind/backend/internal/modules/customerchat"
	"github.com/trademind-ai/trademind/backend/internal/modules/order"
)

// r125Cleanup removes rows seeded by this file so reruns stay deterministic.
func r125Cleanup(t *testing.T, h *harness) {
	t.Helper()
	require.NoError(t, h.DB.Exec("DELETE FROM order_automation_logs WHERE order_no LIKE 'perm-matrix-r125-%'").Error)
	require.NoError(t, h.DB.Exec("DELETE FROM order_automation_rules WHERE name LIKE 'perm-matrix-r125-%'").Error)
	require.NoError(t, h.DB.Exec("DELETE FROM order_review_rules WHERE name LIKE 'perm-matrix-r125-%'").Error)
	require.NoError(t, h.DB.Exec("DELETE FROM buyer_message_drafts WHERE order_no LIKE 'perm-matrix-r125-%'").Error)
	require.NoError(t, h.DB.Exec("DELETE FROM buyer_message_rules WHERE name LIKE 'perm-matrix-r125-%'").Error)
	require.NoError(t, h.DB.Exec("DELETE FROM customer_reply_templates WHERE name LIKE 'perm-matrix-r125-%'").Error)
	require.NoError(t, h.DB.Exec("DELETE FROM orders WHERE order_no LIKE 'perm-matrix-r125-%'").Error)
}

func r125SeedOrder(t *testing.T, h *harness, shopID uuid.UUID, tag string) *order.Order {
	t.Helper()
	sid := shopID
	o := &order.Order{
		TenantID:          tenantA,
		Platform:          "manual",
		ShopID:            &sid,
		OrderNo:           "perm-matrix-r125-" + tag + "-" + uuid.NewString()[:8],
		CustomerName:      "perm-matrix-r125",
		Status:            "pending",
		PaymentStatus:     "unpaid",
		FulfillmentStatus: "unfulfilled",
		Currency:          "CNY",
		TotalAmount:       66,
	}
	require.NoError(t, h.DB.Create(o).Error)
	return o
}

// TestAutomationRuleShopIDsScope is R125 audit evidence: rule payloads that
// pin shopIds must not accept cross-tenant or operator-ungranted shops.
// Cross-tenant / unknown shops resolve 404 (no existence leak); a shop the
// operator can only view resolves 403.
func TestAutomationRuleShopIDsScope(t *testing.T) {
	h := sharedHarness(t)
	r125Cleanup(t, h)

	shopB, err := h.seedShop(tenantB, "perm-matrix-r125-tenant-b")
	require.NoError(t, err)

	ruleBody := func(shopID uuid.UUID) string {
		return fmt.Sprintf(`{"name":"perm-matrix-r125-auto","triggerEvent":"order_paid","action":"mark_printed","shopIds":[%q]}`, shopID)
	}
	reviewBody := func(shopID uuid.UUID) string {
		return fmt.Sprintf(`{"name":"perm-matrix-r125-review","action":"review","shopIds":[%q]}`, shopID)
	}

	cases := []struct {
		name    string
		persona string
		shop    uuid.UUID
		want    int
	}{
		{"operator granted shop allowed", personaOperator, h.ShopGranted, http.StatusOK},
		{"operator ungranted shop hidden", personaOperator, h.ShopUngranted, http.StatusNotFound},
		{"operator cross-tenant shop hidden", personaOperator, shopB, http.StatusNotFound},
		{"admin cross-tenant shop hidden", personaAdmin, shopB, http.StatusNotFound},
		{"admin unknown shop hidden", personaAdmin, uuid.New(), http.StatusNotFound},
	}
	for _, tc := range cases {
		tok := h.Personas[tc.persona].Token
		w := h.doBody(t, http.MethodPost, "/api/v1/order-automation-rules", tok, ruleBody(tc.shop))
		require.Equalf(t, tc.want, w.Code, "automation rule create %s: %s", tc.name, w.Body.String())
		w = h.doBody(t, http.MethodPost, "/api/v1/order-review-rules", tok, reviewBody(tc.shop))
		require.Equalf(t, tc.want, w.Code, "review rule create %s: %s", tc.name, w.Body.String())
	}
	r125Cleanup(t, h)
}

// TestAutomationDryRunStoreScope: dry-run must not leak orders outside the
// operator's store grants (R125 finding: dry-run scanned all tenant orders).
func TestAutomationDryRunStoreScope(t *testing.T) {
	h := sharedHarness(t)
	r125Cleanup(t, h)

	granted := r125SeedOrder(t, h, h.ShopGranted, "granted")
	hidden := r125SeedOrder(t, h, h.ShopUngranted, "hidden")

	probes := []struct {
		path, body string
	}{
		{"/api/v1/order-automation-rules/dry-run",
			`{"name":"perm-matrix-r125-dry","triggerEvent":"order_created","action":"mark_printed"}`},
		{"/api/v1/order-review-rules/dry-run",
			`{"name":"perm-matrix-r125-dry","action":"review","minAmount":1}`},
	}
	for _, p := range probes {
		w := h.doBody(t, http.MethodPost, p.path, h.Personas[personaOperator].Token, p.body)
		require.Equalf(t, http.StatusOK, w.Code, "%s: %s", p.path, w.Body.String())
		body := w.Body.String()
		require.NotContainsf(t, body, hidden.OrderNo,
			"%s must not leak ungranted-shop orders", p.path)
		require.Containsf(t, body, granted.OrderNo,
			"%s should still evaluate granted-shop orders", p.path)
	}
	r125Cleanup(t, h)
}

// TestAutomationLogStoreScope: execution log list, order trail and retry must
// stay tenant- and store-scoped (404 for out-of-scope, no side effects).
func TestAutomationLogStoreScope(t *testing.T) {
	h := sharedHarness(t)
	r125Cleanup(t, h)

	granted := r125SeedOrder(t, h, h.ShopGranted, "granted")
	hidden := r125SeedOrder(t, h, h.ShopUngranted, "hidden")

	rule := &order.OrderAutomationRule{
		TenantID: tenantA, Name: "perm-matrix-r125-rule",
		TriggerEvent: order.AutomationEventOrderPaid,
		Action:       order.AutomationActionMarkPrinted, Enabled: true,
	}
	require.NoError(t, h.DB.Create(rule).Error)
	mkLog := func(o *order.Order) *order.OrderAutomationLog {
		l := &order.OrderAutomationLog{
			TenantID: tenantA, RuleID: rule.ID, RuleName: rule.Name,
			OrderID: o.ID, OrderNo: o.OrderNo,
			TriggerEvent: rule.TriggerEvent, Action: rule.Action,
			Status: order.AutomationLogFailed, Reason: "perm-matrix-r125",
			DedupKey: fmt.Sprintf("%d:%s:%s:%s", tenantA, rule.ID, o.ID, rule.TriggerEvent),
		}
		require.NoError(t, h.DB.Create(l).Error)
		return l
	}
	grantedLog := mkLog(granted)
	hiddenLog := mkLog(hidden)

	opTok := h.Personas[personaOperator].Token

	// List: operator must not see logs of orders in ungranted shops.
	w := h.doBody(t, http.MethodGet, "/api/v1/order-automation-logs?pageSize=200", opTok, "")
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	body := w.Body.String()
	require.Contains(t, body, grantedLog.ID.String())
	require.NotContains(t, body, hiddenLog.ID.String(),
		"automation log list must hide logs of ungranted-shop orders")

	// Order trail: out-of-scope order resolves 404.
	w = h.doBody(t, http.MethodGet, "/api/v1/orders/"+hidden.ID.String()+"/automation-logs", opTok, "")
	require.Equal(t, http.StatusNotFound, w.Code, w.Body.String())

	// Retry: out-of-scope log resolves 404 and the log stays untouched.
	for _, pk := range []string{personaOperator, personaCrossTenant} {
		w = h.doBody(t, http.MethodPost,
			"/api/v1/order-automation-logs/"+hiddenLog.ID.String()+"/retry",
			h.Personas[pk].Token, "{}")
		require.Equalf(t, http.StatusNotFound, w.Code, "retry [%s]: %s", pk, w.Body.String())
	}
	var attempts int
	require.NoError(t, h.DB.Model(&order.OrderAutomationLog{}).
		Where("id = ?", hiddenLog.ID).Pluck("attempts", &attempts).Error)
	require.Equal(t, 1, attempts, "denied retry must not execute the action")

	r125Cleanup(t, h)
}

// TestBuyerMsgScope covers R125 buyer messaging surfaces: rule shopIds must
// stay within the caller's grants and drafts must be invisible (404, no
// content leak) outside tenant + store scope.
func TestBuyerMsgScope(t *testing.T) {
	h := sharedHarness(t)
	r125Cleanup(t, h)

	shopB, err := h.seedShop(tenantB, "perm-matrix-r125-tenant-b-msg")
	require.NoError(t, err)

	tpl := &customerchat.CustomerReplyTemplate{
		TenantID: tenantA, GroupKey: "shipping", Name: "perm-matrix-r125-tpl",
		Content: "hello {订单号}", Enabled: true,
	}
	require.NoError(t, h.DB.Create(tpl).Error)

	opTok := h.Personas[personaOperator].Token
	ruleBody := func(shopID uuid.UUID) string {
		return fmt.Sprintf(`{"name":"perm-matrix-r125-msg","node":"paid","templateId":%q,"shopIds":[%q]}`, tpl.ID, shopID)
	}
	for _, tc := range []struct {
		name string
		shop uuid.UUID
		want int
	}{
		{"granted", h.ShopGranted, http.StatusOK},
		{"ungranted", h.ShopUngranted, http.StatusNotFound},
		{"cross-tenant", shopB, http.StatusNotFound},
	} {
		w := h.doBody(t, http.MethodPost, "/api/v1/customer/buyer-message-rules", opTok, ruleBody(tc.shop))
		require.Equalf(t, tc.want, w.Code, "buyer msg rule create %s: %s", tc.name, w.Body.String())
	}

	// Seed one pending draft per shop; secret marks the content leak check.
	secret := "perm-matrix-r125-secret-" + uuid.NewString()[:8]
	mkDraft := func(o *order.Order, content string) *customerchat.BuyerMessageDraft {
		d := &customerchat.BuyerMessageDraft{
			TenantID: tenantA, OrderID: o.ID, Node: "paid",
			RuleID: uuid.New(), TemplateID: tpl.ID, TemplateName: tpl.Name,
			Platform: "manual", ShopID: o.ShopID, OrderNo: o.OrderNo,
			CustomerName: "perm-matrix-r125", Content: content,
			Status: customerchat.BuyerMsgDraftPending,
		}
		require.NoError(t, h.DB.Create(d).Error)
		return d
	}
	granted := r125SeedOrder(t, h, h.ShopGranted, "msg-granted")
	hidden := r125SeedOrder(t, h, h.ShopUngranted, "msg-hidden")
	grantedDraft := mkDraft(granted, "granted content")
	hiddenDraft := mkDraft(hidden, secret)

	// List: drafts of ungranted shops must be invisible, content included.
	w := h.doBody(t, http.MethodGet, "/api/v1/customer/buyer-messages/drafts?pageSize=200", opTok, "")
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	require.Contains(t, w.Body.String(), grantedDraft.ID.String())
	require.NotContains(t, w.Body.String(), hiddenDraft.ID.String(),
		"draft list must hide ungranted-shop drafts")
	require.NotContains(t, w.Body.String(), secret, "draft content must not leak")

	// Detail writes: 404 for out-of-scope drafts, no state change.
	probes := []struct {
		method, path, body string
	}{
		{http.MethodPut, "/api/v1/customer/buyer-messages/drafts/%s", `{"content":"x"}`},
		{http.MethodPost, "/api/v1/customer/buyer-messages/drafts/%s/mark-sent", "{}"},
		{http.MethodPost, "/api/v1/customer/buyer-messages/drafts/%s/ignore", "{}"},
	}
	for _, p := range probes {
		for _, pk := range []string{personaOperator, personaCrossTenant} {
			w := h.doBody(t, p.method, fmt.Sprintf(p.path, hiddenDraft.ID), h.Personas[pk].Token, p.body)
			require.Equalf(t, http.StatusNotFound, w.Code,
				"%s %s [%s]: %s", p.method, p.path, pk, w.Body.String())
			require.NotContains(t, w.Body.String(), secret)
		}
	}

	// Batch mark-sent must skip out-of-scope drafts.
	w = h.doBody(t, http.MethodPost, "/api/v1/customer/buyer-messages/drafts/batch-mark-sent",
		opTok, fmt.Sprintf(`{"ids":[%q]}`, hiddenDraft.ID))
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())

	var status string
	require.NoError(t, h.DB.Model(&customerchat.BuyerMessageDraft{}).
		Where("id = ?", hiddenDraft.ID).Pluck("status", &status).Error)
	require.Equal(t, customerchat.BuyerMsgDraftPending, status,
		"denied probes must not mutate the out-of-scope draft")

	r125Cleanup(t, h)
}

// TestSelectionCandidateTenantScope: the R119+ selection read routes must
// resolve cross-tenant candidates as 404.
func TestSelectionCandidateTenantScope(t *testing.T) {
	h := sharedHarness(t)
	fake := uuid.New()
	tok := h.Personas[personaAdmin].Token
	for _, path := range []string{
		"/api/v1/selection/candidates/" + fake.String() + "/insights",
		"/api/v1/selection/candidates/" + fake.String() + "/price-trend",
		"/api/v1/selection/compare?ids=" + fake.String() + "," + uuid.NewString(),
	} {
		w := h.doBody(t, http.MethodGet, path, tok, "")
		require.Truef(t, w.Code == http.StatusNotFound || w.Code == http.StatusBadRequest,
			"%s: expected 404/400 for unknown candidate, got %d: %s", path, w.Code, w.Body.String())
		var resp struct {
			Data json.RawMessage `json:"data"`
		}
		require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
		require.False(t, strings.Contains(string(resp.Data), fake.String()) && w.Code == http.StatusOK)
	}
}
