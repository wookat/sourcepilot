// Package trade abstracts supplier-side purchase ordering (1688 trade API).
// While the official API is unavailable the system runs in "manual order"
// transition mode: Mock1688 backs local state, and operators place orders on
// 1688 by hand then backfill the external order/tracking numbers. The real
// open1688 implementation can be added later without changing callers.
package trade

import (
	"context"
	"time"
)

// PreviewItem is one SKU line to validate before ordering.
type PreviewItem struct {
	OfferID       string  `json:"offerId"`
	ExternalSKUID string  `json:"externalSkuId"`
	Quantity      int     `json:"quantity"`
	ExpectedPrice float64 `json:"expectedPrice"`
}

// PreviewRequest validates price/stock/MOQ before creating an order.
type PreviewRequest struct {
	Items []PreviewItem `json:"items"`
}

// PreviewLine is the per-line validation result.
type PreviewLine struct {
	OfferID       string  `json:"offerId"`
	ExternalSKUID string  `json:"externalSkuId"`
	CurrentPrice  float64 `json:"currentPrice"`
	InStock       bool    `json:"inStock"`
	PriceChanged  bool    `json:"priceChanged"`
}

// PreviewResult aggregates line checks.
type PreviewResult struct {
	OK          bool          `json:"ok"`
	TotalAmount float64       `json:"totalAmount"`
	Lines       []PreviewLine `json:"lines"`
}

// CreateOrderRequest creates a supplier order (idempotency key passthrough).
type CreateOrderRequest struct {
	IdempotencyKey string         `json:"idempotencyKey"`
	Receiver       map[string]any `json:"receiver"`
	Items          []PreviewItem  `json:"items"`
}

// CreateOrderResult reports the created supplier order.
type CreateOrderResult struct {
	ExternalOrderID string  `json:"externalOrderId"`
	TotalAmount     float64 `json:"totalAmount"`
	// Manual is true when the provider cannot place real orders and the
	// operator must order by hand and backfill the external order id.
	Manual bool `json:"manual"`
}

// PayStatus mirrors the supplier-side payment state.
type PayStatus struct {
	ExternalOrderID string     `json:"externalOrderId"`
	Status          string     `json:"status"` // unpaid|paying|paid|refunding|refunded
	PaidAt          *time.Time `json:"paidAt,omitempty"`
}

// OrderDetail is the supplier-side order snapshot (for polling).
type OrderDetail struct {
	ExternalOrderID string  `json:"externalOrderId"`
	Status          string  `json:"status"` // created|paid|shipped|delivered|cancelled
	TotalAmount     float64 `json:"totalAmount"`
}

// LogisticsInfo is the supplier-side shipping snapshot.
type LogisticsInfo struct {
	ExternalOrderID string   `json:"externalOrderId"`
	TrackingNo      string   `json:"trackingNo"`
	Carrier         string   `json:"carrier"`
	Status          string   `json:"status"` // pending|in_transit|delivered
	Traces          []string `json:"traces,omitempty"`
}

// Provider is the supplier trade interface (aligned with 1688 trade API).
type Provider interface {
	Platform() string
	PreviewOrder(ctx context.Context, req PreviewRequest) (*PreviewResult, error)
	CreateOrder(ctx context.Context, req CreateOrderRequest) (*CreateOrderResult, error)
	GetPayStatus(ctx context.Context, externalOrderID string) (*PayStatus, error)
	GetOrder(ctx context.Context, externalOrderID string) (*OrderDetail, error)
	GetLogistics(ctx context.Context, externalOrderID string) (*LogisticsInfo, error)
	CancelOrder(ctx context.Context, externalOrderID string, reason string) error
}
