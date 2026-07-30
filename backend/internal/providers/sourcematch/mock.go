package sourcematch

import (
	"context"
	"encoding/json"
	"fmt"
	"hash/fnv"
	"math"
	"strings"
)

// MockProvider deterministically fabricates 1688 offers so the pipeline can
// run end-to-end without credentials. Same keyword → same offers.
type MockProvider struct{}

// Name implements Provider.
func (p *MockProvider) Name() string { return "mock" }

var mockSuppliers = []string{
	"义乌市优品电子商务有限公司",
	"深圳市联创科技实业有限公司",
	"广州市尚品服饰贸易商行",
	"东莞市恒达五金制品厂",
	"杭州千汇日用品有限公司",
	"宁波贝乐母婴用品有限公司",
}

// MatchByImage implements Provider (mock: same generator, higher similarity).
func (p *MockProvider) MatchByImage(ctx context.Context, req MatchRequest) ([]Match, error) {
	if strings.TrimSpace(req.ImageURL) == "" {
		return nil, fmt.Errorf("sourcematch mock: imageUrl required for image search")
	}
	return p.generate(req, "image", 0.86)
}

// MatchByKeyword implements Provider.
func (p *MockProvider) MatchByKeyword(ctx context.Context, req MatchRequest) ([]Match, error) {
	if strings.TrimSpace(req.Keyword) == "" {
		return nil, fmt.Errorf("sourcematch mock: keyword required")
	}
	return p.generate(req, "keyword", 0.72)
}

func (p *MockProvider) generate(req MatchRequest, method string, baseSim float64) ([]Match, error) {
	limit := req.Limit
	if limit <= 0 {
		limit = 3
	}
	if limit > 10 {
		limit = 10
	}
	h := fnv.New64a()
	_, _ = h.Write([]byte(method + "|" + strings.ToLower(strings.TrimSpace(req.Keyword)) + "|" + req.ImageURL))
	seed := h.Sum64()

	out := make([]Match, 0, limit)
	for i := 0; i < limit; i++ {
		s := seed + uint64(i)*2654435761
		// Purchase price band 3.5 – 68.5 CNY, later matches slightly cheaper/less similar.
		minPrice := 3.5 + float64(s%6500)/100.0
		maxPrice := minPrice * (1.15 + float64(s%20)/100.0)
		sim := baseSim - float64(i)*0.06 - float64(s%5)/100.0
		if sim < 0.35 {
			sim = 0.35
		}
		offerID := fmt.Sprintf("%d", 600000000000+s%99999999999)
		raw, _ := json.Marshal(map[string]any{"provider": "mock", "seed": s})
		out = append(out, Match{
			SourcePlatform: "1688",
			SourceURL:      "https://detail.1688.com/offer/" + offerID + ".html",
			SourceOfferID:  offerID,
			MatchMethod:    method,
			Similarity:     math.Round(sim*10000) / 10000,
			MinPrice:       math.Round(minPrice*100) / 100,
			MaxPrice:       math.Round(maxPrice*100) / 100,
			Currency:       "CNY",
			MOQ:            int(2 + s%98),
			SupplierName:   mockSuppliers[int(s)%len(mockSuppliers)],
			SupplierRating: math.Round((3.6+float64(s%14)/10.0)*100) / 100,
			Raw:            raw,
		})
	}
	return out, nil
}
