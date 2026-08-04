package auth

import (
	"context"
	"testing"
)

// Round108: ValidateSessionAccessDetailed must report when the decision was
// bridged from the last-known-good snapshot (database outage), so the
// middleware can mark the request and 5xx handler failures get rewritten to
// the AUTH_STATE_UNAVAILABLE/503 retryable contract.
func TestValidateSessionAccessDetailedReportsBridged(t *testing.T) {
	svc, res, u := newSessionFailClosedFixture(t)
	bridged, err := svc.ValidateSessionAccessDetailed(context.Background(), res.SessionID, u.ID, u.TokenVersion)
	if err != nil {
		t.Fatalf("warm validate: %v", err)
	}
	if bridged {
		t.Fatal("healthy database validation must not report bridged")
	}
	if err := svc.DB.Exec(`DROP TABLE auth_sessions`).Error; err != nil {
		t.Fatal(err)
	}
	bridged, err = svc.ValidateSessionAccessDetailed(context.Background(), res.SessionID, u.ID, u.TokenVersion)
	if err != nil {
		t.Fatalf("fresh snapshot should bridge db error, got %v", err)
	}
	if !bridged {
		t.Fatal("snapshot-bridged validation must report bridged")
	}
}
