// Package markettrend abstracts external market-trend data sources (platform
// hot-sales boards, keyword trend APIs, …) for the selection engine. No real
// credentials are wired yet: the registry exposes declared slots so the UI
// can show a "not configured" degrade state instead of fabricated data. Real
// implementations (TikTok Shop hot list, Shopee top products, …) plug in
// behind Provider without touching the selection business layer.
package markettrend

import (
	"context"
	"errors"
)

// ErrNotConfigured is returned by providers that lack credentials.
var ErrNotConfigured = errors.New("markettrend: provider not configured")

// Query describes one hot-list lookup.
type Query struct {
	Platform string `json:"platform"` // e.g. tiktok / shopee / amazon
	Country  string `json:"country"`  // target country/region, e.g. US
	Category string `json:"category"` // optional category filter
	Limit    int    `json:"limit"`
}

// HotItem is one entry of a platform hot-sales board.
type HotItem struct {
	Rank     int      `json:"rank"`
	Title    string   `json:"title"`
	Category string   `json:"category,omitempty"`
	Price    *float64 `json:"price,omitempty"`
	Currency string   `json:"currency,omitempty"`
	Sales30d *int     `json:"sales30d,omitempty"`
	URL      string   `json:"url,omitempty"`
}

// Provider resolves platform hot-list data for a query.
type Provider interface {
	// Name is the stable provider code, e.g. tiktok_hotlist.
	Name() string
	// DisplayName is the Chinese label shown in the admin UI.
	DisplayName() string
	// Configured reports whether required credentials are present.
	Configured(ctx context.Context) bool
	// HotList returns hot-sales entries; ErrNotConfigured when unusable.
	HotList(ctx context.Context, q Query) ([]HotItem, error)
}

// SourceStatus is the admin-facing availability row for one source slot.
type SourceStatus struct {
	Name        string `json:"name"`
	DisplayName string `json:"displayName"`
	Configured  bool   `json:"configured"`
	Message     string `json:"message,omitempty"`
}

// Registry holds registered providers plus declared-but-unregistered slots so
// the UI can render a complete "external data sources" panel with degrade
// hints instead of hiding missing sources.
type Registry struct {
	providers []Provider
	slots     []SourceStatus
}

// NewRegistry declares the known external source slots. Slots without a
// registered provider surface as configured=false.
func NewRegistry() *Registry {
	return &Registry{
		slots: []SourceStatus{
			{Name: "tiktok_hotlist", DisplayName: "TikTok Shop 热销榜", Message: "未配置平台开放接口凭证"},
			{Name: "shopee_hotlist", DisplayName: "Shopee 热销榜", Message: "未配置平台开放接口凭证"},
			{Name: "amazon_bestsellers", DisplayName: "Amazon Best Sellers", Message: "未配置平台开放接口凭证"},
		},
	}
}

// Register adds a real provider implementation.
func (r *Registry) Register(p Provider) {
	if r == nil || p == nil {
		return
	}
	r.providers = append(r.providers, p)
}

// Get returns the registered provider by name, nil when absent.
func (r *Registry) Get(name string) Provider {
	if r == nil {
		return nil
	}
	for _, p := range r.providers {
		if p.Name() == name {
			return p
		}
	}
	return nil
}

// Status merges registered providers over declared slots.
func (r *Registry) Status(ctx context.Context) []SourceStatus {
	if r == nil {
		return []SourceStatus{}
	}
	out := make([]SourceStatus, 0, len(r.slots)+len(r.providers))
	seen := map[string]bool{}
	for _, p := range r.providers {
		st := SourceStatus{Name: p.Name(), DisplayName: p.DisplayName(), Configured: p.Configured(ctx)}
		if !st.Configured {
			st.Message = "未配置平台开放接口凭证"
		}
		out = append(out, st)
		seen[p.Name()] = true
	}
	for _, s := range r.slots {
		if !seen[s.Name] {
			out = append(out, s)
		}
	}
	return out
}
