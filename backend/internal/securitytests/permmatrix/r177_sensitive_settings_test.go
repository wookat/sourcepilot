package permmatrix

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/trademind-ai/trademind/backend/internal/modules/settings"
)

// TestSettingsSensitiveRegistryForcesEncryptionOnCreate pins the R177 rule on
// the production router: creating a registry-listed sensitive settings item
// through PUT /api/v1/settings must encrypt it at rest and mask it on read,
// even when the client declares isEncrypted:false. Before the fix a brand-new
// row (no sticky-encryption anchor) trusted the client flag and stored the
// secret in cleartext, echoing it back unmasked on GET /api/v1/settings.
func TestSettingsSensitiveRegistryForcesEncryptionOnCreate(t *testing.T) {
	h := sharedHarness(t)
	adminTok := h.Personas[personaAdmin].Token

	cases := []struct{ group, key string }{
		{"ai", "qwen_api_key"},              // integration schema (AI key)
		{"storage", "s3_secret_access_key"}, // integration schema (S3 secret)
		{"platform_tiktok", "app_secret"},   // platform app config schema (bootstrap-registered)
		{"alert_notify", "webhook_secret"},  // integration schema (webhook secret)
	}
	cleanup := func() {
		for _, tc := range cases {
			require.NoError(t, h.DB.Exec(
				"DELETE FROM settings WHERE tenant_id = ? AND group_key = ? AND item_key = ?",
				tenantA, tc.group, tc.key).Error)
		}
	}
	cleanup()
	t.Cleanup(cleanup)

	for _, tc := range cases {
		plain := fmt.Sprintf("r177-secret-%s-%s", tc.group, tc.key)
		body := fmt.Sprintf(
			`{"items":[{"groupKey":%q,"itemKey":%q,"itemValue":%q,"isEncrypted":false}]}`,
			tc.group, tc.key, plain)
		w := h.doBody(t, http.MethodPut, "/api/v1/settings", adminTok, body)
		require.Equalf(t, http.StatusOK, w.Code,
			"PUT /api/v1/settings [%s/%s]: got %d: %s", tc.group, tc.key, w.Code, w.Body.String())

		var row settings.Setting
		require.NoError(t, h.DB.
			Where("tenant_id = ? AND group_key = ? AND item_key = ?", tenantA, tc.group, tc.key).
			First(&row).Error)
		require.Truef(t, row.IsEncrypted,
			"%s/%s: registry-listed sensitive key must be stored with is_encrypted=true", tc.group, tc.key)
		require.NotContainsf(t, row.ItemValue, plain,
			"%s/%s: secret must not be stored in cleartext", tc.group, tc.key)

		// Read-back through the API must be masked, never the plaintext.
		g := h.do(t, http.MethodGet, "/api/v1/settings", adminTok)
		require.Equal(t, http.StatusOK, g.Code, g.Body.String())
		var envelope struct {
			Data struct {
				Items []struct {
					GroupKey  string `json:"groupKey"`
					ItemKey   string `json:"itemKey"`
					ItemValue string `json:"itemValue"`
				} `json:"items"`
			} `json:"data"`
		}
		require.NoError(t, json.Unmarshal(g.Body.Bytes(), &envelope))
		found := false
		for _, it := range envelope.Data.Items {
			if it.GroupKey != tc.group || it.ItemKey != tc.key {
				continue
			}
			found = true
			require.NotEqualf(t, plain, it.ItemValue,
				"%s/%s: GET /settings must mask the secret", tc.group, tc.key)
			require.Falsef(t, strings.Contains(it.ItemValue, plain),
				"%s/%s: GET /settings leaked the plaintext", tc.group, tc.key)
		}
		require.Truef(t, found, "%s/%s: created item must appear in GET /settings", tc.group, tc.key)
	}
}
