package order_test

import (
	"strings"
	"testing"

	"github.com/trademind-ai/trademind/backend/internal/modules/carrier"
	"github.com/trademind-ai/trademind/backend/internal/modules/order"
	"gorm.io/gorm"
)

func openCarrierOrderTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	db := openImportTestDB(t)
	if err := db.AutoMigrate(&carrier.Carrier{}); err != nil {
		t.Fatal(err)
	}
	return db
}

func carrierOrderSvc(db *gorm.DB) *order.Service {
	return &order.Service{DB: db, Carriers: &carrier.Service{DB: db}}
}

func paidTestOrder(t *testing.T, svc *order.Service, orderNo string) *order.Order {
	t.Helper()
	c := importTestCtx(1)
	body := importOrderBody(orderNo)
	body.PaymentStatus = order.PaymentPaid
	sum, err := svc.ImportOrders(c, order.ImportBody{Orders: []order.CreateBody{body}}, nil)
	if err != nil || sum.Created != 1 {
		t.Fatalf("import failed: %v %+v", err, sum)
	}
	var o order.Order
	if err := svc.DB.First(&o, "id = ?", *sum.Results[0].OrderID).Error; err != nil {
		t.Fatal(err)
	}
	return &o
}

func TestAppendShipmentWithCarrierCode(t *testing.T) {
	db := openCarrierOrderTestDB(t)
	svc := carrierOrderSvc(db)
	o := paidTestOrder(t, svc, "SO-CARRIER-1")

	c := importTestCtx(1)
	row, err := svc.AppendShipment(c, o.ID, order.OrderShipmentInput{
		CarrierCode: "sf",
		TrackingNo:  "SF1234567890123",
		Status:      order.ShipmentShipped,
	}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if row.CarrierCode != "sf" || row.CarrierID == nil {
		t.Fatalf("expected carrier link, got %+v", row)
	}
	if row.Carrier != "顺丰速运" {
		t.Fatalf("expected carrier name snapshot, got %q", row.Carrier)
	}
	if !strings.Contains(row.TrackingURL, "SF1234567890123") {
		t.Fatalf("expected tracking url from template, got %q", row.TrackingURL)
	}
}

func TestAppendShipmentRejectsBadTrackingNo(t *testing.T) {
	db := openCarrierOrderTestDB(t)
	svc := carrierOrderSvc(db)
	o := paidTestOrder(t, svc, "SO-CARRIER-2")

	c := importTestCtx(1)
	if _, err := svc.AppendShipment(c, o.ID, order.OrderShipmentInput{
		CarrierCode: "sf",
		TrackingNo:  "YT999",
		Status:      order.ShipmentShipped,
	}, nil); err == nil {
		t.Fatal("expected tracking number validation error")
	}
	if _, err := svc.AppendShipment(c, o.ID, order.OrderShipmentInput{
		CarrierCode: "no-such-carrier",
		TrackingNo:  "SF1234567890123",
		Status:      order.ShipmentShipped,
	}, nil); err == nil {
		t.Fatal("expected unknown carrier error")
	}
}

func TestAppendShipmentLegacyFreeTextStillWorks(t *testing.T) {
	db := openCarrierOrderTestDB(t)
	svc := carrierOrderSvc(db)
	o := paidTestOrder(t, svc, "SO-CARRIER-3")

	c := importTestCtx(1)
	row, err := svc.AppendShipment(c, o.ID, order.OrderShipmentInput{
		Carrier:    "某小众物流",
		TrackingNo: "X1",
		Status:     order.ShipmentShipped,
	}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if row.Carrier != "某小众物流" || row.CarrierID != nil || row.CarrierCode != "" {
		t.Fatalf("expected legacy free-text carrier untouched, got %+v", row)
	}
}

func TestBatchShipmentsWithCarrierColumn(t *testing.T) {
	db := openCarrierOrderTestDB(t)
	svc := carrierOrderSvc(db)
	paidTestOrder(t, svc, "SO-BATCH-1")
	paidTestOrder(t, svc, "SO-BATCH-2")
	paidTestOrder(t, svc, "SO-BATCH-3")

	c := importTestCtx(1)
	res, err := svc.BatchShipments(c, order.BatchShipmentsBody{
		DefaultCarrierCode: "zto",
		Items: []order.BatchShipmentItem{
			{OrderNo: "SO-BATCH-1", TrackingNo: "SF1234567890111", CarrierCode: "sf"}, // explicit code
			{OrderNo: "SO-BATCH-2", TrackingNo: "YD123456789012", Carrier: "韵达"},      // free-text mapped
			{OrderNo: "SO-BATCH-3", TrackingNo: "78901234567890"},                     // default carrier
		},
	}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if res.Succeeded != 3 || res.Failed != 0 {
		t.Fatalf("expected 3 ok, got %+v", res)
	}
	var ships []order.OrderShipment
	if err := db.Order("created_at ASC").Find(&ships).Error; err != nil {
		t.Fatal(err)
	}
	codes := map[string]string{}
	for _, sh := range ships {
		codes[sh.TrackingNo] = sh.CarrierCode
	}
	if codes["SF1234567890111"] != "sf" || codes["YD123456789012"] != "yunda" || codes["78901234567890"] != "zto" {
		t.Fatalf("unexpected carrier codes: %+v", codes)
	}
}

func TestBatchShipmentsLegacyTwoColumnsAndLineErrors(t *testing.T) {
	db := openCarrierOrderTestDB(t)
	svc := carrierOrderSvc(db)
	paidTestOrder(t, svc, "SO-BATCH-OLD-1")
	paidTestOrder(t, svc, "SO-BATCH-OLD-2")

	c := importTestCtx(1)
	res, err := svc.BatchShipments(c, order.BatchShipmentsBody{
		Items: []order.BatchShipmentItem{
			{OrderNo: "SO-BATCH-OLD-1", TrackingNo: "TRK-778899"},                     // legacy two-column
			{OrderNo: "SO-BATCH-OLD-2", TrackingNo: "YT999", CarrierCode: "sf"},       // bad waybill → line fails
			{OrderNo: "SO-MISSING", TrackingNo: "TRK-1", CarrierCode: "no-such-code"}, // missing order → line fails
		},
	}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if res.Succeeded != 1 || res.Failed != 2 {
		t.Fatalf("expected 1 ok / 2 failed, got %+v", res)
	}
	var sh order.OrderShipment
	if err := db.First(&sh, "tracking_no = ?", "TRK-778899").Error; err != nil {
		t.Fatal(err)
	}
	if sh.Carrier != "其他快递" || sh.CarrierCode != "" {
		t.Fatalf("expected legacy default carrier, got %+v", sh)
	}
}

func TestManualTrackingUpdateAdvancesOrder(t *testing.T) {
	db := openCarrierOrderTestDB(t)
	svc := carrierOrderSvc(db)
	o := paidTestOrder(t, svc, "SO-TRACK-1")

	c := importTestCtx(1)
	row, err := svc.AppendShipment(c, o.ID, order.OrderShipmentInput{
		CarrierCode: "sf", TrackingNo: "SF1234567890123", Status: order.ShipmentShipped,
	}, nil)
	if err != nil {
		t.Fatal(err)
	}
	// Manual provider cannot fetch: refresh is a safe no-op.
	if _, err := svc.RefreshShipmentTracking(c, o.ID, row.ID, nil); err != nil {
		t.Fatal(err)
	}
	// Manual status edit still drives the order lifecycle.
	if _, err := svc.PatchShipment(c, o.ID, row.ID, order.OrderShipmentInput{
		Carrier: row.Carrier, TrackingNo: row.TrackingNo, Status: order.ShipmentDelivered,
	}, nil); err != nil {
		t.Fatal(err)
	}
	var got order.Order
	if err := db.First(&got, "id = ?", o.ID).Error; err != nil {
		t.Fatal(err)
	}
	if got.Status != order.StatusDelivered {
		t.Fatalf("expected order delivered, got %s", got.Status)
	}
	if got.DeliveredAt == nil {
		t.Fatal("expected deliveredAt filled")
	}
}
