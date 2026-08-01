package order_test

import (
	"testing"

	"github.com/trademind-ai/trademind/backend/internal/modules/order"
)

func createFlowTestOrder(t *testing.T, svc *order.Service, orderNo string) *order.Order {
	t.Helper()
	c := importTestCtx(1)
	sum, err := svc.ImportOrders(c, order.ImportBody{Orders: []order.CreateBody{importOrderBody(orderNo)}}, nil)
	if err != nil || sum.Created != 1 {
		t.Fatalf("import failed: %v %+v", err, sum)
	}
	var o order.Order
	if err := svc.DB.First(&o, "id = ?", *sum.Results[0].OrderID).Error; err != nil {
		t.Fatal(err)
	}
	return &o
}

func TestAppendShipmentAdvancesOrderToShipped(t *testing.T) {
	db := openImportTestDB(t)
	svc := &order.Service{DB: db}
	o := createFlowTestOrder(t, svc, "SO-SHIP-1")

	c := importTestCtx(1)
	row, err := svc.AppendShipment(c, o.ID, order.OrderShipmentInput{
		Carrier:    "云仓快递",
		TrackingNo: "TRK-001",
		Status:     order.ShipmentShipped,
	}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if row.ShippedAt == nil {
		t.Fatalf("expected shipment shippedAt auto-filled, got %+v", row)
	}
	var got order.Order
	if err := db.First(&got, "id = ?", o.ID).Error; err != nil {
		t.Fatal(err)
	}
	if got.Status != order.StatusShipped || got.FulfillmentStatus != order.FulfillmentFulfilled {
		t.Fatalf("expected order advanced to shipped/fulfilled, got %s/%s", got.Status, got.FulfillmentStatus)
	}
	if got.ShippedAt == nil {
		t.Fatalf("expected order shippedAt filled")
	}
}

func TestPatchShipmentDeliveredAdvancesOrder(t *testing.T) {
	db := openImportTestDB(t)
	svc := &order.Service{DB: db}
	o := createFlowTestOrder(t, svc, "SO-SHIP-2")

	c := importTestCtx(1)
	row, err := svc.AppendShipment(c, o.ID, order.OrderShipmentInput{Carrier: "云仓快递", Status: order.ShipmentShipped}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := svc.PatchShipment(c, o.ID, row.ID, order.OrderShipmentInput{
		Carrier: "云仓快递",
		Status:  order.ShipmentDelivered,
	}, nil); err != nil {
		t.Fatal(err)
	}
	var got order.Order
	if err := db.First(&got, "id = ?", o.ID).Error; err != nil {
		t.Fatal(err)
	}
	if got.Status != order.StatusDelivered || got.DeliveredAt == nil {
		t.Fatalf("expected order delivered with deliveredAt, got %s deliveredAt=%v", got.Status, got.DeliveredAt)
	}
}

func TestPendingShipmentDoesNotRegressOrder(t *testing.T) {
	db := openImportTestDB(t)
	svc := &order.Service{DB: db}
	o := createFlowTestOrder(t, svc, "SO-SHIP-3")
	if err := db.Model(&order.Order{}).Where("id = ?", o.ID).
		Updates(map[string]any{"status": order.StatusDelivered, "fulfillment_status": order.FulfillmentFulfilled}).Error; err != nil {
		t.Fatal(err)
	}

	c := importTestCtx(1)
	if _, err := svc.AppendShipment(c, o.ID, order.OrderShipmentInput{Carrier: "云仓快递", Status: order.ShipmentPending}, nil); err != nil {
		t.Fatal(err)
	}
	var got order.Order
	if err := db.First(&got, "id = ?", o.ID).Error; err != nil {
		t.Fatal(err)
	}
	if got.Status != order.StatusDelivered {
		t.Fatalf("expected order status unchanged (delivered), got %s", got.Status)
	}
}
