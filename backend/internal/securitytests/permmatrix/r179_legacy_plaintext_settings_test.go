package permmatrix

import (
	"encoding/json"
	"net/http"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/trademind-ai/trademind/backend/internal/modules/settings"
)

// TestSettingsLegacyPlaintextSensitiveMaskedAndAdoptedOnRead pins the R179
// rule (P2-R178-1) on the production router: a registry-listed sensitive
// settings row persisted in plaintext by an older version (which trusted the
// client isEncrypted:false) must be masked by GET /api/v1/settings and
// lazily re-encrypted at rest on read. Before the fix the legacy plaintext
// secret was echoed back unmasked until the row was next rewritten.
func TestSettingsLegacyPlaintextSensitiveMaskedAndAdoptedOnRead(t *testing.T) {
	h := sharedHarness(t)
	adminTok := h.Personas[personaAdmin].Token

	const group, key = "platform_tiktok", "app_secret"
	const plain = "r179-legacy-plaintext-app-secret"
	cleanup := func() {
		require.NoError(t, h.DB.Exec(
			"DELETE FROM settings WHERE tenant_id = ? AND group_key = ? AND item_key = ?",
			tenantA, group, key).Error)
	}
	cleanup()
	t.Cleanup(cleanup)

	// Seed the legacy row directly: old versions stored plaintext with
	// is_encrypted=false when the client declared it.
	require.NoError(t, h.DB.Create(&settings.Setting{
		TenantID:    tenantA,
		GroupKey:    group,
		ItemKey:     key,
		ItemValue:   plain,
		ValueType:   "string",
		IsEncrypted: false,
	}).Error)

	g := h.do(t, http.MethodGet, "/api/v1/settings", adminTok)
	require.Equal(t, http.StatusOK, g.Code, g.Body.String())
	var envelope struct {
		Data struct {
			Items []struct {
				GroupKey    string `json:"groupKey"`
				ItemKey     string `json:"itemKey"`
				ItemValue   string `json:"itemValue"`
				IsEncrypted bool   `json:"isEncrypted"`
			} `json:"items"`
		} `json:"data"`
	}
	require.NoError(t, json.Unmarshal(g.Body.Bytes(), &envelope))
	found := false
	for _, it := range envelope.Data.Items {
		if it.GroupKey != group || it.ItemKey != key {
			continue
		}
		found = true
		require.Falsef(t, strings.Contains(it.ItemValue, plain),
			"%s/%s: GET /settings must mask legacy plaintext secrets", group, key)
		require.Truef(t, it.IsEncrypted,
			"%s/%s: masked sensitive item must report isEncrypted=true", group, key)
	}
	require.Truef(t, found, "%s/%s: seeded item must appear in GET /settings", group, key)

	// Lazy adoption: the row must now be encrypted at rest.
	var row settings.Setting
	require.NoError(t, h.DB.
		Where("tenant_id = ? AND group_key = ? AND item_key = ?", tenantA, group, key).
		First(&row).Error)
	require.True(t, row.IsEncrypted, "legacy plaintext row must be adopted as encrypted after read")
	require.NotContains(t, row.ItemValue, plain, "secret must no longer be stored in cleartext")
}
