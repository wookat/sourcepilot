package ai

import (
	"context"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/trademind-ai/trademind/backend/internal/pkg/security"
)

type tenantFakeSettings struct {
	byTenant map[int64]map[string]string // tenant -> ai group kv
}

func (f *tenantFakeSettings) PlainByGroup(_ context.Context, tenantID int64, groupKey string) (map[string]string, error) {
	if groupKey != "ai" {
		return map[string]string{}, nil
	}
	out := map[string]string{}
	for k, v := range f.byTenant[tenantID] {
		out[k] = v
	}
	return out, nil
}

// A tenant with its own API key but incomplete connection settings must get
// configuration guidance for its own group instead of silently running on the
// platform's base_url/credentials.
func TestGatewayChatTenantGroupIsAtomic(t *testing.T) {
	g := &Gateway{Settings: &tenantFakeSettings{byTenant: map[int64]map[string]string{
		0: {"provider": "openai_compatible", "base_url": "https://platform.example/v1", "api_key": "platform-key"},
		2: {"provider": "openai_compatible", "api_key": "tenant-key"},
	}}}
	ctx := security.WithTenantContext(context.Background(), &security.TenantContext{TenantID: 2, UserID: uuid.New()})
	_, err := g.Chat(ctx, ChatRequest{Messages: []Message{{Role: "user", Content: "ping"}}})
	if err == nil || !strings.Contains(err.Error(), "base_url") {
		t.Fatalf("expected base_url guidance for tenant-owned group, got %v", err)
	}
}
