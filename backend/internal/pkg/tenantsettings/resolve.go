// Package tenantsettings resolves effective settings for the tenant carried
// by the request/worker context, with explicit per-group fallback semantics
// to the platform defaults stored under tenant 0.
//
// Group policies (business decision, round 104):
//   - ai: whole-group. A tenant's ai group is used only when the tenant has
//     configured its own API key; otherwise the platform default group is
//     used as-is. Tenant and platform credentials are never mixed key-by-key
//     so a tenant model can never run against the platform's API key (or
//     vice versa).
//   - collector: per-key merge. Collector settings are behavioral knobs
//     (timeouts, retries, batch policies, auth-check URLs); a tenant may
//     override individual keys while inheriting platform defaults for the
//     rest. Empty tenant values are treated as unset.
package tenantsettings

import (
	"context"
	"strings"

	"github.com/trademind-ai/trademind/backend/internal/pkg/security"
)

// Reader is the narrow settings dependency (implemented by settings.Service).
type Reader interface {
	PlainByGroup(ctx context.Context, tenantID int64, groupKey string) (map[string]string, error)
}

// TenantID returns the trusted tenant from the context, or 0 (platform) when
// no tenant context is attached (system/bootstrap paths).
func TenantID(ctx context.Context) int64 {
	tc := security.FromContext(ctx)
	if tc != nil && tc.TenantID > 0 {
		return tc.TenantID
	}
	return 0
}

// AIPlain resolves the effective "ai" settings group for the context tenant.
// The tenant group is authoritative as a whole once the tenant configured an
// API key of its own; otherwise the platform (tenant 0) group applies.
func AIPlain(ctx context.Context, r Reader) (map[string]string, error) {
	if r == nil {
		return map[string]string{}, nil
	}
	tid := TenantID(ctx)
	if tid > 0 {
		m, err := r.PlainByGroup(ctx, tid, "ai")
		if err != nil {
			return nil, err
		}
		if hasOwnAPIKey(m) {
			return m, nil
		}
	}
	return r.PlainByGroup(ctx, 0, "ai")
}

// hasOwnAPIKey reports whether the tenant configured any AI credential
// (legacy "api_key" or a provider-specific "<provider>_api_key").
func hasOwnAPIKey(m map[string]string) bool {
	for k, v := range m {
		if strings.TrimSpace(v) == "" {
			continue
		}
		if k == "api_key" || strings.HasSuffix(k, "_api_key") {
			return true
		}
	}
	return false
}

// CollectorPlain resolves the effective "collector" settings group for the
// context tenant: platform defaults overlaid by the tenant's non-empty keys.
func CollectorPlain(ctx context.Context, r Reader) (map[string]string, error) {
	return mergedPlain(ctx, r, TenantID(ctx), "collector")
}

// InventoryPlain resolves the effective "inventory" settings group for the
// context tenant. Inventory settings are behavioral knobs (stock thresholds,
// alert switches, deduct/restore policies), so per-key merge applies: a
// tenant overrides individual keys and inherits platform defaults for the
// rest. Empty tenant values are treated as unset.
func InventoryPlain(ctx context.Context, r Reader) (map[string]string, error) {
	return mergedPlain(ctx, r, TenantID(ctx), "inventory")
}

// PricingPlain resolves the effective "pricing" settings group for the
// context tenant (per-key merge, same rationale as InventoryPlain).
func PricingPlain(ctx context.Context, r Reader) (map[string]string, error) {
	return mergedPlain(ctx, r, TenantID(ctx), "pricing")
}

// SourcingPlain resolves the effective "sourcing" settings group for the
// context tenant (per-key merge, same rationale as InventoryPlain).
func SourcingPlain(ctx context.Context, r Reader) (map[string]string, error) {
	return mergedPlain(ctx, r, TenantID(ctx), "sourcing")
}

// AlertNotifyPlain resolves the effective "alert_notify" settings group for
// the context tenant with whole-group semantics: once a tenant configured any
// alert_notify value of its own, its group is used as-is (recipients, webhook
// URL/secret and channel switches never mix with the platform's), otherwise
// the platform (tenant 0) group applies unchanged.
func AlertNotifyPlain(ctx context.Context, r Reader) (map[string]string, error) {
	return AlertNotifyPlainForTenant(ctx, r, TenantID(ctx))
}

// AlertNotifyPlainForTenant is AlertNotifyPlain keyed by an explicit tenant —
// used by the alert scan/notify pipeline where the owning tenant comes from
// the alert row rather than the request context.
func AlertNotifyPlainForTenant(ctx context.Context, r Reader, tenantID int64) (map[string]string, error) {
	if r == nil {
		return map[string]string{}, nil
	}
	if tenantID > 0 {
		m, err := r.PlainByGroup(ctx, tenantID, "alert_notify")
		if err != nil {
			return nil, err
		}
		if hasAnyValue(m) {
			return m, nil
		}
	}
	return r.PlainByGroup(ctx, 0, "alert_notify")
}

func hasAnyValue(m map[string]string) bool {
	for _, v := range m {
		if strings.TrimSpace(v) != "" {
			return true
		}
	}
	return false
}

// mergedPlain resolves a per-key-merge group: platform defaults (tenant 0)
// overlaid by the tenant's non-empty keys.
func mergedPlain(ctx context.Context, r Reader, tid int64, group string) (map[string]string, error) {
	if r == nil {
		return map[string]string{}, nil
	}
	base, err := r.PlainByGroup(ctx, 0, group)
	if err != nil {
		return nil, err
	}
	if tid == 0 {
		return base, nil
	}
	overrides, err := r.PlainByGroup(ctx, tid, group)
	if err != nil {
		return nil, err
	}
	out := make(map[string]string, len(base)+len(overrides))
	for k, v := range base {
		out[k] = v
	}
	for k, v := range overrides {
		if strings.TrimSpace(v) == "" {
			continue
		}
		out[k] = v
	}
	return out, nil
}
