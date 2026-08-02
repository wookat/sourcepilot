package adminperm

import (
	"fmt"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"github.com/google/uuid"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

type scopedAuditRow struct {
	ID     int64      `gorm:"primaryKey;autoIncrement"`
	ShopID *uuid.UUID `gorm:"type:char(36)"`
	Label  string
}

func newScopeTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(fmt.Sprintf("file:%s?mode=memory&cache=shared", uuid.NewString())), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	if err != nil {
		t.Skipf("sqlite unavailable: %v", err)
	}
	if err := db.AutoMigrate(&scopedAuditRow{}); err != nil {
		t.Fatal(err)
	}
	return db
}

func newScopeTestContext(t *testing.T, p *Principal) *gin.Context {
	t.Helper()
	gin.SetMode(gin.TestMode)
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Request = httptest.NewRequest("GET", "/", nil)
	c.Set(ctxPrincipalKey, p)
	return c
}

func TestApplyStoreScopeOrNull(t *testing.T) {
	db := newScopeTestDB(t)
	shopA := uuid.New()
	shopB := uuid.New()
	rows := []scopedAuditRow{
		{ShopID: nil, Label: "tenant-level"},
		{ShopID: &shopA, Label: "shop-a"},
		{ShopID: &shopB, Label: "shop-b"},
	}
	if err := db.Create(&rows).Error; err != nil {
		t.Fatal(err)
	}

	cases := []struct {
		name      string
		principal *Principal
		want      int64
	}{
		{
			name:      "admin sees all rows",
			principal: &Principal{Role: RoleAdmin},
			want:      3,
		},
		{
			name:      "operator without grants sees tenant-level rows",
			principal: &Principal{Role: RoleOperator},
			want:      1,
		},
		{
			name: "operator with grant sees tenant-level plus granted shop",
			principal: &Principal{
				Role:        RoleOperator,
				StoreGrants: []StoreGrant{{StoreID: shopA, PermissionScope: "view"}},
			},
			want: 2,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			c := newScopeTestContext(t, tc.principal)
			tx := db.Model(&scopedAuditRow{})
			scoped, err := ApplyStoreScopeOrNull(c, db, tx, "shop_id")
			if err != nil {
				t.Fatal(err)
			}
			var count int64
			if err := scoped.Count(&count).Error; err != nil {
				t.Fatal(err)
			}
			if count != tc.want {
				t.Fatalf("got %d rows, want %d", count, tc.want)
			}
		})
	}
}

func TestApplyStoreScopeStillEmptyForBusinessData(t *testing.T) {
	db := newScopeTestDB(t)
	shopA := uuid.New()
	rows := []scopedAuditRow{
		{ShopID: nil, Label: "tenant-level"},
		{ShopID: &shopA, Label: "shop-a"},
	}
	if err := db.Create(&rows).Error; err != nil {
		t.Fatal(err)
	}
	c := newScopeTestContext(t, &Principal{Role: RoleOperator})
	scoped, err := ApplyStoreScope(c, db, db.Model(&scopedAuditRow{}), "shop_id")
	if err != nil {
		t.Fatal(err)
	}
	var count int64
	if err := scoped.Count(&count).Error; err != nil {
		t.Fatal(err)
	}
	if count != 0 {
		t.Fatalf("business scope with no grants should stay empty, got %d", count)
	}
}
