package settings

import (
	"context"
	"strings"
	"testing"
)

// TestPutBulkNewSensitiveKeyForcedEncrypted pins the R177 rule: creating a
// registry-listed sensitive item must encrypt it at rest even when the client
// declares isEncrypted:false (or omits it). Before the fix, first-time
// creation of e.g. ai/deepseek_api_key with isEncrypted:false stored the
// secret in cleartext and echoed it back unmasked.
func TestPutBulkNewSensitiveKeyForcedEncrypted(t *testing.T) {
	svc := newSettingsTestSvc(t)
	ctx := context.Background()

	const plain = "sk-r177-new-item-secret"
	if err := svc.PutBulk(ctx, []PutItem{
		{GroupKey: "ai", ItemKey: "deepseek_api_key", ItemValue: plain, IsEncrypted: false},
	}); err != nil {
		t.Fatal(err)
	}

	var row Setting
	if err := svc.DB.Where("group_key = ? AND item_key = ?", "ai", "deepseek_api_key").First(&row).Error; err != nil {
		t.Fatal(err)
	}
	if !row.IsEncrypted {
		t.Fatal("registry-listed sensitive key must be stored with is_encrypted=true regardless of client flag")
	}
	if strings.Contains(row.ItemValue, plain) {
		t.Fatalf("secret stored in cleartext at rest: %q", row.ItemValue)
	}

	plainOut, err := svc.PlainByGroup(ctx, 0, "ai")
	if err != nil {
		t.Fatal(err)
	}
	if plainOut["deepseek_api_key"] != plain {
		t.Fatalf("stored secret must round-trip, got %q", plainOut["deepseek_api_key"])
	}
	list, err := svc.List(ctx, 0)
	if err != nil {
		t.Fatal(err)
	}
	for _, it := range list {
		if it.ItemKey == "deepseek_api_key" && it.ItemValue == plain {
			t.Fatal("List must mask the secret, not echo the plaintext")
		}
	}
}

// TestPutBulkNonRegistryKeyKeepsClientBehaviour pins compatibility: items
// outside the sensitive registry keep the client-declared isEncrypted
// behaviour (plaintext by default, encrypted when requested).
func TestPutBulkNonRegistryKeyKeepsClientBehaviour(t *testing.T) {
	svc := newSettingsTestSvc(t)
	ctx := context.Background()

	if err := svc.PutBulk(ctx, []PutItem{
		{GroupKey: "ai", ItemKey: "deepseek_base_url", ItemValue: "https://api.deepseek.com/v1"},
		{GroupKey: "custom_group", ItemKey: "custom_secret", ItemValue: "opt-in-secret", IsEncrypted: true},
	}); err != nil {
		t.Fatal(err)
	}

	var row Setting
	if err := svc.DB.Where("group_key = ? AND item_key = ?", "ai", "deepseek_base_url").First(&row).Error; err != nil {
		t.Fatal(err)
	}
	if row.IsEncrypted || row.ItemValue != "https://api.deepseek.com/v1" {
		t.Fatalf("non-registry item must keep plaintext behaviour, got encrypted=%v value=%q", row.IsEncrypted, row.ItemValue)
	}

	var optIn Setting
	if err := svc.DB.Where("group_key = ? AND item_key = ?", "custom_group", "custom_secret").First(&optIn).Error; err != nil {
		t.Fatal(err)
	}
	if !optIn.IsEncrypted || strings.Contains(optIn.ItemValue, "opt-in-secret") {
		t.Fatalf("client-requested encryption must still work, got encrypted=%v value=%q", optIn.IsEncrypted, optIn.ItemValue)
	}
}

// TestSensitiveRegistryCoversPlatformCredentialShape pins the registry API
// used by platform bootstrap: registered keys match case-insensitively and
// unknown keys stay out.
func TestSensitiveRegistryCoversPlatformCredentialShape(t *testing.T) {
	RegisterSensitiveKeys("platform_r177test", "app_secret")
	if !IsSensitiveKey("platform_r177test", "app_secret") {
		t.Fatal("registered platform key must be sensitive")
	}
	if !IsSensitiveKey("Platform_R177Test", "App_Secret") {
		t.Fatal("registry must match case-insensitively")
	}
	if IsSensitiveKey("platform_r177test", "app_key") {
		t.Fatal("unregistered key must not be sensitive")
	}
	if !IsSensitiveKey("storage", "s3_secret_access_key") {
		t.Fatal("integration schema sensitive fields must seed the registry")
	}
	if !IsSensitiveKey("email", "smtp_password") {
		t.Fatal("legacy email/smtp_password must be in the registry")
	}
}
