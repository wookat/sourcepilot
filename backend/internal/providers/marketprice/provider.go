// Package marketprice provides overseas listing price/sales providers for the
// selection engine. Current sources: manual import (rows carried in the task
// payload) and a mock generator; real platform search APIs (Amazon SP-API,
// TikTok Shop open platform, …) plug in behind Provider later.
package marketprice

import (
	"context"
	"encoding/json"
)

// Query describes one candidate whose overseas listing should be resolved.
type Query struct {
	Platform string `json:"platform"` // e.g. tiktok / shopee / amazon
	Country  string `json:"country"`  // target country/region, e.g. US
	Keyword  string `json:"keyword"`  // title or search keyword
	ImageURL string `json:"imageUrl"` // optional main image
}

// Listing is one resolved overseas listing.
type Listing struct {
	Platform  string          `json:"platform"`
	Title     string          `json:"title"`
	Price     float64         `json:"price"`
	Currency  string          `json:"currency"`
	Sales30d  int             `json:"sales30d"`
	URL       string          `json:"url"`
	ImageURL  string          `json:"imageUrl"`
	Raw       json.RawMessage `json:"raw,omitempty"`
	Confident bool            `json:"confident"` // false when the match is heuristic
}

// Provider resolves overseas in-market listings for a query.
type Provider interface {
	Name() string
	FetchListing(ctx context.Context, q Query) (*Listing, error)
}
