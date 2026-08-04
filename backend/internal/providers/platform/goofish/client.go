package goofish

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	platformp "github.com/trademind-ai/trademind/backend/internal/providers/platform"
)

// RuntimeConfig resolves bridge endpoint + token from shop auth (no global app config needed).
type RuntimeConfig struct {
	Endpoint string // goofish bridge base URL, e.g. http://127.0.0.1:8787
	Token    string // bridge bearer token (sensitive)
}

// ResolveRuntime extracts bridge connection info from shop auth fields.
func ResolveRuntime(req platformp.TestConnectionRequest) (RuntimeConfig, error) {
	endpoint := strings.TrimSpace(req.Extra["endpoint"])
	if endpoint == "" {
		endpoint = strings.TrimSpace(req.SellerID) // legacy fallback slot
	}
	if endpoint == "" {
		return RuntimeConfig{}, fmt.Errorf("goofish bridge endpoint required (auth field endpoint)")
	}
	return RuntimeConfig{
		Endpoint: strings.TrimRight(endpoint, "/"),
		Token:    strings.TrimSpace(req.AccessToken),
	}, nil
}

type healthResponse struct {
	OK       bool   `json:"ok"`
	UserID   string `json:"userId"`
	Nickname string `json:"nickname"`
	Message  string `json:"message"`
}

type publishRequest struct {
	Title       string  `json:"title"`
	Description string  `json:"description"`
	PriceCNY    float64 `json:"priceCny"`
	ImageURL    string  `json:"imageUrl,omitempty"`
	CategoryOpt string  `json:"categoryOption,omitempty"`
}

type publishResponse struct {
	OK      bool   `json:"ok"`
	ItemID  string `json:"itemId"`
	URL     string `json:"url"`
	Message string `json:"message"`
}

func doJSON(ctx context.Context, cfg RuntimeConfig, method, path string, body any, timeout time.Duration, out any) error {
	var rd io.Reader
	if body != nil {
		b, err := json.Marshal(body)
		if err != nil {
			return err
		}
		rd = bytes.NewReader(b)
	}
	cctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	req, err := http.NewRequestWithContext(cctx, method, cfg.Endpoint+path, rd)
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	if cfg.Token != "" {
		req.Header.Set("Authorization", "Bearer "+cfg.Token)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer func() { _ = resp.Body.Close() }()
	data, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return err
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("goofish bridge %s %s: http %d", method, path, resp.StatusCode)
	}
	return json.Unmarshal(data, out)
}

func bridgeHealth(ctx context.Context, cfg RuntimeConfig) (*healthResponse, error) {
	var out healthResponse
	if err := doJSON(ctx, cfg, http.MethodGet, "/health", nil, 30*time.Second, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

func bridgePublish(ctx context.Context, cfg RuntimeConfig, req publishRequest) (*publishResponse, error) {
	var out publishResponse
	// 浏览器自动化发布 + item id 回填耗时约 2-4 分钟
	if err := doJSON(ctx, cfg, http.MethodPost, "/publish", req, 8*time.Minute, &out); err != nil {
		return nil, err
	}
	if !out.OK {
		return nil, fmt.Errorf("goofish publish failed: %s", strings.TrimSpace(out.Message))
	}
	return &out, nil
}
