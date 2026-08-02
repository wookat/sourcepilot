package errmap

import (
	"errors"
	"fmt"
	"testing"

	"github.com/trademind-ai/trademind/backend/internal/providers/ai/compatclient"
)

func TestMapChatErrorCodes(t *testing.T) {
	cases := []struct {
		name     string
		err      error
		wantCode string
	}{
		{"401 invalid key", &compatclient.HTTPError{StatusCode: 401}, CodeInvalidKey},
		{"403 forbidden", &compatclient.HTTPError{StatusCode: 403}, CodeForbidden},
		{"429 quota", &compatclient.HTTPError{StatusCode: 429}, CodeQuotaExceeded},
		{"503 upstream", &compatclient.HTTPError{StatusCode: 503}, CodeUpstreamError},
		{"api_key empty", errors.New("api_key empty"), CodeNotConfigured},
		{"base_url empty", errors.New("base_url empty"), CodeNotConfigured},
		{"connection refused", errors.New("dial tcp: connection refused"), CodeBadBaseURL},
		{"unclassified", errors.New("some other failure"), ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			mapped := MapChatError("测试服务商", tc.err)
			if got := CodeOf(mapped); got != tc.wantCode {
				t.Fatalf("CodeOf(%v) = %q, want %q", mapped, got, tc.wantCode)
			}
			if mapped == nil || mapped.Error() == "" {
				t.Fatalf("mapped error must keep a user-facing message")
			}
		})
	}
}

func TestCodeOfWrappedError(t *testing.T) {
	inner := NotConfigured("请配置 API Key")
	wrapped := fmt.Errorf("optimize title: %w", inner)
	if got := CodeOf(wrapped); got != CodeNotConfigured {
		t.Fatalf("CodeOf(wrapped) = %q, want %q", got, CodeNotConfigured)
	}
	if got := CodeOf(nil); got != "" {
		t.Fatalf("CodeOf(nil) = %q, want empty", got)
	}
}
