package response

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/trademind-ai/trademind/backend/internal/pkg/ctxkey"
)

// Round108 regression: when authentication passed via the last-known-good
// snapshot (database outage), a handler-level 500 must be rewritten to the
// retryable AUTH_STATE_UNAVAILABLE/503 contract instead of a plain internal
// error, so the frontend backoff-retry path covers it.
func TestJSONRewritesBridged5xxToUnavailable(t *testing.T) {
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Set(ctxkey.AuthStateBridged, true)
	Fail(c, 500, CodeInternalError, "db query failed")
	if w.Code != http.StatusServiceUnavailable {
		t.Fatalf("status = %d, want 503", w.Code)
	}
	var env Envelope
	if err := json.Unmarshal(w.Body.Bytes(), &env); err != nil {
		t.Fatal(err)
	}
	if env.Code != CodeServiceUnavailable || env.Message != MsgAuthStateUnavailable {
		t.Fatalf("envelope = %d/%s, want %d/%s", env.Code, env.Message, CodeServiceUnavailable, MsgAuthStateUnavailable)
	}
}

// Without the bridged flag a 500 stays a 500, and bridged non-5xx responses
// are untouched.
func TestJSONBridgedRewriteScope(t *testing.T) {
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	Fail(c, 500, CodeInternalError, "boom")
	if w.Code != http.StatusInternalServerError {
		t.Fatalf("non-bridged 500 rewritten: %d", w.Code)
	}
	w2 := httptest.NewRecorder()
	c2, _ := gin.CreateTestContext(w2)
	c2.Set(ctxkey.AuthStateBridged, true)
	Fail(c2, 400, CodeBadRequest, "bad input")
	if w2.Code != http.StatusBadRequest {
		t.Fatalf("bridged 400 rewritten: %d", w2.Code)
	}
}
