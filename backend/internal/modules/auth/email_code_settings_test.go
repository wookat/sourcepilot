package auth

import (
	"context"
	"errors"
	"testing"

	"github.com/trademind-ai/trademind/backend/internal/modules/settings"
)

// Missing SMTP configuration must be detected before the registration lookup
// so that registered and unregistered addresses get the same response
// (anti-enumeration): both must see 503 when mail is unconfigured.
func TestCheckEmailSettingsIncompleteWithoutSMTP(t *testing.T) {
	db := newTenantStateTestDB(t)
	if err := db.AutoMigrate(&settings.Setting{}); err != nil {
		t.Fatal(err)
	}
	h := &Handler{Settings: &settings.Service{DB: db}}

	if err := h.checkEmailSettings(context.Background()); !errors.Is(err, errEmailSettingsIncomplete) {
		t.Fatalf("expected errEmailSettingsIncomplete without SMTP config, got %v", err)
	}

	if err := db.Exec(`INSERT INTO settings (tenant_id, group_key, item_key, item_value, is_encrypted)
		VALUES (0, 'mail', 'smtp_host', 'smtp.example.com', 0),
		       (0, 'mail', 'smtp_from', 'noreply@example.com', 0)`).Error; err != nil {
		t.Fatal(err)
	}
	if err := h.checkEmailSettings(context.Background()); err != nil {
		t.Fatalf("expected usable mail settings, got %v", err)
	}
}
