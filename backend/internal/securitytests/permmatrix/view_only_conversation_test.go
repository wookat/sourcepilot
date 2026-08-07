package permmatrix

import (
	"encoding/json"
	"fmt"
	"net/http"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/trademind-ai/trademind/backend/internal/modules/customerchat"
)

// TestViewOnlyPersonaConversationWriteScope is the R164 conversation tier of
// the view-only persona gate: an operator whose only grant on a store is
// scope "view" must be rejected with 403 (business code 40303) on every
// mutating customer-conversation / reply-suggestion route addressing that
// store, with zero mutation, while conversation reads keep working.
func TestViewOnlyPersonaConversationWriteScope(t *testing.T) {
	h := sharedHarness(t)
	cleanup := func() {
		require.NoError(t, h.DB.Exec(
			"DELETE FROM customer_reply_suggestions WHERE conversation_id IN (SELECT id FROM customer_conversations WHERE customer_name LIKE 'perm-matrix-r164-%')").Error)
		require.NoError(t, h.DB.Exec(
			"DELETE FROM customer_messages WHERE conversation_id IN (SELECT id FROM customer_conversations WHERE customer_name LIKE 'perm-matrix-r164-%')").Error)
		require.NoError(t, h.DB.Exec(
			"DELETE FROM customer_conversations WHERE customer_name LIKE 'perm-matrix-r164-%'").Error)
	}
	cleanup()
	t.Cleanup(cleanup)

	tok := h.Personas[personaViewOnly].Token
	sid := h.ShopViewOnly
	conv := &customerchat.CustomerConversation{
		TenantID: tenantA, Platform: "manual", ShopID: &sid,
		CustomerName: "perm-matrix-r164-conv", CustomerLanguage: "en",
		Status: customerchat.StatusOpen,
	}
	require.NoError(t, h.DB.Create(conv).Error)
	su := &customerchat.CustomerReplySuggestion{
		ConversationID: conv.ID, SuggestedReply: "original suggestion",
		Status: customerchat.SuggestionGenerated,
	}
	require.NoError(t, h.DB.Create(su).Error)

	convPath := func(suffix string) string {
		return "/api/v1/customer/conversations/" + conv.ID.String() + suffix
	}
	probes := []struct {
		method, path, body string
	}{
		{http.MethodPut, convPath(""), `{"customerName":"tampered"}`},
		{http.MethodDelete, convPath(""), `{}`},
		{http.MethodPost, convPath("/messages"), `{"role":"agent","content":"tampered"}`},
		{http.MethodPost, convPath("/mark-replied"), `{"reply":"tampered"}`},
		{http.MethodPost, convPath("/ai/generate-reply"), `{}`},
		{http.MethodPost, convPath("/ai-suggestions"), `{}`},
		{http.MethodPost, convPath("/send-platform-message"),
			`{"reply":"tampered","clientMessageId":"perm-matrix-r164"}`},
		{http.MethodPut, "/api/v1/customer/reply-suggestions/" + su.ID.String(), `{"editedReply":"tampered"}`},
		{http.MethodPost, "/api/v1/customer/reply-suggestions/" + su.ID.String() + "/accept", `{"finalReply":"tampered"}`},
		{http.MethodPost, "/api/v1/customer/reply-suggestions/" + su.ID.String() + "/discard", `{}`},
		{http.MethodPost, "/api/v1/customer/ai-suggestions/" + su.ID.String() + "/apply", `{"finalReply":"tampered"}`},
		{http.MethodPost, "/api/v1/customer/ai-suggestions/" + su.ID.String() + "/reject", `{"reason":"tampered"}`},
		// creating a conversation bound to a view-only store is a write too
		{http.MethodPost, "/api/v1/customer/conversations",
			fmt.Sprintf(`{"customerName":"perm-matrix-r164-create","shopId":%q}`, sid)},
	}
	for _, p := range probes {
		w := h.doBody(t, p.method, p.path, tok, p.body)
		require.Equalf(t, http.StatusForbidden, w.Code,
			"%s %s [viewOnlyOperator]: expected 403, got %d: %s", p.method, p.path, w.Code, w.Body.String())
		requireCode40303(t, w, p.method+" "+p.path)
	}

	// Zero mutation.
	var after customerchat.CustomerConversation
	require.NoError(t, h.DB.Where("id = ?", conv.ID).First(&after).Error)
	require.Equal(t, "perm-matrix-r164-conv", after.CustomerName)
	require.Equal(t, customerchat.StatusOpen, after.Status)
	var msgCount int64
	require.NoError(t, h.DB.Model(&customerchat.CustomerMessage{}).
		Where("conversation_id = ?", conv.ID).Count(&msgCount).Error)
	require.Zero(t, msgCount, "denied probes must not create messages")
	var suAfter customerchat.CustomerReplySuggestion
	require.NoError(t, h.DB.Where("id = ?", su.ID).First(&suAfter).Error)
	require.Equal(t, customerchat.SuggestionGenerated, suAfter.Status)
	require.Equal(t, "original suggestion", suAfter.SuggestedReply)
	var created int64
	require.NoError(t, h.DB.Model(&customerchat.CustomerConversation{}).
		Where("customer_name = ?", "perm-matrix-r164-create").Count(&created).Error)
	require.Zero(t, created, "denied create must not persist a conversation")

	// Reads keep working: the view grant makes the store's data visible.
	w := h.doBody(t, http.MethodGet, convPath(""), tok, "")
	require.Equal(t, http.StatusOK, w.Code,
		"view-only persona must still read the conversation: %s", w.Body.String())
	var detail struct {
		Data struct {
			CanWrite bool `json:"canWrite"`
		} `json:"data"`
	}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &detail))
	require.False(t, detail.Data.CanWrite,
		"conversation detail must report canWrite=false for a view-only store grant")
	w = h.doBody(t, http.MethodGet, convPath("/messages"), tok, "")
	require.Equal(t, http.StatusOK, w.Code,
		"view-only persona must still read messages: %s", w.Body.String())
}
