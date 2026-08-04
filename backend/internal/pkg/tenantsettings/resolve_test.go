package tenantsettings

import (
	"context"
	"fmt"
	"testing"

	"github.com/google/uuid"
	"github.com/trademind-ai/trademind/backend/internal/pkg/security"
)

type fakeReader struct {
	byTenant map[int64]map[string]map[string]string // tenant -> group -> kv
	err      error
}

func (f *fakeReader) PlainByGroup(_ context.Context, tenantID int64, groupKey string) (map[string]string, error) {
	if f.err != nil {
		return nil, f.err
	}
	g := f.byTenant[tenantID][groupKey]
	out := map[string]string{}
	for k, v := range g {
		out[k] = v
	}
	return out, nil
}

func tenantCtx(tid int64) context.Context {
	return security.WithTenantContext(context.Background(), &security.TenantContext{TenantID: tid, UserID: uuid.New()})
}

func TestTenantIDFromContext(t *testing.T) {
	if got := TenantID(context.Background()); got != 0 {
		t.Fatalf("no tenant context should resolve platform tenant 0, got %d", got)
	}
	if got := TenantID(tenantCtx(7)); got != 7 {
		t.Fatalf("expected 7, got %d", got)
	}
}

func TestAIPlainFallsBackToPlatformWhenTenantHasNoKey(t *testing.T) {
	r := &fakeReader{byTenant: map[int64]map[string]map[string]string{
		0: {"ai": {"provider": "deepseek", "api_key": "platform-key", "model": "deepseek-chat"}},
		2: {"ai": {"model": "qwen-max"}}, // model without own credential must NOT be mixed in
	}}
	m, err := AIPlain(tenantCtx(2), r)
	if err != nil {
		t.Fatal(err)
	}
	if m["api_key"] != "platform-key" || m["model"] != "deepseek-chat" {
		t.Fatalf("expected whole platform group, got %v", m)
	}
}

func TestAIPlainUsesTenantGroupWhenTenantHasOwnKey(t *testing.T) {
	r := &fakeReader{byTenant: map[int64]map[string]map[string]string{
		0: {"ai": {"provider": "deepseek", "api_key": "platform-key", "base_url": "https://platform.example"}},
		2: {"ai": {"provider": "qwen", "qwen_api_key": "tenant-key"}},
	}}
	m, err := AIPlain(tenantCtx(2), r)
	if err != nil {
		t.Fatal(err)
	}
	if m["qwen_api_key"] != "tenant-key" || m["provider"] != "qwen" {
		t.Fatalf("expected tenant group, got %v", m)
	}
	if m["api_key"] != "" || m["base_url"] != "" {
		t.Fatalf("platform credentials must not leak into tenant group: %v", m)
	}
}

func TestAIPlainPlatformTenantReadsOwnGroup(t *testing.T) {
	r := &fakeReader{byTenant: map[int64]map[string]map[string]string{
		0: {"ai": {"api_key": "platform-key"}},
	}}
	for _, ctx := range []context.Context{context.Background(), tenantCtx(0)} {
		m, err := AIPlain(ctx, r)
		if err != nil {
			t.Fatal(err)
		}
		if m["api_key"] != "platform-key" {
			t.Fatalf("tenant 0 must keep reading its own group, got %v", m)
		}
	}
}

func TestCollectorPlainMergesPerKey(t *testing.T) {
	r := &fakeReader{byTenant: map[int64]map[string]map[string]string{
		0: {"collector": {"goto_timeout_ms": "30000", "collect_taobao_tmall_max_retries": "2"}},
		3: {"collector": {"goto_timeout_ms": "60000", "collect_pinduoduo_timeout_ms": "45000", "collect_taobao_tmall_max_retries": " "}},
	}}
	m, err := CollectorPlain(tenantCtx(3), r)
	if err != nil {
		t.Fatal(err)
	}
	if m["goto_timeout_ms"] != "60000" {
		t.Fatalf("tenant override must win, got %v", m)
	}
	if m["collect_taobao_tmall_max_retries"] != "2" {
		t.Fatalf("blank tenant value must fall back to platform default, got %v", m)
	}
	if m["collect_pinduoduo_timeout_ms"] != "45000" {
		t.Fatalf("tenant-only key must survive merge, got %v", m)
	}
}

func TestCollectorPlainPlatformTenantUnchanged(t *testing.T) {
	r := &fakeReader{byTenant: map[int64]map[string]map[string]string{
		0: {"collector": {"goto_timeout_ms": "30000"}},
	}}
	m, err := CollectorPlain(context.Background(), r)
	if err != nil {
		t.Fatal(err)
	}
	if m["goto_timeout_ms"] != "30000" {
		t.Fatalf("expected platform values, got %v", m)
	}
}

func TestResolversPropagateReaderErrors(t *testing.T) {
	r := &fakeReader{err: fmt.Errorf("boom")}
	if _, err := AIPlain(tenantCtx(2), r); err == nil {
		t.Fatal("AIPlain must propagate reader errors")
	}
	if _, err := CollectorPlain(tenantCtx(2), r); err == nil {
		t.Fatal("CollectorPlain must propagate reader errors")
	}
}
