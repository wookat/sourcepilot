package taskcenter

import (
	"testing"
	"time"

	"github.com/trademind-ai/trademind/backend/internal/modules/collect"
)

// 失败中心口径：不可修复失败（如 PRODUCT_NOT_FOUND）不得标记为可重试。
func TestMapCollectTaskRetryableFollowsErrorCode(t *testing.T) {
	now := time.Now()
	base := collect.CollectTask{
		Source:    "1688",
		SourceURL: "https://detail.1688.com/offer/999999999999.html",
		Status:    collect.StatusFailed,
	}

	notFound := base
	notFound.ErrorMessage = "offer_not_found:redirected_to_non_offer_page"
	dto := mapCollectTask(&notFound, nil, markSet{}, now)
	if dto.Retryable {
		t.Fatalf("PRODUCT_NOT_FOUND failure should not be retryable, error code %q", dto.ErrorCode)
	}
	if dto.ErrorCode != "PRODUCT_NOT_FOUND" {
		t.Fatalf("expected PRODUCT_NOT_FOUND error code, got %q", dto.ErrorCode)
	}
	if dto.SafeRetry {
		t.Fatal("PRODUCT_NOT_FOUND failure should not be safe-retryable")
	}

	timeout := base
	timeout.ErrorMessage = "TIMEOUT: page load timeout"
	dto = mapCollectTask(&timeout, nil, markSet{}, now)
	if !dto.Retryable {
		t.Fatal("TIMEOUT failure should stay retryable")
	}
}
