package collect

import "testing"

func TestInferErrorCodeFromMessageOfferNotFound(t *testing.T) {
	got := InferErrorCodeFromMessage("offer_not_found:redirected_to_non_offer_page")
	if got != "PRODUCT_NOT_FOUND" {
		t.Fatalf("expected PRODUCT_NOT_FOUND, got %q", got)
	}
}

func TestIsHardNonRetryableCode(t *testing.T) {
	cases := []struct {
		code string
		want bool
	}{
		{"PRODUCT_NOT_FOUND", true},
		{"product_not_found", true},
		{"INVALID_URL", true},
		{"CUSTOM_RULE_MISSING", true},
		{"TIMEOUT", false},
		{"PAGE_BLOCKED_OR_VERIFY_REQUIRED", false},
		{"PARSE_FAILED", false},
		{"", false},
	}
	for _, c := range cases {
		if got := IsHardNonRetryableCode(c.code); got != c.want {
			t.Errorf("IsHardNonRetryableCode(%q) = %v, want %v", c.code, got, c.want)
		}
	}
}

func TestIsCollectorCodeRetryableHardCodes(t *testing.T) {
	if isCollectorCodeRetryable("PRODUCT_NOT_FOUND", true, BatchSourcePolicy{RetryOnBlocked: true, RetryOnTimeout: true}) {
		t.Fatal("PRODUCT_NOT_FOUND must not be retryable even in batch")
	}
	if !isCollectorCodeRetryable("TIMEOUT", false, BatchSourcePolicy{}) {
		t.Fatal("TIMEOUT should stay retryable for single tasks")
	}
}
