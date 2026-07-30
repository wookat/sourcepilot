// Package sourceinfo abstracts supplier-side offer price/stock lookup.
// The real 1688 open-platform implementation can replace Mock without
// touching the sourcing business layer.
package sourceinfo

import "context"

// SKUQuote is one supplier-side SKU snapshot.
type SKUQuote struct {
	ExternalSKUID string  `json:"externalSkuId"`
	Price         float64 `json:"price"`
	Currency      string  `json:"currency"`
	Stock         int     `json:"stock"`
}

// OfferQuote is the snapshot for one offer (supplier listing).
type OfferQuote struct {
	OfferID string     `json:"offerId"`
	SKUs    []SKUQuote `json:"skus"`
}

// Provider fetches current price/stock for a supplier offer.
type Provider interface {
	Platform() string
	// FetchOffer returns the current quote for offerID. externalSKUIDs
	// limits the SKUs of interest; empty means all known SKUs.
	FetchOffer(ctx context.Context, offerID string, externalSKUIDs []string) (*OfferQuote, error)
}
