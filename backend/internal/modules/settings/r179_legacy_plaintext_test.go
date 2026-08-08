package settings

import (
	"context"
	"strings"
	"testing"

	"github.com/trademind-ai/trademind/backend/internal/encrypt"
)

func seedLegacyPlainRow(t *testing.T, svc *Service, tenantID int64, gk, ik, plain string) {
	t.Helper()
	row := Setting{
		TenantID:    tenantID,
		GroupKey:    gk,
		ItemKey:     ik,
		ItemValue:   plain,
		ValueType:   "string",
		IsEncrypted: false,
	}
	if err := svc.DB.Create(&row).Error; err != nil {
		t.Fatal(err)
	}
}

// TestListMasksLegacyPlaintextSensitiveRow pins the R179 rule (P2-R178-1):
// a registry-listed sensitive item that was stored in cleartext by an old
// version (which trusted the client isEncrypted:false) must still be masked
// on read. Before the fix, GET /api/v1/settings echoed the legacy plaintext
// secret back unmasked until the row was rewritten.
func TestListMasksLegacyPlaintextSensitiveRow(t *testing.T) {
	svc := newSettingsTestSvc(t)
	ctx := context.Background()

	// Platform schema keys are registered at API bootstrap; mirror that here.
	RegisterSensitiveKeys("platform_r179test", "app_secret")
	const plain = "legacy-r179-platform-app-secret"
	seedLegacyPlainRow(t, svc, 0, "platform_r179test", "app_secret", plain)

	list, err := svc.List(ctx, 0)
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, it := range list {
		if it.GroupKey != "platform_r179test" || it.ItemKey != "app_secret" {
			continue
		}
		found = true
		if strings.Contains(it.ItemValue, plain) {
			t.Fatalf("legacy plaintext sensitive item must be masked on read, got %q", it.ItemValue)
		}
		if !encrypt.LooksMasked(it.ItemValue) {
			t.Fatalf("expected masked value, got %q", it.ItemValue)
		}
		if !it.IsEncrypted {
			t.Fatal("masked sensitive item must report isEncrypted=true so clients treat it as a secret")
		}
	}
	if !found {
		t.Fatal("seeded row missing from List")
	}
}

// TestListLazilyEncryptsLegacyPlaintextSensitiveRow pins the lazy adoption
// path: reading a legacy plaintext sensitive row rewrites it encrypted at
// rest, and the secret still round-trips through PlainByGroup.
func TestListLazilyEncryptsLegacyPlaintextSensitiveRow(t *testing.T) {
	svc := newSettingsTestSvc(t)
	ctx := context.Background()

	const plain = "legacy-r179-qwen-api-key"
	seedLegacyPlainRow(t, svc, 0, "ai", "qwen_api_key", plain)

	if _, err := svc.List(ctx, 0); err != nil {
		t.Fatal(err)
	}

	var row Setting
	if err := svc.DB.Where("group_key = ? AND item_key = ?", "ai", "qwen_api_key").First(&row).Error; err != nil {
		t.Fatal(err)
	}
	if !row.IsEncrypted {
		t.Fatal("legacy plaintext sensitive row must be adopted as encrypted after read")
	}
	if strings.Contains(row.ItemValue, plain) {
		t.Fatalf("secret still stored in cleartext at rest after read: %q", row.ItemValue)
	}

	out, err := svc.PlainByGroup(ctx, 0, "ai")
	if err != nil {
		t.Fatal(err)
	}
	if out["qwen_api_key"] != plain {
		t.Fatalf("secret must round-trip after lazy encryption, got %q", out["qwen_api_key"])
	}
}

// TestListMasksLegacyPlaintextWithoutEncrypter pins the degraded mode: with
// no APP_MASTER_KEY the read path must still mask registry-listed plaintext
// rows, and must NOT rewrite them (there is no key to encrypt with).
func TestListMasksLegacyPlaintextWithoutEncrypter(t *testing.T) {
	svc := newSettingsTestSvc(t)
	svc.Encrypter = nil
	ctx := context.Background()

	const plain = "legacy-r179-smtp-password"
	seedLegacyPlainRow(t, svc, 0, "email", "smtp_password", plain)

	list, err := svc.List(ctx, 0)
	if err != nil {
		t.Fatal(err)
	}
	for _, it := range list {
		if it.GroupKey == "email" && it.ItemKey == "smtp_password" && strings.Contains(it.ItemValue, plain) {
			t.Fatalf("plaintext sensitive item must be masked even without an encrypter, got %q", it.ItemValue)
		}
	}

	var row Setting
	if err := svc.DB.Where("group_key = ? AND item_key = ?", "email", "smtp_password").First(&row).Error; err != nil {
		t.Fatal(err)
	}
	if row.IsEncrypted || row.ItemValue != plain {
		t.Fatalf("row must stay untouched without an encrypter, got encrypted=%v value=%q", row.IsEncrypted, row.ItemValue)
	}
}

// TestListLeavesNonRegistryPlaintextAlone pins compatibility: plaintext items
// outside the sensitive registry keep their value on read and are not
// rewritten.
func TestListLeavesNonRegistryPlaintextAlone(t *testing.T) {
	svc := newSettingsTestSvc(t)
	ctx := context.Background()

	seedLegacyPlainRow(t, svc, 0, "ai", "deepseek_base_url", "https://api.deepseek.com/v1")

	list, err := svc.List(ctx, 0)
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, it := range list {
		if it.GroupKey == "ai" && it.ItemKey == "deepseek_base_url" {
			found = true
			if it.ItemValue != "https://api.deepseek.com/v1" || it.IsEncrypted {
				t.Fatalf("non-registry plaintext item must be returned as-is, got encrypted=%v value=%q", it.IsEncrypted, it.ItemValue)
			}
		}
	}
	if !found {
		t.Fatal("seeded row missing from List")
	}
	var row Setting
	if err := svc.DB.Where("group_key = ? AND item_key = ?", "ai", "deepseek_base_url").First(&row).Error; err != nil {
		t.Fatal(err)
	}
	if row.IsEncrypted {
		t.Fatal("non-registry row must not be rewritten")
	}
}
