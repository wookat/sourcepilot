// Package sourcematch provides 1688 same-item matching providers for the
// selection engine. Implementations: mock (default, credential-free), crawler
// (collector-backed fallback that needs a 1688 browser login profile) and an
// open-platform API shell to be filled once开放平台资质 is granted.
package sourcematch

import (
	"context"
	"encoding/json"
	"errors"
)

// MatchRequest describes one candidate to find 1688 supply for.
type MatchRequest struct {
	Keyword  string `json:"keyword"`  // title / search keyword
	ImageURL string `json:"imageUrl"` // main image for image search
	Category string `json:"category"` // optional category hint
	// SourceURL is an optional already-known 1688 offer URL (e.g. the candidate
	// originated from a collected draft); providers may use it directly.
	SourceURL string `json:"sourceUrl"`
	Limit     int    `json:"limit"` // max matches to return (default 3)
}

// Match is one matched 1688 offer.
type Match struct {
	SourcePlatform string          `json:"sourcePlatform"` // always "1688" for now
	SourceURL      string          `json:"sourceUrl"`
	SourceOfferID  string          `json:"sourceOfferId"`
	MatchMethod    string          `json:"matchMethod"` // image|keyword|attribute|direct
	Similarity     float64         `json:"similarity"`  // 0–1
	MinPrice       float64         `json:"minPrice"`    // CNY
	MaxPrice       float64         `json:"maxPrice"`    // CNY
	Currency       string          `json:"currency"`
	MOQ            int             `json:"moq"`
	SupplierName   string          `json:"supplierName"`
	SupplierRating float64         `json:"supplierRating"` // 0–5
	Raw            json.RawMessage `json:"raw,omitempty"`
}

// ErrUnavailable means the provider cannot serve right now (e.g. crawler has
// no 1688 login profile, or the official API is not yet credentialed). The
// pipeline treats it as a graceful degrade signal, not a hard failure.
var ErrUnavailable = errors.New("sourcematch: provider unavailable")

// Provider matches a candidate against 1688 supply.
type Provider interface {
	Name() string
	// MatchByImage searches by main image (拍立淘类图搜).
	MatchByImage(ctx context.Context, req MatchRequest) ([]Match, error)
	// MatchByKeyword searches by title keyword / attributes.
	MatchByKeyword(ctx context.Context, req MatchRequest) ([]Match, error)
}
