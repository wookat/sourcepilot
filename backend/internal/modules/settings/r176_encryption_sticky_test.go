package settings

import (
	"context"
	"strings"
	"testing"
)

// TestPutBulkCannotDowngradeEncryptedSetting pins the sticky-encryption rule
// (R176): a payload that omits isEncrypted must not turn an already-encrypted
// setting into plaintext at rest, and the value must stay masked on read.
// Before the fix `PUT /api/v1/settings` with `{"itemKey":"deepseek_api_key",
// "itemValue":"sk-..."}` stored the key in cleartext and echoed it back.
func TestPutBulkCannotDowngradeEncryptedSetting(t *testing.T) {
	svc := newSettingsTestSvc(t)
	ctx := context.Background()

	const plain = "sk-r176-audit-secret"
	if err := svc.PutBulk(ctx, []PutItem{
		{GroupKey: "ai", ItemKey: "deepseek_api_key", ItemValue: "sk-initial", IsEncrypted: true},
	}); err != nil {
		t.Fatal(err)
	}

	// Payload without IsEncrypted (e.g. a hand-written API call).
	if err := svc.PutBulk(ctx, []PutItem{
		{GroupKey: "ai", ItemKey: "deepseek_api_key", ItemValue: plain},
	}); err != nil {
		t.Fatal(err)
	}

	var row Setting
	if err := svc.DB.Where("group_key = ? AND item_key = ?", "ai", "deepseek_api_key").First(&row).Error; err != nil {
		t.Fatal(err)
	}
	if !row.IsEncrypted {
		t.Fatal("is_encrypted must stay true: encryption is sticky")
	}
	if strings.Contains(row.ItemValue, plain) {
		t.Fatalf("secret stored in cleartext at rest: %q", row.ItemValue)
	}

	// The new secret must still be usable (decryptable) and masked on read.
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
		if it.ItemKey != "deepseek_api_key" {
			continue
		}
		if it.ItemValue == plain {
			t.Fatal("List must mask the secret, not echo the plaintext")
		}
	}
}
