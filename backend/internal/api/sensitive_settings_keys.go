package api

import (
	"strings"

	"github.com/trademind-ai/trademind/backend/internal/modules/settings"
	platformp "github.com/trademind-ai/trademind/backend/internal/providers/platform"
)

// registerPlatformSensitiveSettingsKeys feeds every registered platform's
// app/publish config schema sensitive fields into the settings sensitive key
// registry, so credentials saved through the generic settings API are
// encrypted at rest regardless of the client-supplied isEncrypted flag.
// Must run after platformp.Bootstrap().
func registerPlatformSensitiveSettingsKeys() {
	for _, p := range platformp.All() {
		for _, sch := range []platformp.PlatformAppConfigSchema{p.AppConfigSchema(), p.PublishConfigSchema()} {
			if strings.TrimSpace(sch.GroupKey) == "" {
				continue
			}
			for _, f := range sch.Fields {
				if f.Sensitive {
					settings.RegisterSensitiveKeys(sch.GroupKey, f.Name)
				}
			}
		}
	}
}
