package mcptoken_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/trademind-ai/trademind/backend/internal/modules/mcptoken"
)

func TestCreateRejectsPastExpiry(t *testing.T) {
	svc := &mcptoken.Service{DB: openTestDB(t)}
	past := time.Now().UTC().Add(-time.Hour)
	if _, err := svc.Create(context.Background(), 1, "past", &past, nil); !errors.Is(err, mcptoken.ErrInvalidExpiry) {
		t.Fatalf("expected ErrInvalidExpiry, got %v", err)
	}
}

func TestAuthenticateHonorsExpiry(t *testing.T) {
	svc := &mcptoken.Service{DB: openTestDB(t)}

	future := time.Now().UTC().Add(time.Hour)
	res, err := svc.Create(context.Background(), 1, "future", &future, nil)
	if err != nil {
		t.Fatal(err)
	}
	if res.Token.ExpiresAt == nil {
		t.Fatal("expected ExpiresAt persisted")
	}
	if _, err := svc.Authenticate(context.Background(), res.Plaintext); err != nil {
		t.Fatalf("unexpired token should authenticate: %v", err)
	}

	// Force the stored expiry into the past and expect rejection.
	expired := time.Now().UTC().Add(-time.Minute)
	if err := svc.DB.Model(&mcptoken.Token{}).Where("id = ?", res.Token.ID).
		Update("expires_at", expired).Error; err != nil {
		t.Fatal(err)
	}
	if _, err := svc.Authenticate(context.Background(), res.Plaintext); !errors.Is(err, mcptoken.ErrInvalidToken) {
		t.Fatalf("expired token must be rejected, got %v", err)
	}
}

func TestAuthenticateNoExpiryStillWorks(t *testing.T) {
	svc := &mcptoken.Service{DB: openTestDB(t)}
	res, err := svc.Create(context.Background(), 1, "forever", nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	if res.Token.ExpiresAt != nil {
		t.Fatal("expected no expiry by default")
	}
	if _, err := svc.Authenticate(context.Background(), res.Plaintext); err != nil {
		t.Fatalf("non-expiring token should authenticate: %v", err)
	}
}
