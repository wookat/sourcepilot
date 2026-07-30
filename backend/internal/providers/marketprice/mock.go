package marketprice

import (
	"context"
	"encoding/json"
	"fmt"
	"hash/fnv"
	"math"
	"strings"
)

// MockProvider deterministically fabricates realistic-looking listings so the
// full selection pipeline can run without any platform credentials. The same
// keyword always yields the same listing (stable tests and demos).
type MockProvider struct{}

// Name implements Provider.
func (p *MockProvider) Name() string { return "mock" }

var mockCurrencyByCountry = map[string]string{
	"US": "USD", "GB": "GBP", "DE": "EUR", "FR": "EUR",
	"JP": "JPY", "SG": "SGD", "MY": "MYR", "TH": "THB",
	"VN": "VND", "ID": "IDR", "PH": "PHP",
}

// FetchListing implements Provider with seeded pseudo-random data.
func (p *MockProvider) FetchListing(_ context.Context, q Query) (*Listing, error) {
	kw := strings.TrimSpace(q.Keyword)
	if kw == "" {
		return nil, fmt.Errorf("marketprice mock: keyword required")
	}
	platform := strings.TrimSpace(q.Platform)
	if platform == "" {
		platform = "tiktok"
	}
	country := strings.ToUpper(strings.TrimSpace(q.Country))
	if country == "" {
		country = "US"
	}
	currency := mockCurrencyByCountry[country]
	if currency == "" {
		currency = "USD"
	}

	h := fnv.New64a()
	_, _ = h.Write([]byte(platform + "|" + country + "|" + strings.ToLower(kw)))
	seed := h.Sum64()

	// Price in a plausible band: 5.99 – 89.99 (major currency units).
	price := 5.0 + float64(seed%8500)/100.0
	price = math.Floor(price) + 0.99
	if currency == "JPY" || currency == "VND" || currency == "IDR" {
		price = math.Round(price * 150)
	}
	sales := int(200 + seed%4800)

	raw, _ := json.Marshal(map[string]any{
		"provider": "mock",
		"seedHash": seed,
		"query":    q,
	})
	return &Listing{
		Platform:  platform,
		Title:     kw,
		Price:     price,
		Currency:  currency,
		Sales30d:  sales,
		URL:       fmt.Sprintf("https://mock.%s.example/listing/%d", platform, seed%1000000),
		ImageURL:  q.ImageURL,
		Raw:       raw,
		Confident: false,
	}, nil
}
