package settings

import (
	"context"
	"strings"
	"testing"
)

// R180（承接 R179 线2 遗留）：Shopee partner_key 必须在敏感注册表内 —— 即使
// 平台 provider bootstrap 尚未运行（静态种子兜底），写入也必须强制加密、读取
// 必须脱敏。
func TestShopeePartnerKeyInSensitiveRegistry(t *testing.T) {
	if !IsSensitiveKey("platform_shopee", "partner_key") {
		t.Fatal("platform_shopee/partner_key must be registered sensitive")
	}
	// Case-insensitive match.
	if !IsSensitiveKey("Platform_Shopee", "Partner_Key") {
		t.Fatal("sensitive registry must match case-insensitively")
	}
}

func TestShopeePartnerKeyEncryptedAtRest(t *testing.T) {
	svc := newSettingsTestSvc(t)
	ctx := context.Background()

	const plain = "shopee-partner-key-r180"
	if err := svc.PutBulk(ctx, []PutItem{
		{GroupKey: "platform_shopee", ItemKey: "partner_key", ItemValue: plain, IsEncrypted: false},
	}); err != nil {
		t.Fatal(err)
	}

	var row Setting
	if err := svc.DB.Where("group_key = ? AND item_key = ?", "platform_shopee", "partner_key").First(&row).Error; err != nil {
		t.Fatal(err)
	}
	if !row.IsEncrypted {
		t.Fatal("partner_key must be stored encrypted regardless of client flag")
	}
	if strings.Contains(row.ItemValue, plain) {
		t.Fatalf("partner_key stored in cleartext at rest: %q", row.ItemValue)
	}

	list, err := svc.List(ctx, 0)
	if err != nil {
		t.Fatal(err)
	}
	for _, it := range list {
		if it.GroupKey == "platform_shopee" && it.ItemKey == "partner_key" && it.ItemValue == plain {
			t.Fatal("List must mask partner_key, not echo the plaintext")
		}
	}
}
