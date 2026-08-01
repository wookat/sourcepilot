package order_test

import (
	"testing"

	"github.com/trademind-ai/trademind/backend/internal/modules/order"
)

func TestBatchShipmentsFlow(t *testing.T) {
	db := openImportTestDB(t)
	svc := &order.Service{DB: db}
	paid := createFlowTestOrder(t, svc, "SO-BS-PAID")
	if err := db.Model(&order.Order{}).Where("id = ?", paid.ID).
		Update("payment_status", order.PaymentPaid).Error; err != nil {
		t.Fatal(err)
	}
	createFlowTestOrder(t, svc, "SO-BS-UNPAID")

	c := importTestCtx(1)
	res, err := svc.BatchShipments(c, order.BatchShipmentsBody{Items: []order.BatchShipmentItem{
		{OrderNo: "SO-BS-PAID", TrackingNo: "TRK-BS-1", Carrier: "云仓快递"},
		{OrderNo: "SO-BS-UNPAID", TrackingNo: "TRK-BS-2"},
		{OrderNo: "SO-BS-MISSING", TrackingNo: "TRK-BS-3"},
		{OrderNo: "SO-BS-PAID", TrackingNo: "TRK-BS-DUP"},
		{OrderNo: "", TrackingNo: "TRK-BS-4"},
	}}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if res.Succeeded != 1 || res.Failed != 4 {
		t.Fatalf("expected 1 ok / 4 failed, got %d/%d: %+v", res.Succeeded, res.Failed, res.Results)
	}
	if !res.Results[0].OK || res.Results[0].Status != order.StatusShipped {
		t.Fatalf("paid order line should ship, got %+v", res.Results[0])
	}
	for i, wantMsg := range map[int]string{
		1: "订单未付款，不能发货",
		2: "没有找到该订单号对应的销售订单",
		3: "重复的订单号，已跳过",
		4: "订单号与快递单号均不能为空",
	} {
		if res.Results[i].OK || res.Results[i].Message != wantMsg {
			t.Fatalf("line %d expected %q, got %+v", i, wantMsg, res.Results[i])
		}
	}
	var got order.Order
	if err := db.First(&got, "id = ?", paid.ID).Error; err != nil {
		t.Fatal(err)
	}
	if got.Status != order.StatusShipped || got.FulfillmentStatus != order.FulfillmentFulfilled {
		t.Fatalf("expected shipped/fulfilled, got %s/%s", got.Status, got.FulfillmentStatus)
	}
	var count int64
	if err := db.Model(&order.OrderShipment{}).Where("order_id = ?", paid.ID).Count(&count).Error; err != nil {
		t.Fatal(err)
	}
	if count != 1 {
		t.Fatalf("expected 1 shipment, got %d", count)
	}

	if _, err := svc.BatchShipments(c, order.BatchShipmentsBody{}, nil); err == nil {
		t.Fatalf("empty items must error")
	}
}
