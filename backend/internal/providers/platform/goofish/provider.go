package goofish

import (
	"context"
	"fmt"
	"strings"
	"time"

	platformp "github.com/trademind-ai/trademind/backend/internal/providers/platform"
)

// goofishProvider publishes listings to 闲鱼 (Goofish) via a self-hosted browser-automation
// bridge service (goofish has no official open API for个人卖家). The bridge holds the logged-in
// session and executes the publish flow; this adapter only speaks HTTP to the bridge.
type goofishProvider struct{}

// NewProvider constructs the Goofish (闲鱼) platform integration (beta).
func NewProvider() platformp.Provider { return goofishProvider{} }

func (goofishProvider) Platform() string { return "goofish" }

func (goofishProvider) Name() string { return "闲鱼" }

func (goofishProvider) Status() string { return platformp.StatusBeta }

func (goofishProvider) Capabilities() []platformp.Capability {
	return []platformp.Capability{platformp.CapProductPublish, platformp.CapShopInfo}
}

func (goofishProvider) AuthSchema() platformp.AuthSchema {
	return platformp.AuthSchema{
		AuthType: "api_key",
		Fields: []platformp.AuthField{
			{Name: "endpoint", Label: "Bridge 服务地址", Type: "text", Required: true, Sensitive: false, Hint: "自托管闲鱼发布桥接服务，如 http://127.0.0.1:8787"},
			{Name: "accessToken", Label: "Bridge Token", Type: "password", Required: true, Sensitive: true, Hint: "桥接服务的 Bearer Token"},
		},
	}
}

func (goofishProvider) AppConfigSchema() platformp.PlatformAppConfigSchema {
	return platformp.PlatformAppConfigSchema{}
}

func (goofishProvider) PublishConfigSchema() platformp.PlatformAppConfigSchema {
	return platformp.PlatformAppConfigSchema{
		GroupKey:    "platform_publish_goofish",
		Title:       "闲鱼商品发布配置",
		Description: "浏览器自动化发布默认参数；发布节奏受平台风控约束，请勿批量连发。",
		Fields: []platformp.AppConfigField{
			{Name: "category_option", Label: "默认分类选项（可选）", Type: "text", Required: false, Sensitive: false, DefaultValue: ""},
		},
	}
}

func (goofishProvider) TestConnection(ctx context.Context, req platformp.TestConnectionRequest) (*platformp.TestConnectionResult, error) {
	cfg, err := ResolveRuntime(req)
	if err != nil {
		return &platformp.TestConnectionResult{OK: false, Message: err.Error()}, nil
	}
	h, err := bridgeHealth(ctx, cfg)
	if err != nil {
		return &platformp.TestConnectionResult{OK: false, Message: err.Error()}, nil
	}
	if !h.OK {
		return &platformp.TestConnectionResult{OK: false, Message: strings.TrimSpace(h.Message)}, nil
	}
	return &platformp.TestConnectionResult{
		OK:             true,
		Message:        "goofish bridge ok",
		ShopName:       h.Nickname,
		ExternalShopID: h.UserID,
		Region:         "CN",
		Currency:       "CNY",
	}, nil
}

func (goofishProvider) PublishProduct(ctx context.Context, req platformp.PublishProductRequest) (*platformp.PublishProductResult, error) {
	cfg, err := ResolveRuntime(req.Auth)
	if err != nil {
		return nil, err
	}
	title := strings.TrimSpace(req.Product.Title)
	if title == "" {
		return nil, fmt.Errorf("product title required for goofish publish")
	}
	price := firstSKUPrice(req.Product.SKUs)
	if price <= 0 {
		return nil, fmt.Errorf("positive sku price required for goofish publish")
	}
	body := publishRequest{
		Title:       title,
		Description: strings.TrimSpace(req.Product.Description),
		PriceCNY:    price,
		ImageURL:    firstMainImageURL(req.Product.Images),
		CategoryOpt: stringOpt(req.PublishConfig, "category_option"),
	}
	res, err := bridgePublish(ctx, cfg, body)
	if err != nil {
		return nil, err
	}
	return &platformp.PublishProductResult{
		ExternalProductID: res.ItemID,
		ExternalURL:       res.URL,
		Status:            "published",
		RawSummary: map[string]any{
			"provider":   "goofish",
			"itemId":     res.ItemID,
			"receivedAt": time.Now().UTC().Format(time.RFC3339),
		},
	}, nil
}

func firstSKUPrice(skus []platformp.PlatformProductSKU) float64 {
	for _, s := range skus {
		if s.Price > 0 {
			return s.Price
		}
	}
	return 0
}

func firstMainImageURL(images []platformp.PlatformProductImage) string {
	for _, img := range images {
		if img.Type == "main" && strings.TrimSpace(img.URL) != "" {
			return img.URL
		}
	}
	for _, img := range images {
		if strings.TrimSpace(img.URL) != "" {
			return img.URL
		}
	}
	return ""
}

func stringOpt(m map[string]any, key string) string {
	if m == nil {
		return ""
	}
	if v, ok := m[key].(string); ok {
		return strings.TrimSpace(v)
	}
	return ""
}
