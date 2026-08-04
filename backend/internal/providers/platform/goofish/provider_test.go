package goofish

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/google/uuid"

	platformp "github.com/trademind-ai/trademind/backend/internal/providers/platform"
)

func authReq(endpoint, token string) platformp.TestConnectionRequest {
	return platformp.TestConnectionRequest{
		AccessToken: token,
		Extra:       map[string]string{"endpoint": endpoint},
	}
}

func TestResolveRuntimeRequiresEndpoint(t *testing.T) {
	if _, err := ResolveRuntime(platformp.TestConnectionRequest{}); err == nil {
		t.Fatal("expected error without endpoint")
	}
	cfg, err := ResolveRuntime(authReq("http://127.0.0.1:8787/", "tok"))
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Endpoint != "http://127.0.0.1:8787" || cfg.Token != "tok" {
		t.Fatalf("unexpected cfg: %+v", cfg)
	}
}

func TestProviderMetadata(t *testing.T) {
	p := NewProvider()
	if p.Platform() != "goofish" || p.Status() != platformp.StatusBeta {
		t.Fatalf("unexpected metadata: %s %s", p.Platform(), p.Status())
	}
	if !platformp.HasCapability(p, platformp.CapProductPublish) {
		t.Fatal("expected product_publish capability")
	}
	if !platformp.IsProductPublishRunnable(p) {
		t.Fatal("expected goofish publish to be runnable (beta)")
	}
}

func TestTestConnectionAndPublish(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer tok" {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		switch r.URL.Path {
		case "/health":
			_ = json.NewEncoder(w).Encode(map[string]any{"ok": true, "userId": "u1", "nickname": "店铺A"})
		case "/publish":
			var body publishRequest
			_ = json.NewDecoder(r.Body).Decode(&body)
			if body.Title == "" || body.PriceCNY <= 0 {
				w.WriteHeader(http.StatusBadRequest)
				return
			}
			_ = json.NewEncoder(w).Encode(map[string]any{
				"ok": true, "itemId": "10086", "url": "https://www.goofish.com/item?id=10086",
			})
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer srv.Close()

	p := NewProvider()
	res, err := p.TestConnection(context.Background(), authReq(srv.URL, "tok"))
	if err != nil {
		t.Fatal(err)
	}
	if !res.OK || res.ExternalShopID != "u1" || res.ShopName != "店铺A" {
		t.Fatalf("unexpected test connection result: %+v", res)
	}

	pp, ok := platformp.AsProductPublish(p)
	if !ok {
		t.Fatal("expected ProductPublishProvider")
	}
	out, err := pp.PublishProduct(context.Background(), platformp.PublishProductRequest{
		Auth: authReq(srv.URL, "tok"),
		Product: platformp.PlatformProductDraft{
			ProductID: uuid.New(),
			Title:     "测试商品",
			Images:    []platformp.PlatformProductImage{{URL: "https://img.example/1.png", Type: "main"}},
			SKUs:      []platformp.PlatformProductSKU{{Price: 2.99, Stock: 99}},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if out.ExternalProductID != "10086" || out.ExternalURL == "" {
		t.Fatalf("unexpected publish result: %+v", out)
	}
}

func TestPublishValidation(t *testing.T) {
	pp, _ := platformp.AsProductPublish(NewProvider())
	if _, err := pp.PublishProduct(context.Background(), platformp.PublishProductRequest{
		Auth:    authReq("http://127.0.0.1:1", "tok"),
		Product: platformp.PlatformProductDraft{Title: ""},
	}); err == nil {
		t.Fatal("expected title validation error")
	}
	if _, err := pp.PublishProduct(context.Background(), platformp.PublishProductRequest{
		Auth:    authReq("http://127.0.0.1:1", "tok"),
		Product: platformp.PlatformProductDraft{Title: "x", SKUs: []platformp.PlatformProductSKU{{Price: 0}}},
	}); err == nil {
		t.Fatal("expected price validation error")
	}
}
