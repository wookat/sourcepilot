package sourcematch

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
)

// CollectorGateway is the minimal surface the crawler provider needs from the
// Node collector (implemented by an adapter over collect.CollectorClient so
// the provider layer does not depend on the collect module).
type CollectorGateway interface {
	// HasAuth reports whether a logged-in 1688 browser profile is available.
	HasAuth(ctx context.Context) (bool, error)
	// CollectDetail fetches and normalizes one 1688 offer detail page.
	// Returns the collector's normalized product JSON.
	CollectDetail(ctx context.Context, rawURL string) (json.RawMessage, error)
}

// CrawlerProvider is the collector-backed兜底 implementation. It can only
// resolve offers when the candidate already carries a 1688 offer URL (e.g.
// from a collected draft) and a logged-in browser profile exists; otherwise
// it degrades gracefully with ErrUnavailable. 1688 has no crawlable public
// search without heavy风控 handling, so keyword/image search is not attempted.
type CrawlerProvider struct {
	Collector CollectorGateway
}

// Name implements Provider.
func (p *CrawlerProvider) Name() string { return "crawler" }

// MatchByImage implements Provider. Image search needs the official API.
func (p *CrawlerProvider) MatchByImage(ctx context.Context, req MatchRequest) ([]Match, error) {
	return p.matchDirect(ctx, req)
}

// MatchByKeyword implements Provider.
func (p *CrawlerProvider) MatchByKeyword(ctx context.Context, req MatchRequest) ([]Match, error) {
	return p.matchDirect(ctx, req)
}

func (p *CrawlerProvider) matchDirect(ctx context.Context, req MatchRequest) ([]Match, error) {
	if p == nil || p.Collector == nil {
		return nil, ErrUnavailable
	}
	u := strings.TrimSpace(req.SourceURL)
	if u == "" || !strings.Contains(u, "1688.com") {
		return nil, fmt.Errorf("%w: crawler needs a 1688 offer url", ErrUnavailable)
	}
	ok, err := p.Collector.HasAuth(ctx)
	if err != nil || !ok {
		return nil, fmt.Errorf("%w: 1688 login profile missing", ErrUnavailable)
	}
	raw, err := p.Collector.CollectDetail(ctx, u)
	if err != nil {
		return nil, fmt.Errorf("sourcematch crawler: collect detail: %w", err)
	}
	m, err := matchFromNormalized(u, raw)
	if err != nil {
		return nil, err
	}
	return []Match{*m}, nil
}

// matchFromNormalized converts the collector's normalized product JSON into a Match.
func matchFromNormalized(sourceURL string, raw json.RawMessage) (*Match, error) {
	var n struct {
		Title string `json:"title"`
		SKUs  []struct {
			Price     float64 `json:"price"`
			CostPrice float64 `json:"costPrice"`
		} `json:"skus"`
		Shop struct {
			Name string `json:"name"`
		} `json:"shop"`
	}
	if err := json.Unmarshal(raw, &n); err != nil {
		return nil, fmt.Errorf("sourcematch crawler: parse normalized product: %w", err)
	}
	minP, maxP := 0.0, 0.0
	for _, s := range n.SKUs {
		price := s.Price
		if price <= 0 {
			price = s.CostPrice
		}
		if price <= 0 {
			continue
		}
		if minP == 0 || price < minP {
			minP = price
		}
		if price > maxP {
			maxP = price
		}
	}
	if minP <= 0 {
		return nil, fmt.Errorf("sourcematch crawler: no price in collected detail")
	}
	return &Match{
		SourcePlatform: "1688",
		SourceURL:      sourceURL,
		SourceOfferID:  offerIDFromURL(sourceURL),
		MatchMethod:    "direct",
		Similarity:     1,
		MinPrice:       minP,
		MaxPrice:       maxP,
		Currency:       "CNY",
		SupplierName:   n.Shop.Name,
		Raw:            raw,
	}, nil
}

func offerIDFromURL(u string) string {
	// https://detail.1688.com/offer/<id>.html
	i := strings.LastIndex(u, "/offer/")
	if i < 0 {
		return ""
	}
	rest := u[i+len("/offer/"):]
	if j := strings.Index(rest, "."); j > 0 {
		return rest[:j]
	}
	return rest
}
