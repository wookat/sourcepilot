package settings

import (
	"strings"
	"sync"
)

// The sensitive key registry is the server-side declarative list of settings
// items (AI keys, platform credentials, storage secrets, webhook secrets…)
// that must be encrypted at rest and masked on read, regardless of the
// client-supplied isEncrypted flag. Writes always encrypt registry-listed
// keys; legacy rows persisted in plaintext by older versions are masked on
// read and lazily re-encrypted when an encrypter is configured (see
// Service.adoptLegacyPlaintext). It is seeded from the static integration
// schema (IntegrationConfigDefinitions) plus legacy keys; platform app/publish
// config schemas add their sensitive fields at bootstrap via
// RegisterSensitiveKeys. Items outside the registry keep the client-declared
// behaviour for compatibility.
var (
	sensitiveMu       sync.RWMutex
	sensitiveRegistry = map[string]map[string]struct{}{}
)

func init() {
	for _, sch := range IntegrationConfigDefinitions() {
		if strings.TrimSpace(sch.GroupKey) == "" {
			continue
		}
		for _, f := range sch.Fields {
			if f.Sensitive {
				RegisterSensitiveKeys(sch.GroupKey, f.Name)
			}
		}
	}
	// Legacy keys not covered by the integration schema.
	RegisterSensitiveKeys("email", "smtp_password")
	// Platform credentials are also registered at bootstrap from the platform
	// app-config schemas; the Shopee partner key is seeded statically too so
	// the guarantee holds regardless of provider bootstrap ordering (R179 线2).
	RegisterSensitiveKeys("platform_shopee", "partner_key")
}

// RegisterSensitiveKeys marks (groupKey, itemKey) pairs as sensitive.
// Group and item keys are matched case-insensitively.
func RegisterSensitiveKeys(groupKey string, itemKeys ...string) {
	gk := strings.ToLower(strings.TrimSpace(groupKey))
	if gk == "" {
		return
	}
	sensitiveMu.Lock()
	defer sensitiveMu.Unlock()
	set := sensitiveRegistry[gk]
	if set == nil {
		set = map[string]struct{}{}
		sensitiveRegistry[gk] = set
	}
	for _, ik := range itemKeys {
		k := strings.ToLower(strings.TrimSpace(ik))
		if k != "" {
			set[k] = struct{}{}
		}
	}
}

// IsSensitiveKey reports whether the registry requires (groupKey, itemKey)
// to be encrypted at rest and masked on read.
func IsSensitiveKey(groupKey, itemKey string) bool {
	sensitiveMu.RLock()
	defer sensitiveMu.RUnlock()
	set := sensitiveRegistry[strings.ToLower(strings.TrimSpace(groupKey))]
	if set == nil {
		return false
	}
	_, ok := set[strings.ToLower(strings.TrimSpace(itemKey))]
	return ok
}
