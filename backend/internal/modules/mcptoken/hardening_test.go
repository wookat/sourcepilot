package mcptoken_test

import (
	"context"
	"errors"
	"fmt"
	"testing"

	"github.com/trademind-ai/trademind/backend/internal/modules/mcptoken"
)

// A token row whose scope is not readonly must never authenticate: the MCP
// entry accepts read-only scope only, so any future scope fails closed here.
func TestAuthenticateRejectsNonReadonlyScope(t *testing.T) {
	svc := &mcptoken.Service{DB: openTestDB(t)}
	res, err := svc.Create(context.Background(), 1, "scope-check", nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := svc.Authenticate(context.Background(), res.Plaintext); err != nil {
		t.Fatalf("readonly token rejected: %v", err)
	}
	if err := svc.DB.Model(&mcptoken.Token{}).Where("id = ?", res.Token.ID).
		Update("scope", "write").Error; err != nil {
		t.Fatal(err)
	}
	if _, err := svc.Authenticate(context.Background(), res.Plaintext); !errors.Is(err, mcptoken.ErrInvalidToken) {
		t.Fatalf("scope=write token: err = %v, want ErrInvalidToken", err)
	}
}

// Each token carries its own rate-limit bucket, so the number of active tokens
// per tenant is capped; revoking one frees a slot.
func TestCreateEnforcesActiveTokenCap(t *testing.T) {
	svc := &mcptoken.Service{DB: openTestDB(t)}
	var lastID = ""
	for i := 0; i < mcptoken.MaxActiveTokensPerTenant; i++ {
		res, err := svc.Create(context.Background(), 1, fmt.Sprintf("tok-%d", i), nil, nil)
		if err != nil {
			t.Fatalf("create %d: %v", i, err)
		}
		lastID = res.Token.ID.String()
	}
	if _, err := svc.Create(context.Background(), 1, "over-cap", nil, nil); !errors.Is(err, mcptoken.ErrTooManyTokens) {
		t.Fatalf("over cap: err = %v, want ErrTooManyTokens", err)
	}
	// Another tenant is unaffected by this tenant's cap.
	if _, err := svc.Create(context.Background(), 2, "other-tenant", nil, nil); err != nil {
		t.Fatalf("other tenant blocked by cap: %v", err)
	}
	var row mcptoken.Token
	if err := svc.DB.First(&row, "id = ?", lastID).Error; err != nil {
		t.Fatal(err)
	}
	if _, err := svc.Revoke(context.Background(), 1, row.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.Create(context.Background(), 1, "after-revoke", nil, nil); err != nil {
		t.Fatalf("create after revoke: %v", err)
	}
}
