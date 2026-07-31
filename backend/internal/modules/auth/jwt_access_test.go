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
	token, _, err := LegacyMintToken(cfg, uid, "admin@example.com", 7)
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
