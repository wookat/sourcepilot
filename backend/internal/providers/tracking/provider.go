// Package tracking defines the logistics-tracking provider abstraction. The
// current product ships with only the manual provider (operators update
// shipment status by hand); real carrier tracking APIs (快递100 / 快递鸟 /
// carrier-native) plug in behind the same interface later.
package tracking

import (
	"context"
	"errors"
	"time"
)

// ErrManualOnly is returned by providers that cannot fetch remote tracking
// events; callers should fall back to manual status updates.
var ErrManualOnly = errors.New("tracking: manual provider, update shipment status by hand")

// Event is one tracking checkpoint of a waybill.
type Event struct {
	Status      string    `json:"status"`
	Description string    `json:"description,omitempty"`
	OccurredAt  time.Time `json:"occurredAt"`
}

// Result is the latest tracking snapshot of a waybill.
type Result struct {
	// Status uses order shipment status values (shipped / in_transit /
	// delivered / exception / returned) so callers can drive the existing
	// order lifecycle without translation.
	Status string  `json:"status"`
	Events []Event `json:"events,omitempty"`
}

// Provider fetches tracking events for one waybill.
type Provider interface {
	// Name is a stable provider id, e.g. "manual".
	Name() string
	// SupportsFetch reports whether Fetch can return remote events.
	SupportsFetch() bool
	// Fetch returns the latest tracking snapshot; manual providers return
	// ErrManualOnly.
	Fetch(ctx context.Context, carrierCode, trackingNo string) (*Result, error)
}

// ManualProvider is the built-in no-API provider: operators edit shipment
// status themselves, which drives the order 在途→送达 flow unchanged.
type ManualProvider struct{}

// Name implements Provider.
func (ManualProvider) Name() string { return "manual" }

// SupportsFetch implements Provider.
func (ManualProvider) SupportsFetch() bool { return false }

// Fetch implements Provider.
func (ManualProvider) Fetch(context.Context, string, string) (*Result, error) {
	return nil, ErrManualOnly
}

// Default returns the active tracking provider. Only manual exists today;
// settings-driven selection slots in here when real providers land.
func Default() Provider { return ManualProvider{} }
