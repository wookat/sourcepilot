package migrationimport_test

import (
	"fmt"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"github.com/google/uuid"
	"gorm.io/gorm"

	"github.com/trademind-ai/trademind/backend/internal/modules/migrationimport"
	"github.com/trademind-ai/trademind/backend/internal/modules/order"
	"github.com/trademind-ai/trademind/backend/internal/modules/product"
	"github.com/trademind-ai/trademind/backend/internal/modules/productpublish"
	"github.com/trademind-ai/trademind/backend/internal/modules/shop"
	"github.com/trademind-ai/trademind/backend/internal/pkg/adminperm"
	"github.com/trademind-ai/trademind/backend/internal/pkg/ctxkey"
)

func openTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	dsn := fmt.Sprintf("file:migration_import_%s?mode=memory&cache=shared&_pragma=busy_timeout(5000)", uuid.New().String())
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Skipf("sqlite unavailable: %v", err)
	}
	sqlDB, _ := db.DB()
	sqlDB.SetMaxOpenConns(1)
	if err := db.AutoMigrate(
		&migrationimport.ImportJob{}, &migrationimport.ImportJobRow{},
		&product.Product{}, &product.ProductSKU{}, &product.ProductImage{}, &product.ProductPlatformPublishConfig{}, &productpublish.ProductPublication{},
		&order.Order{}, &order.OrderItem{}, &order.OrderShipment{},
		&order.OrderReviewRule{}, &order.OrderReviewHit{},
		&shop.Shop{},
	); err != nil {
		t.Fatal(err)
	}
	return db
}

func testCtx(tenantID int64) *gin.Context {
	gin.SetMode(gin.TestMode)
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Request = httptest.NewRequest("POST", "/api/v1/imports/commit", nil)
	c.Set(ctxkey.TenantID, tenantID)
	return c
}

func newSvc(db *gorm.DB) *migrationimport.Service {
	return &migrationimport.Service{
		DB:       db,
		Products: &product.Service{DB: db},
		Orders:   &order.Service{DB: db},
	}
}

func seedShop(t *testing.T, db *gorm.DB, tenantID int64) uuid.UUID {
	t.Helper()
	s := &shop.Shop{TenantID: tenantID, Platform: "tiktok", ShopName: "测试店铺"}
	if err := db.Create(s).Error; err != nil {
		t.Fatal(err)
	}
	return s.ID
}

func productBody(shopID uuid.UUID) migrationimport.WizardBody {
	return migrationimport.WizardBody{
		Kind:    migrationimport.KindProduct,
		ShopID:  shopID.String(),
		Columns: []string{"商品名称", "SKU", "规格", "售价", "库存"},
		Rows: [][]string{
			{"迁移商品A", "MIG-A-1", "红色", "9.90", "10"},
			{"迁移商品A", "MIG-A-2", "蓝色", "10.90", "5"},
			{"迁移商品B", "MIG-B-1", "", "abc", "3"}, // invalid price
		},
		Mapping: map[string]int{
			"title": 0, "skuCode": 1, "skuName": 2, "price": 3, "stock": 4,
		},
		FileName:     "products.csv",
		FileHash:     "hash-product-1",
		SourceFormat: migrationimport.SourceDianxiaomi,
	}
}

func TestValidateProducts(t *testing.T) {
	db := openTestDB(t)
	svc := newSvc(db)
	c := testCtx(1)
	shopID := seedShop(t, db, 1)

	out, err := svc.Validate(c, productBody(shopID))
	if err != nil {
		t.Fatal(err)
	}
	if out.TotalRows != 3 || out.ValidRows != 2 || out.ErrorRows != 1 {
		t.Fatalf("unexpected: %+v", out)
	}
	if out.Errors[0].RowNumber != 3 || out.Errors[0].Field != "price" {
		t.Fatalf("error row: %+v", out.Errors[0])
	}
}

func TestCommitProductsAndIdempotency(t *testing.T) {
	db := openTestDB(t)
	svc := newSvc(db)
	c := testCtx(1)
	shopID := seedShop(t, db, 1)

	out, err := svc.Commit(c, productBody(shopID), nil)
	if err != nil {
		t.Fatal(err)
	}
	if out.SuccessRows != 2 || out.FailedRows != 1 || out.Replayed {
		t.Fatalf("commit: %+v", out)
	}
	if out.Status != migrationimport.JobStatusPartialSuccess {
		t.Fatalf("status: %s", out.Status)
	}
	var productCount, skuCount int64
	db.Model(&product.Product{}).Count(&productCount)
	db.Model(&product.ProductSKU{}).Count(&skuCount)
	if productCount != 1 || skuCount != 2 {
		t.Fatalf("products=%d skus=%d", productCount, skuCount)
	}
	var p product.Product
	if err := db.First(&p).Error; err != nil {
		t.Fatal(err)
	}
	if p.Status != product.StatusDraft {
		t.Fatalf("expected draft, got %s", p.Status)
	}

	// Same batch re-upload replays the original result without inserting.
	out2, err := svc.Commit(testCtx(1), productBody(shopID), nil)
	if err != nil {
		t.Fatal(err)
	}
	if !out2.Replayed || out2.JobID != out.JobID {
		t.Fatalf("replay: %+v", out2)
	}
	db.Model(&product.Product{}).Count(&productCount)
	if productCount != 1 {
		t.Fatalf("replay inserted products: %d", productCount)
	}

	// A new batch skips already-existing SKUs as duplicates.
	b := productBody(shopID)
	b.FileHash = "hash-product-2"
	out3, err := svc.Commit(testCtx(1), b, nil)
	if err != nil {
		t.Fatal(err)
	}
	if out3.DuplicateRows != 2 || out3.SuccessRows != 0 {
		t.Fatalf("dup batch: %+v", out3)
	}
}

func orderBody(shopID uuid.UUID) migrationimport.WizardBody {
	return migrationimport.WizardBody{
		Kind:    migrationimport.KindOrder,
		ShopID:  shopID.String(),
		Columns: []string{"订单号", "收件人", "商品名称", "数量", "单价", "订单状态", "国家"},
		Rows: [][]string{
			{"SO-1001", "张三", "商品甲", "2", "9.9", "已发货", "美国"},
			{"SO-1001", "张三", "商品乙", "1", "5", "已发货", "美国"},
			{"SO-1002", "李四", "商品丙", "1", "20", "未知状态", "英国"},
			{"SO-1003", "王五", "商品丁", "x", "1", "已付款", "德国"},
		},
		Mapping: map[string]int{
			"orderNo": 0, "customerName": 1, "productTitle": 2, "quantity": 3,
			"unitPrice": 4, "status": 5, "country": 6,
		},
		FileName:     "orders.csv",
		FileHash:     "hash-order-1",
		SourceFormat: migrationimport.SourceMabang,
	}
}

func TestCommitOrders(t *testing.T) {
	db := openTestDB(t)
	svc := newSvc(db)
	shopID := seedShop(t, db, 1)

	out, err := svc.Commit(testCtx(1), orderBody(shopID), nil)
	if err != nil {
		t.Fatal(err)
	}
	// SO-1001 (2 rows) imported; unknown status + bad quantity rows fail.
	if out.SuccessRows != 2 || out.FailedRows != 2 {
		t.Fatalf("commit: %+v", out)
	}
	var o order.Order
	if err := db.First(&o, "order_no = ?", "SO-1001").Error; err != nil {
		t.Fatal(err)
	}
	if o.Status != order.StatusShipped || o.PaymentStatus != order.PaymentPaid {
		t.Fatalf("status mapping: %s / %s", o.Status, o.PaymentStatus)
	}
	var itemCount int64
	db.Model(&order.OrderItem{}).Count(&itemCount)
	if itemCount != 2 {
		t.Fatalf("items: %d", itemCount)
	}
	var badCount int64
	db.Model(&order.Order{}).Where("order_no IN ?", []string{"SO-1002", "SO-1003"}).Count(&badCount)
	if badCount != 0 {
		t.Fatal("invalid rows must not be inserted")
	}

	// Error rows are persisted for the downloadable report.
	job, rows, err := svc.GetJob(testCtx(1), out.JobID)
	if err != nil {
		t.Fatal(err)
	}
	if job.FailedRows != 2 || len(rows) != 2 {
		t.Fatalf("job rows: %+v %d", job, len(rows))
	}

	// Re-import of an existing order no is a duplicate skip in a new batch.
	b := orderBody(shopID)
	b.FileHash = "hash-order-2"
	b.Rows = b.Rows[:2]
	out2, err := svc.Commit(testCtx(1), b, nil)
	if err != nil {
		t.Fatal(err)
	}
	if out2.DuplicateRows != 2 || out2.SuccessRows != 0 {
		t.Fatalf("dup: %+v", out2)
	}
}

func TestCommitRequiresShop(t *testing.T) {
	db := openTestDB(t)
	svc := newSvc(db)
	b := productBody(uuid.New())
	b.ShopID = ""
	if _, err := svc.Commit(testCtx(1), b, nil); err == nil {
		t.Fatal("expected shop required error")
	}
}

func TestOperatorScope(t *testing.T) {
	db := openTestDB(t)
	svc := newSvc(db)
	shopA := seedShop(t, db, 1)
	shopB := seedShop(t, db, 1)

	c := testCtx(1)
	c.Set("adminperm.principal", &adminperm.Principal{
		Role:        adminperm.RoleOperator,
		Permissions: adminperm.PermissionsForRole(adminperm.RoleOperator),
		StoreGrants: []adminperm.StoreGrant{{StoreID: shopA, PermissionScope: "operate"}},
	})
	b := productBody(shopB)
	b.FileHash = "hash-scope-1"
	if _, err := svc.Commit(c, b, nil); err == nil {
		t.Fatal("expected operator scope rejection for unauthorized shop")
	}

	c2 := testCtx(1)
	c2.Set("adminperm.principal", &adminperm.Principal{
		Role:        adminperm.RoleOperator,
		Permissions: adminperm.PermissionsForRole(adminperm.RoleOperator),
		StoreGrants: []adminperm.StoreGrant{{StoreID: shopA, PermissionScope: "operate"}},
	})
	b2 := productBody(shopA)
	b2.FileHash = "hash-scope-2"
	out, err := svc.Commit(c2, b2, nil)
	if err != nil {
		t.Fatal(err)
	}
	if out.SuccessRows != 2 {
		t.Fatalf("authorized shop commit: %+v", out)
	}
}

func TestListJobs(t *testing.T) {
	db := openTestDB(t)
	svc := newSvc(db)
	shopID := seedShop(t, db, 1)
	if _, err := svc.Commit(testCtx(1), productBody(shopID), nil); err != nil {
		t.Fatal(err)
	}
	jobs, total, err := svc.ListJobs(testCtx(1), "", 1, 20)
	if err != nil {
		t.Fatal(err)
	}
	if total != 1 || len(jobs) != 1 || jobs[0].Kind != migrationimport.KindProduct {
		t.Fatalf("list: total=%d jobs=%+v", total, jobs)
	}
	// Other tenant sees nothing.
	_, total2, err := svc.ListJobs(testCtx(2), "", 1, 20)
	if err != nil {
		t.Fatal(err)
	}
	if total2 != 0 {
		t.Fatalf("tenant isolation: %d", total2)
	}
}
