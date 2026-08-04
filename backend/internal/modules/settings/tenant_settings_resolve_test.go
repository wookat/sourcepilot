package settings

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/trademind-ai/trademind/backend/internal/pkg/security"
	"github.com/trademind-ai/trademind/backend/internal/pkg/tenantsettings"
)

func resolveCtx(tenantID int64) context.Context {
	return security.WithTenantContext(context.Background(), &security.TenantContext{TenantID: tenantID, UserID: uuid.New()})
}

// Round 104: AI settings are tenant-scoped with whole-group fallback to the
// platform defaults; encrypted values keep the AES-GCM storage/decrypt path
// regardless of tenant. Two tenants' configurations must never mix.
func TestAIPlainDualTenantIsolationWithEncryption(t *testing.T) {
	svc := newSettingsTestSvc(t)
	ctx := context.Background()

	if err := svc.PutBulk(ctx, []PutItem{
		{TenantID: 0, GroupKey: "ai", ItemKey: "provider", ItemValue: "deepseek"},
		{TenantID: 0, GroupKey: "ai", ItemKey: "base_url", ItemValue: "https://platform.example/v1"},
		{TenantID: 0, GroupKey: "ai", ItemKey: "api_key", ItemValue: "sk-platform", IsEncrypted: true},
		{TenantID: 2, GroupKey: "ai", ItemKey: "provider", ItemValue: "qwen"},
		{TenantID: 2, GroupKey: "ai", ItemKey: "base_url", ItemValue: "https://tenant2.example/v1"},
		{TenantID: 2, GroupKey: "ai", ItemKey: "qwen_api_key", ItemValue: "sk-tenant2", IsEncrypted: true},
	}); err != nil {
		t.Fatal(err)
	}

	// Tenant 2 configured its own credential: whole tenant group applies.
	m2, err := tenantsettings.AIPlain(resolveCtx(2), svc)
	if err != nil {
		t.Fatal(err)
	}
	if m2["qwen_api_key"] != "sk-tenant2" || m2["provider"] != "qwen" {
		t.Fatalf("tenant 2 must read its own decrypted group, got %v", m2)
	}
	if m2["api_key"] != "" {
		t.Fatalf("platform credential must not leak into tenant 2 group: %v", m2)
	}

	// Tenant 3 configured nothing: whole platform group applies (decrypted).
	m3, err := tenantsettings.AIPlain(resolveCtx(3), svc)
	if err != nil {
		t.Fatal(err)
	}
	if m3["api_key"] != "sk-platform" || m3["provider"] != "deepseek" {
		t.Fatalf("unconfigured tenant must fall back to platform defaults, got %v", m3)
	}

	// Platform tenant keeps reading its own group out of the box.
	m0, err := tenantsettings.AIPlain(context.Background(), svc)
	if err != nil {
		t.Fatal(err)
	}
	if m0["api_key"] != "sk-platform" {
		t.Fatalf("tenant 0 must keep its own configuration, got %v", m0)
	}
}

// Collector settings merge per key: tenant overrides win, blanks inherit.
func TestCollectorPlainDualTenantMerge(t *testing.T) {
	svc := newSettingsTestSvc(t)
	ctx := context.Background()

	if err := svc.PutBulk(ctx, []PutItem{
		{TenantID: 0, GroupKey: "collector", ItemKey: "goto_timeout_ms", ItemValue: "30000"},
		{TenantID: 0, GroupKey: "collector", ItemKey: "collect_taobao_tmall_max_retries", ItemValue: "2"},
		{TenantID: 2, GroupKey: "collector", ItemKey: "goto_timeout_ms", ItemValue: "60000"},
	}); err != nil {
		t.Fatal(err)
	}

	m2, err := tenantsettings.CollectorPlain(resolveCtx(2), svc)
	if err != nil {
		t.Fatal(err)
	}
	if m2["goto_timeout_ms"] != "60000" || m2["collect_taobao_tmall_max_retries"] != "2" {
		t.Fatalf("tenant 2 merge mismatch: %v", m2)
	}

	m3, err := tenantsettings.CollectorPlain(resolveCtx(3), svc)
	if err != nil {
		t.Fatal(err)
	}
	if m3["goto_timeout_ms"] != "30000" {
		t.Fatalf("unconfigured tenant must see platform defaults: %v", m3)
	}
}
