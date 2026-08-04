package response

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/trademind-ai/trademind/backend/internal/pkg/ctxkey"
)

// OK writes a success envelope with HTTP 200.
func OK(c *gin.Context, data any) {
	JSON(c, http.StatusOK, CodeOK, "ok", data)
}

// Fail writes an error envelope; pick HTTP status and business code to match rules.
func Fail(c *gin.Context, httpStatus, bizCode int, msg string) {
	if msg == "" {
		msg = "error"
	}
	JSON(c, httpStatus, bizCode, msg, nil)
}

// JSON writes the unified API body; use for custom cases.
func JSON(c *gin.Context, httpStatus, bizCode int, msg string, data any) {
	if httpStatus >= 500 && c.GetBool(ctxkey.AuthStateBridged) {
		// Authentication passed via the last-known-good snapshot while the
		// database is unreachable: handler-level 5xx failures on such requests
		// are the same transient outage, so answer with the retryable
		// AUTH_STATE_UNAVAILABLE/503 contract instead of an internal error.
		httpStatus = http.StatusServiceUnavailable
		bizCode = CodeServiceUnavailable
		msg = MsgAuthStateUnavailable
		data = nil
	}
	tid, _ := c.Get(ctxkey.TraceID)
	trace, _ := tid.(string)
	c.JSON(httpStatus, Envelope{
		Code:    bizCode,
		Message: msg,
		Data:    data,
		TraceID: trace,
	})
}

// HandleError maps errors to HTTP + business codes (extend as domain errors appear).
func HandleError(c *gin.Context, err error) {
	if err == nil {
		return
	}
	Fail(c, http.StatusInternalServerError, CodeInternalError, "internal error")
}
