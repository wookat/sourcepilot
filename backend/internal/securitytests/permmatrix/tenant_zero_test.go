package permmatrix

import (
	"encoding/json"
	"net/http"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"github.com/trademind-ai/trademind/backend/internal/modules/collectbrowserprofile"
	"github.com/trademind-ai/trademind/backend/internal/modules/collectrule"
	"github.com/trademind-ai/trademind/backend/internal/modules/product"
	"github.com/trademind-ai/trademind/backend/internal/modules/settings"
	"github.com/trademind-ai/trademind/backend/internal/pkg/model"
	"gorm.io/datatypes"
)

// platformOnlyRoutes are deployment-wide operations: a business tenant admin
// must never reach them, whatever role it holds inside its own tenant.
var platformOnlyRoutes = []struct{ method, path string }{
	{http.MethodGet, "/api/v1/ops/backups"},
	{http.MethodPost, "/api/v1/ops/backups"},
	{http.MethodGet, "/api/v1/ops/restores"},
	{http.MethodPost, "/api/v1/ops/restores"},
	{http.MethodGet, "/api/v1/ops/releases"},
	{http.MethodPost, "/api/v1/ops/releases"},
	{http.MethodGet, "/api/v1/ops/dr/status"},
	{http.MethodPost, "/api/v1/ops/dr/drills"},
}

func TestPlatformOnlyOpsRoutesRejectTenantAdmin(t *testing.T) {
	h := sharedHarness(t)
	for _, r := range platformOnlyRoutes {
		w := h.do(t, r.method, r.path, h.Personas[personaAdmin].Token)
		require.Equal(t, http.StatusForbidden, w.Code,
			"%s %s must be platform-tenant only, got %d body=%s", r.method, r.path, w.Code, w.Body.String())
	}
}

// The shared prompt catalog has no tenant column and may carry
// platform-customized prompt content, so reads and writes are both platform
// only (round103); tenant AI features consume it server-side.
func TestAIPromptWritesArePlatformOnly(t *testing.T) {
	h := sharedHarness(t)
	admin := h.Personas[personaAdmin].Token

	require.Equal(t, http.StatusForbidden, h.do(t, http.MethodGet, "/api/v1/ai/prompts", admin).Code)
	require.Equal(t, http.StatusOK, h.do(t, http.MethodGet, "/api/v1/ai/prompts", h.Personas[personaPlatformAdmin].Token).Code)

	body := `{"code":"perm-matrix-probe","name":"probe","scene":"title","systemPrompt":"s","userPrompt":"u"}`
	w := h.doBody(t, http.MethodPost, "/api/v1/ai/prompts", admin, body)
	require.Equal(t, http.StatusForbidden, w.Code, "tenant admin must not create prompts: %s", w.Body.String())

	w = h.doBody(t, http.MethodPut, "/api/v1/ai/prompts/"+uuid.NewString(), admin, body)
	require.Equal(t, http.StatusForbidden, w.Code, "tenant admin must not update prompts: %s", w.Body.String())
}

// Settings rows of tenant 0 hold platform credentials; business tenants must
// neither read nor write them.
func TestSettingsTenantZeroIsolation(t *testing.T) {
	h := sharedHarness(t)
	require.NoError(t, h.DB.Exec(
		`INSERT INTO settings (tenant_id, group_key, item_key, item_value, value_type, is_encrypted)
		 VALUES (0, 'perm_matrix_platform', 'secret_probe', 'platform-only', 'string', false)
		 ON CONFLICT (tenant_id, group_key, item_key) DO NOTHING`).Error)

	w := h.do(t, http.MethodGet, "/api/v1/settings", h.Personas[personaAdmin].Token)
	require.Equal(t, http.StatusOK, w.Code)
	var env struct {
		Data struct {
			Items []settings.Setting `json:"items"`
		} `json:"data"`
	}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &env))
	for _, it := range env.Data.Items {
		require.NotEqual(t, int64(0), it.TenantID, "tenant admin must not read tenant 0 settings")
	}

	w = h.doBody(t, http.MethodPut, "/api/v1/settings", h.Personas[personaAdmin].Token,
		`{"items":[{"tenantId":0,"groupKey":"perm_matrix_platform","itemKey":"secret_probe","itemValue":"tampered"}]}`)
	require.Equal(t, http.StatusForbidden, w.Code, "writing tenant 0 settings must be denied: %s", w.Body.String())

	var value string
	require.NoError(t, h.DB.Raw(
		`SELECT item_value FROM settings WHERE tenant_id = 0 AND group_key = 'perm_matrix_platform' AND item_key = 'secret_probe'`).
		Scan(&value).Error)
	require.Equal(t, "platform-only", value, "tenant 0 setting must be unchanged")
}

// Collect rules and browser profiles are tenant owned: a peer tenant must not
// list, read, mutate or delete them.
func TestCollectRuleAndProfileTenantIsolation(t *testing.T) {
	h := sharedHarness(t)
	cross := h.Personas[personaCrossTenant].Token

	rule := &collectrule.CollectRule{
		Base:     model.Base{ID: uuid.New()},
		TenantID: tenantA,
		Name:     "perm-matrix-rule",
		Source:   collectrule.SourceCustom,
		Domain:   "perm-matrix.example.com",
		Status:   collectrule.StatusDisabled,
		Priority: 100,
		Rule:     datatypes.JSON([]byte(`{"fields":{"title":{"selector":"h1"}}}`)),
	}
	require.NoError(t, h.DB.Create(rule).Error)
	profile := &collectbrowserprofile.CollectBrowserProfile{
		Base:       model.Base{ID: uuid.New()},
		TenantID:   tenantA,
		Name:       "perm-matrix-profile",
		Domain:     "perm-matrix.example.com",
		ProfileKey: "perm_matrix_" + uuid.NewString(),
		Status:     collectbrowserprofile.StatusActive,
	}
	require.NoError(t, h.DB.Create(profile).Error)
	t.Cleanup(func() {
		h.DB.Unscoped().Delete(&collectrule.CollectRule{}, "id = ?", rule.ID)
		h.DB.Unscoped().Delete(&collectbrowserprofile.CollectBrowserProfile{}, "id = ?", profile.ID)
	})

	w := h.do(t, http.MethodGet, "/api/v1/collect/rules?pageSize=200", cross)
	require.Equal(t, http.StatusOK, w.Code)
	require.NotContains(t, w.Body.String(), rule.ID.String(), "peer tenant must not list tenant A rules")

	for _, r := range []struct{ method, path string }{
		{http.MethodGet, "/api/v1/collect/rules/" + rule.ID.String()},
		{http.MethodPut, "/api/v1/collect/rules/" + rule.ID.String()},
		{http.MethodDelete, "/api/v1/collect/rules/" + rule.ID.String()},
		{http.MethodPost, "/api/v1/collect/browser-profiles/" + profile.ID.String() + "/disable"},
		{http.MethodDelete, "/api/v1/collect/browser-profiles/" + profile.ID.String()},
	} {
		w := h.do(t, r.method, r.path, cross)
		require.Contains(t, []int{http.StatusNotFound, http.StatusForbidden, http.StatusBadRequest}, w.Code,
			"%s %s must be denied for peer tenant, got %d body=%s", r.method, r.path, w.Code, w.Body.String())
	}

	w = h.do(t, http.MethodGet, "/api/v1/collect/browser-profiles?pageSize=200", cross)
	require.Equal(t, http.StatusOK, w.Code)
	require.NotContains(t, w.Body.String(), profile.ID.String(), "peer tenant must not list tenant A profiles")

	var status string
	require.NoError(t, h.DB.Raw(`SELECT status FROM collect_browser_profiles WHERE id = ?`, profile.ID).Scan(&status).Error)
	require.Equal(t, collectbrowserprofile.StatusActive, status, "peer tenant must not disable tenant A profile")
}

// Product sub-resources are spread over several modules; the route guard must
// reject any product id that belongs to another tenant.
func TestProductSubResourceTenantGuard(t *testing.T) {
	h := sharedHarness(t)
	p := &product.Product{Base: model.Base{ID: uuid.New()}, TenantID: tenantA, Title: "perm-matrix-product", Status: "draft"}
	require.NoError(t, h.DB.Create(p).Error)
	t.Cleanup(func() { h.DB.Unscoped().Delete(&product.Product{}, "id = ?", p.ID) })

	cross := h.Personas[personaCrossTenant].Token
	for _, path := range []string{
		"/api/v1/products/" + p.ID.String(),
		"/api/v1/products/" + p.ID.String() + "/skus",
		"/api/v1/products/" + p.ID.String() + "/images",
	} {
		w := h.do(t, http.MethodGet, path, cross)
		require.Contains(t, []int{http.StatusNotFound, http.StatusForbidden}, w.Code,
			"GET %s must be denied for peer tenant, got %d body=%s", path, w.Code, w.Body.String())
	}
}
