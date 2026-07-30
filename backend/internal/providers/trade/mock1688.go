package trade

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"
)

// ErrOrderNotFound is returned when the mock has no such external order.
var ErrOrderNotFound = errors.New("trade: order not found")

type mockOrder struct {
	detail    OrderDetail
	pay       PayStatus
	logistics LogisticsInfo
}

// Mock1688 is an in-memory state machine standing in for the 1688 open
// platform. CreateOrder returns Manual=true: in the current transition mode
// the operator places the order on 1688 by hand; RegisterManualOrder lets the
// backfilled external order id join the mock state machine so pay/logistics
// polling still works in demos and tests.
type Mock1688 struct {
	mu     sync.Mutex
	seq    int
	orders map[string]*mockOrder
	byKey  map[string]string // idempotency key → external order id
}

// NewMock1688 builds an empty mock.
func NewMock1688() *Mock1688 {
	return &Mock1688{orders: map[string]*mockOrder{}, byKey: map[string]string{}}
}

// Platform implements Provider.
func (m *Mock1688) Platform() string { return "1688" }

// PreviewOrder implements Provider: accepts all lines, flags nothing.
func (m *Mock1688) PreviewOrder(_ context.Context, req PreviewRequest) (*PreviewResult, error) {
	out := &PreviewResult{OK: true}
	for _, it := range req.Items {
		out.TotalAmount += it.ExpectedPrice * float64(it.Quantity)
		out.Lines = append(out.Lines, PreviewLine{
			OfferID:       it.OfferID,
			ExternalSKUID: it.ExternalSKUID,
			CurrentPrice:  it.ExpectedPrice,
			InStock:       true,
		})
	}
	return out, nil
}

// CreateOrder implements Provider. Manual mode: no real order is placed;
// the caller must export the purchase list and order by hand.
func (m *Mock1688) CreateOrder(_ context.Context, req CreateOrderRequest) (*CreateOrderResult, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if id, ok := m.byKey[req.IdempotencyKey]; ok && req.IdempotencyKey != "" {
		o := m.orders[id]
		return &CreateOrderResult{ExternalOrderID: id, TotalAmount: o.detail.TotalAmount, Manual: true}, nil
	}
	total := 0.0
	for _, it := range req.Items {
		total += it.ExpectedPrice * float64(it.Quantity)
	}
	return &CreateOrderResult{ExternalOrderID: "", TotalAmount: total, Manual: true}, nil
}

// RegisterManualOrder records a manually placed 1688 order so subsequent
// GetOrder/GetPayStatus/GetLogistics calls resolve. Idempotent per id.
func (m *Mock1688) RegisterManualOrder(externalOrderID string, totalAmount float64) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if _, ok := m.orders[externalOrderID]; ok {
		return
	}
	m.seq++
	m.orders[externalOrderID] = &mockOrder{
		detail: OrderDetail{ExternalOrderID: externalOrderID, Status: "created", TotalAmount: totalAmount},
		pay:    PayStatus{ExternalOrderID: externalOrderID, Status: "unpaid"},
		logistics: LogisticsInfo{
			ExternalOrderID: externalOrderID,
			Status:          "pending",
		},
	}
}

// AdvancePay marks the mock order paid (simulates supplier-side payment).
func (m *Mock1688) AdvancePay(externalOrderID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	o, ok := m.orders[externalOrderID]
	if !ok {
		return ErrOrderNotFound
	}
	now := time.Now().UTC()
	o.pay.Status = "paid"
	o.pay.PaidAt = &now
	o.detail.Status = "paid"
	return nil
}

// AdvanceShip attaches tracking info and marks the mock order shipped.
func (m *Mock1688) AdvanceShip(externalOrderID, carrier, trackingNo string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	o, ok := m.orders[externalOrderID]
	if !ok {
		return ErrOrderNotFound
	}
	if trackingNo == "" {
		m.seq++
		trackingNo = fmt.Sprintf("MOCK-TRK-%06d", m.seq)
	}
	if carrier == "" {
		carrier = "mock-express"
	}
	o.detail.Status = "shipped"
	o.logistics.Status = "in_transit"
	o.logistics.Carrier = carrier
	o.logistics.TrackingNo = trackingNo
	o.logistics.Traces = append(o.logistics.Traces, "包裹已由供应商发出")
	return nil
}

// GetPayStatus implements Provider.
func (m *Mock1688) GetPayStatus(_ context.Context, externalOrderID string) (*PayStatus, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	o, ok := m.orders[externalOrderID]
	if !ok {
		return nil, ErrOrderNotFound
	}
	cp := o.pay
	return &cp, nil
}

// GetOrder implements Provider.
func (m *Mock1688) GetOrder(_ context.Context, externalOrderID string) (*OrderDetail, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	o, ok := m.orders[externalOrderID]
	if !ok {
		return nil, ErrOrderNotFound
	}
	cp := o.detail
	return &cp, nil
}

// GetLogistics implements Provider.
func (m *Mock1688) GetLogistics(_ context.Context, externalOrderID string) (*LogisticsInfo, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	o, ok := m.orders[externalOrderID]
	if !ok {
		return nil, ErrOrderNotFound
	}
	cp := o.logistics
	return &cp, nil
}

// CancelOrder implements Provider.
func (m *Mock1688) CancelOrder(_ context.Context, externalOrderID string, _ string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	o, ok := m.orders[externalOrderID]
	if !ok {
		return ErrOrderNotFound
	}
	o.detail.Status = "cancelled"
	return nil
}
