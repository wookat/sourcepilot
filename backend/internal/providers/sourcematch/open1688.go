package sourcematch

import (
	"context"
	"fmt"
)

// Open1688Provider is the official 1688 开放平台 implementation shell.
// Filling it requires enterprise开发者资质: App Key/Secret plus 图搜(拍立淘类)
// and商品搜索 API permission packages. Credentials should be stored as
// encrypted settings items (group selection: open1688_app_key /
// open1688_app_secret) and never logged. Until then every call degrades
// gracefully with ErrUnavailable so the pipeline falls back to mock/crawler.
type Open1688Provider struct {
	AppKey    string
	AppSecret string
	// TODO(open1688): signature helper, rate limiting, error-code mapping —
	// keep all platform specifics inside this provider (see docs/provider.md).
}

// Name implements Provider.
func (p *Open1688Provider) Name() string { return "open1688" }

func (p *Open1688Provider) ready() error {
	if p == nil || p.AppKey == "" || p.AppSecret == "" {
		return fmt.Errorf("%w: 1688 open platform credentials not configured", ErrUnavailable)
	}
	// Credentialed but the API integration is not implemented yet.
	return fmt.Errorf("%w: 1688 open platform API not implemented", ErrUnavailable)
}

// MatchByImage implements Provider (占位: 拍立淘图搜 API).
func (p *Open1688Provider) MatchByImage(_ context.Context, _ MatchRequest) ([]Match, error) {
	return nil, p.ready()
}

// MatchByKeyword implements Provider (占位: 商品关键词搜索 API).
func (p *Open1688Provider) MatchByKeyword(_ context.Context, _ MatchRequest) ([]Match, error) {
	return nil, p.ready()
}
