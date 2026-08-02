package settings

import (
	"context"
	"path/filepath"
	"strings"
	"testing"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"

	"github.com/trademind-ai/trademind/backend/internal/encrypt"
)

func newSettingsTestSvc(t *testing.T) *Service {
	t.Helper()
	dsn := filepath.Join(t.TempDir(), "settings.db")
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Skipf("sqlite unavailable: %v", err)
	}
	if err := db.AutoMigrate(&Setting{}); err != nil {
		t.Fatal(err)
	}
	enc, err := encrypt.NewService(strings.Repeat("k", 32))
	if err != nil {
		t.Fatal(err)
	}
	return &Service{DB: db, Encrypter: enc}
}

func TestPutBulkEmptyEncryptedKeepsSecret(t *testing.T) {
	svc := newSettingsTestSvc(t)
	ctx := context.Background()
	if err := svc.PutBulk(ctx, []PutItem{{GroupKey: "ai", ItemKey: "openai_api_key", ItemValue: "sk-secret", IsEncrypted: true}}); err != nil {
		t.Fatal(err)
	}
	if err := svc.PutBulk(ctx, []PutItem{{GroupKey: "ai", ItemKey: "openai_api_key", ItemValue: "", IsEncrypted: true}}); err != nil {
		t.Fatal(err)
	}
	var row Setting
	if err := svc.DB.Where("group_key = ? AND item_key = ?", "ai", "openai_api_key").First(&row).Error; err != nil {
		t.Fatal(err)
	}
	if strings.TrimSpace(row.ItemValue) == "" {
		t.Fatal("empty encrypted payload must keep stored secret")
	}
}

func TestPutBulkClearEmptiesEncryptedSecret(t *testing.T) {
	svc := newSettingsTestSvc(t)
	ctx := context.Background()
	if err := svc.PutBulk(ctx, []PutItem{
		{GroupKey: "ai", ItemKey: "openai_api_key", ItemValue: "sk-secret", IsEncrypted: true},
		{GroupKey: "ai", ItemKey: "openai_model", ItemValue: "gpt-4o-mini"},
	}); err != nil {
		t.Fatal(err)
	}
	if err := svc.PutBulk(ctx, []PutItem{
		{GroupKey: "ai", ItemKey: "openai_api_key", IsEncrypted: true, Clear: true},
		{GroupKey: "ai", ItemKey: "openai_model", Clear: true},
	}); err != nil {
		t.Fatal(err)
	}
	var rows []Setting
	if err := svc.DB.Where("group_key = ?", "ai").Find(&rows).Error; err != nil {
		t.Fatal(err)
	}
	for _, row := range rows {
		if row.ItemValue != "" {
			t.Fatalf("clear must empty stored value, got %q for %s", row.ItemValue, row.ItemKey)
		}
	}
}

func TestPutBulkClearOnMissingRowIsNoopCreate(t *testing.T) {
	svc := newSettingsTestSvc(t)
	if err := svc.PutBulk(context.Background(), []PutItem{
		{GroupKey: "ai", ItemKey: "openai_api_key", ItemValue: "sk-should-be-ignored", IsEncrypted: true, Clear: true},
	}); err != nil {
		t.Fatal(err)
	}
	var row Setting
	if err := svc.DB.Where("group_key = ? AND item_key = ?", "ai", "openai_api_key").First(&row).Error; err != nil {
		t.Fatal(err)
	}
	if row.ItemValue != "" {
		t.Fatalf("clear must store empty value, got %q", row.ItemValue)
	}
}
