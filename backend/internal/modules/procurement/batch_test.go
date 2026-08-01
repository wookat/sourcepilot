package procurement

import (
	"context"
	"testing"

	"github.com/google/uuid"
)

// advanceToPlacing moves a fresh draft purchase order to placing.
func advanceToPlacing(t *testing.T, f fixture, po PurchaseOrder) PurchaseOrder {
	t.Helper()
	ctx := context.Background()
	if _, err := f.svc.Submit(ctx, po.ID, nil); err != nil {
		t.Fatalf("submit: %v", err)
	}
	got, err := f.svc.Confirm(ctx, po.ID, nil)
	if err != nil {
		t.Fatalf("confirm: %v", err)
	}
	return *got
}

func TestBatchMarkPlacedPartialSuccess(t *testing.T) {
	f := setupFixture(t)
	po := generate(t, f, "batch-placed").Orders[0]
	advanceToPlacing(t, f, po)
	ctx := context.Background()

	res, err := f.svc.BatchMarkPlaced(ctx, BatchMarkPlacedBody{Items: []BatchPlacedItem{
		{PurchaseOrderID: po.ID.String(), ExternalOrderID: "1688-B-1"},
		{PurchaseOrderID: uuid.NewString(), ExternalOrderID: "1688-B-2"}, // not found
		{PurchaseOrderID: "not-a-uuid", ExternalOrderID: "1688-B-3"},
		{PurchaseOrderID: po.ID.String(), ExternalOrderID: "1688-B-4"}, // duplicate line
	}}, nil)
	if err != nil {
		t.Fatalf("batch: %v", err)
	}
	if res.Succeeded != 1 || res.Failed != 3 {
		t.Fatalf("expected 1 ok / 3 failed, got %+v", res)
	}
	if !res.Results[0].OK || res.Results[0].Status != StatusPlaced {
		t.Fatalf("unexpected first line %+v", res.Results[0])
	}
	got, err := f.svc.Detail(ctx, po.ID)
	if err != nil || got.ExternalOrderID != "1688-B-1" {
		t.Fatalf("external order id not backfilled: %v %+v", err, got)
	}
}

func TestBatchMarkPlacedRejectsEmpty(t *testing.T) {
	f := setupFixture(t)
	if _, err := f.svc.BatchMarkPlaced(context.Background(), BatchMarkPlacedBody{}, nil); err == nil {
		t.Fatalf("empty batch must be rejected")
	}
}

func TestBatchFillLogisticsMatchesByExternalOrderID(t *testing.T) {
	f := setupFixture(t)
	po := generate(t, f, "batch-logi").Orders[0]
	advanceToPlacing(t, f, po)
	ctx := context.Background()
	if _, err := f.svc.MarkPlaced(ctx, po.ID, MarkPlacedBody{ExternalOrderID: "1688-L-1"}, nil); err != nil {
		t.Fatalf("mark placed: %v", err)
	}

	res, err := f.svc.BatchFillLogistics(ctx, BatchLogisticsBody{Items: []BatchLogisticsItem{
		{ExternalOrderID: "1688-L-1", TrackingNo: "SF-888", Carrier: "顺丰"},
		{ExternalOrderID: "1688-UNKNOWN", TrackingNo: "YT-1"},
		{ExternalOrderID: "1688-L-1", TrackingNo: "SF-999"}, // duplicate
	}}, nil)
	if err != nil {
		t.Fatalf("batch: %v", err)
	}
	if res.Succeeded != 1 || res.Failed != 2 {
		t.Fatalf("expected 1 ok / 2 failed, got %+v", res)
	}
	// placed(unpaid) line was auto marked paid then shipped
	got, err := f.svc.Detail(ctx, po.ID)
	if err != nil {
		t.Fatal(err)
	}
	if got.Status != StatusShipped || got.PayStatus != PayStatusPaid {
		t.Fatalf("expected shipped+paid, got %+v", got)
	}
	if len(got.Logistics) != 1 || got.Logistics[0].TrackingNo != "SF-888" {
		t.Fatalf("unexpected logistics %+v", got.Logistics)
	}
}
