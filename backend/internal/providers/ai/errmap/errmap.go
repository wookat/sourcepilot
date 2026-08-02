package errmap

import (
	"errors"
	"fmt"
	"strings"

	"github.com/trademind-ai/trademind/backend/internal/providers/ai/compatclient"
)

// Structured AI error codes exposed to API clients via the `errorCode` field.
const (
	CodeNotConfigured = "AI_NOT_CONFIGURED"  // base_url / API Key 未配置
	CodeInvalidKey    = "AI_INVALID_KEY"     // 401 无效或未授权
	CodeForbidden     = "AI_FORBIDDEN"       // 403 无权限访问模型
	CodeModelNotFound = "AI_MODEL_NOT_FOUND" // 模型不存在或无权限
	CodeBadBaseURL    = "AI_BAD_BASE_URL"    // base_url 不可访问或路径错误
	CodeQuotaExceeded = "AI_QUOTA_EXCEEDED"  // 429 频率或额度受限
	CodeUpstreamError = "AI_UPSTREAM_ERROR"  // 5xx 服务商异常
	CodeTimeout       = "AI_TIMEOUT"         // 请求超时
	CodeBadResponse   = "AI_BAD_RESPONSE"    // 响应格式不兼容
)

// Error carries a structured code alongside a user-facing Chinese message.
type Error struct {
	Code    string
	Message string
	cause   error
}

func (e *Error) Error() string { return e.Message }

func (e *Error) Unwrap() error { return e.cause }

func newErr(code, format string, args ...any) *Error {
	return &Error{Code: code, Message: fmt.Sprintf(format, args...)}
}

// NotConfigured builds an AI_NOT_CONFIGURED error with the given message.
func NotConfigured(message string) *Error {
	return &Error{Code: CodeNotConfigured, Message: message}
}

// CodeOf extracts the structured code from an AI error chain ("" when unclassified).
func CodeOf(err error) string {
	var e *Error
	if errors.As(err, &e) {
		return e.Code
	}
	return ""
}

// MapChatError converts compatclient errors to user-facing Chinese messages with structured codes.
func MapChatError(providerLabel string, err error) error {
	if err == nil {
		return nil
	}
	if compatclient.IsTimeout(err) {
		return newErr(CodeTimeout, "%s 请求超时", providerLabel)
	}
	var he *compatclient.HTTPError
	if errors.As(err, &he) {
		msg := compatclient.APIErrorMessage(he.Body)
		switch he.StatusCode {
		case 401:
			return newErr(CodeInvalidKey, "API Key 无效或未授权")
		case 403:
			return newErr(CodeForbidden, "当前账号无权限访问该模型")
		case 404:
			if compatclient.IsInvalidModel(he.Body, msg) {
				return newErr(CodeModelNotFound, "模型不存在或无权限")
			}
			return newErr(CodeBadBaseURL, "base_url 不可访问或接口路径错误")
		case 429:
			return newErr(CodeQuotaExceeded, "请求过于频繁或额度受限")
		case 502, 503, 504:
			return newErr(CodeUpstreamError, "服务商暂时不可用，请稍后重试")
		default:
			if compatclient.IsInvalidModel(he.Body, msg) {
				return newErr(CodeModelNotFound, "模型不存在或无权限")
			}
			if he.StatusCode >= 400 && he.StatusCode < 500 {
				if msg != "" {
					return fmt.Errorf("%s: %s", providerLabel, msg)
				}
				return fmt.Errorf("%s 返回 HTTP %d", providerLabel, he.StatusCode)
			}
			return newErr(CodeUpstreamError, "%s 服务异常（HTTP %d）", providerLabel, he.StatusCode)
		}
	}
	low := strings.ToLower(err.Error())
	if strings.Contains(low, "base_url empty") || strings.Contains(low, "base url") {
		return NotConfigured("请配置 base_url")
	}
	if strings.Contains(low, "api_key empty") {
		return NotConfigured("请配置 API Key")
	}
	if strings.Contains(low, "decode") || strings.Contains(low, "unmarshal") {
		return newErr(CodeBadResponse, "响应格式不兼容")
	}
	if strings.Contains(low, "connection refused") || strings.Contains(low, "no such host") {
		return newErr(CodeBadBaseURL, "base_url 不可访问")
	}
	return fmt.Errorf("%s: %w", providerLabel, err)
}
