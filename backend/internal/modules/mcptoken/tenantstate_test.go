package mcptoken_test

import (
	"context"
	"errors"
	"testing"

	"github.com/trademind-ai/trademind/backend/internal/modules/mcptoken"
	"gorm.io/gorm"
)

// tenantRow mirrors the columns the tenant-state lookup reads.
type tenantRow struct {
	ID     int64 `gorm:"primaryKey"`
	Status string
}

func (tenantRow) TableName() string { return "tenants" }

func withTenant(t *testing.T, db *gorm.DB, id int64, status string) {
	t.Helper()
	if err := db.AutoMigrate(&tenantRow{}); err != nil {
		t.Fatal(err)
	}
	if err := db.Save(&tenantRow{ID: id, Status: status}).Error; err != nil {
		t.Fatal(err)
	}
}

// A tenant disabled by a platform administrator must lose the token entries as
// well, otherwise a terminated tenant keeps reading data through MCP/Open API.
func TestAuthenticateRejectsDisabledTenant(t *testing.T) {
	db := openTestDB(t)
	withTenant(t, db, 7, "active")
	svc := &mcptoken.Service{DB: db}
	res, err := svc.Create(context.Background(), 7, "both", mcptoken.PurposeBoth, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	for _, entry := range []string{mcptoken.PurposeMCP, mcptoken.PurposeOpenAPI} {
		if _, err := svc.AuthenticateFor(context.Background(), res.Plaintext, entry); err != nil {
			t.Fatalf("active tenant %s: %v", entry, err)
		}
	}

	if err := db.Model(&tenantRow{}).Where("id = ?", 7).Update("status", "disabled").Error; err != nil {
		t.Fatal(err)
	}
	for _, entry := range []string{mcptoken.PurposeMCP, mcptoken.PurposeOpenAPI} {
		if _, err := svc.AuthenticateFor(context.Background(), res.Plaintext, entry); !errors.Is(err, mcptoken.ErrInvalidToken) {
			t.Fatalf("disabled tenant %s: want ErrInvalidToken, got %v", entry, err)
		}
	}
}
