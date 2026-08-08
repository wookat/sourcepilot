package mcptoken_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/trademind-ai/trademind/backend/internal/modules/mcptoken"
)

// R179 W1: independent write:ops scope. Readonly tokens keep exactly their
// old surface; write tokens must expire; malformed / unknown / empty scope
// sets fail closed.

func TestCreateScopedDefaultsReadonly(t *testing.T) {
	svc := &mcptoken.Service{DB: openTestDB(t)}
	res, err := svc.CreateScoped(context.Background(), 1, "ro", "", nil, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	if res.Token.Scope != mcptoken.ScopeReadonly {
		t.Fatalf("scope = %q, want readonly", res.Token.Scope)
	}
	if res.Token.ExpiresAt != nil {
		t.Fatal("readonly token must keep optional expiry semantics (nil)")
	}
}

func TestCreateScopedUnknownScopeRejected(t *testing.T) {
	svc := &mcptoken.Service{DB: openTestDB(t)}
	for _, scopes := range [][]string{{"admin"}, {"write:all"}, {"readonly", "root"}, {"*"}} {
		if _, err := svc.CreateScoped(context.Background(), 1, "bad", "", scopes, nil, nil); !errors.Is(err, mcptoken.ErrInvalidScope) {
			t.Fatalf("scopes %v: err = %v, want ErrInvalidScope", scopes, err)
		}
	}
}

func TestWriteTokenForcedExpiry(t *testing.T) {
	svc := &mcptoken.Service{DB: openTestDB(t)}
	// Default: 30 days when unspecified.
	res, err := svc.CreateScoped(context.Background(), 1, "w", "", []string{mcptoken.ScopeWriteOps}, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	if res.Token.ExpiresAt == nil {
		t.Fatal("write token issued without expiry")
	}
	want := time.Now().UTC().Add(mcptoken.WriteTokenDefaultExpiryDays * 24 * time.Hour)
	if d := res.Token.ExpiresAt.Sub(want); d < -time.Minute || d > time.Minute {
		t.Fatalf("default expiry %v, want ~%v", res.Token.ExpiresAt, want)
	}
	// Max: 90 days.
	tooLong := time.Now().UTC().Add(91 * 24 * time.Hour)
	if _, err := svc.CreateScoped(context.Background(), 1, "w2", "", []string{mcptoken.ScopeWriteOps}, &tooLong, nil); !errors.Is(err, mcptoken.ErrWriteExpiryTooLong) {
		t.Fatalf("91d expiry: err = %v, want ErrWriteExpiryTooLong", err)
	}
	// openapi purpose cannot carry write scope.
	if _, err := svc.CreateScoped(context.Background(), 1, "w3", mcptoken.PurposeOpenAPI, []string{mcptoken.ScopeWriteOps}, nil, nil); !errors.Is(err, mcptoken.ErrWritePurposeOpenAPI) {
		t.Fatalf("openapi purpose: err = %v, want ErrWritePurposeOpenAPI", err)
	}
}

func TestScopeSetAuthentication(t *testing.T) {
	svc := &mcptoken.Service{DB: openTestDB(t)}
	ctx := context.Background()

	ro, err := svc.Create(ctx, 1, "ro", mcptoken.PurposeBoth, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	wo, err := svc.CreateScoped(ctx, 1, "wo", "", []string{mcptoken.ScopeWriteOps}, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	both, err := svc.CreateScoped(ctx, 1, "rw", mcptoken.PurposeBoth, []string{mcptoken.ScopeReadonly, mcptoken.ScopeWriteOps}, nil, nil)
	if err != nil {
		t.Fatal(err)
	}

	// MCP entry: readonly, write-only and combined tokens all authenticate.
	for _, plain := range []string{ro.Plaintext, wo.Plaintext, both.Plaintext} {
		if _, err := svc.Authenticate(ctx, plain); err != nil {
			t.Fatalf("mcp entry rejected valid token: %v", err)
		}
	}
	// Open API entry: write-only token is rejected (write:ops never widens
	// the readonly surface); readonly membership passes.
	if _, err := svc.AuthenticateFor(ctx, wo.Plaintext, mcptoken.PurposeOpenAPI); !errors.Is(err, mcptoken.ErrInvalidToken) {
		t.Fatalf("openapi accepted write-only token: %v", err)
	}
	if _, err := svc.AuthenticateFor(ctx, both.Plaintext, mcptoken.PurposeOpenAPI); err != nil {
		t.Fatalf("openapi rejected readonly+write token: %v", err)
	}

	// Malformed / unknown / empty scope columns authorize nothing.
	for _, scope := range []string{"", "root", "readonly2", "write", "WRITE:OPS"} {
		if err := svc.DB.Model(&mcptoken.Token{}).Where("id = ?", ro.Token.ID).Update("scope", scope).Error; err != nil {
			t.Fatal(err)
		}
		if _, err := svc.Authenticate(ctx, ro.Plaintext); !errors.Is(err, mcptoken.ErrInvalidToken) {
			t.Fatalf("scope %q authenticated on mcp entry: %v", scope, err)
		}
		if _, err := svc.AuthenticateFor(ctx, ro.Plaintext, mcptoken.PurposeOpenAPI); !errors.Is(err, mcptoken.ErrInvalidToken) {
			t.Fatalf("scope %q authenticated on openapi entry: %v", scope, err)
		}
	}
}

func TestParseScopesIgnoresUnknownMembers(t *testing.T) {
	got := mcptoken.ParseScopes("readonly, bogus ,write:ops,readonly")
	if len(got) != 2 || got[0] != mcptoken.ScopeReadonly || got[1] != mcptoken.ScopeWriteOps {
		t.Fatalf("ParseScopes = %v", got)
	}
	if len(mcptoken.ParseScopes("bogus")) != 0 {
		t.Fatal("unknown-only scope parsed to members")
	}
}
