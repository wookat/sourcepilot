package auth

import (
	"testing"

	"github.com/google/uuid"
	"github.com/trademind-ai/trademind/backend/internal/config"
)

func TestLegacyMintToken_carriesTenantID(t *testing.T) {
	t.Parallel()
	cfg := &config.Config{JWTSecret: "test-secret-for-legacy-mint"}
	uid := uuid.New()
	token, _, err := LegacyMintToken(cfg, uid, "admin@example.com", 7, 1)
	if err != nil {
		t.Fatalf("mint: %v", err)
	}
	claims, err := ParseAccessToken(cfg, nil, token)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if claims.TenantID != 7 {
		t.Fatalf("got tenantId=%d, want 7", claims.TenantID)
	}
	if claims.Subject != uid.String() {
		t.Fatalf("got subject=%q, want %q", claims.Subject, uid.String())
	}
}

// Legacy tokens must carry the account's current token_version, otherwise
// request time validation locks out every account whose version was bumped
// (password reset, role change, forced logout).
func TestLegacyMintToken_carriesTokenVersion(t *testing.T) {
	t.Parallel()
	cfg := &config.Config{JWTSecret: "test-secret-for-legacy-mint"}
	token, _, err := LegacyMintToken(cfg, uuid.New(), "admin@example.com", 7, 5)
	if err != nil {
		t.Fatalf("mint: %v", err)
	}
	claims, err := ParseAccessToken(cfg, nil, token)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if claims.TokenVersion != 5 {
		t.Fatalf("got tokenVersion=%d, want 5", claims.TokenVersion)
	}
}
