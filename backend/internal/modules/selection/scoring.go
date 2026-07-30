package selection

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/trademind-ai/trademind/backend/internal/modules/aiprompt"
	ai "github.com/trademind-ai/trademind/backend/internal/providers/ai"
)

// PromptCodeScoring is the ai_prompts code for the selection scoring skill.
const PromptCodeScoring = "selection_scoring"

const defaultScoringSystemPrompt = `你是跨境电商选品专家。根据候选商品的海外售价、销量、1688 采购成本与利润测算，
输出 0-100 的选品评分与结构化理由。只输出 JSON，不要输出其他文本。
JSON 结构：{"score": number, "summary": string, "risks": [string], "sellingPoints": [string], "suggestedPrice": number}`

const defaultScoringUserPrompt = `候选商品数据（JSON）：
{{input}}

请评估该商品在 {{platform}}（目标市场 {{country}}）上架的潜力并按要求输出 JSON。`

// ScoreInput is the structured JSON fed to the LLM.
type ScoreInput struct {
	Title          string        `json:"title"`
	Category       string        `json:"category,omitempty"`
	MarketPlatform string        `json:"marketPlatform"`
	MarketPrice    float64       `json:"marketPrice"`
	MarketCurrency string        `json:"marketCurrency"`
	MarketSales30d int           `json:"marketSales30d"`
	PurchaseCNY    float64       `json:"purchaseCostCny"`
	MOQ            int           `json:"moq,omitempty"`
	SupplierRating float64       `json:"supplierRating,omitempty"`
	Similarity     float64       `json:"matchSimilarity,omitempty"`
	Profit         *ProfitResult `json:"profit,omitempty"`
}

// ScoreResult is the parsed LLM (or fallback rule) output.
type ScoreResult struct {
	Score          float64  `json:"score"`
	Summary        string   `json:"summary"`
	Risks          []string `json:"risks"`
	SellingPoints  []string `json:"sellingPoints"`
	SuggestedPrice float64  `json:"suggestedPrice"`
	Model          string   `json:"model,omitempty"`
	Fallback       bool     `json:"fallback"` // true when rule-based (LLM unavailable/failed)
}

// AIGatewayIface is the surface of providers/ai.Gateway the scorer needs.
type AIGatewayIface interface {
	Chat(ctx context.Context, req ai.ChatRequest) (*ai.ChatResponse, error)
}

// PromptReader loads enabled prompt templates (implemented by aiprompt.Service).
type PromptReader interface {
	GetEnabledByCode(ctx context.Context, code string) (*aiprompt.AIPrompt, error)
}

// scoreCandidate runs LLM scoring with rule-based fallback (纯利润率排序兜底).
func scoreCandidate(ctx context.Context, gw AIGatewayIface, prompts PromptReader, in ScoreInput, platform, country string) *ScoreResult {
	if gw != nil {
		if res, err := scoreWithLLM(ctx, gw, prompts, in, platform, country); err == nil {
			return res
		}
	}
	return ruleScore(in)
}

func scoreWithLLM(ctx context.Context, gw AIGatewayIface, prompts PromptReader, in ScoreInput, platform, country string) (*ScoreResult, error) {
	sys, usr := defaultScoringSystemPrompt, defaultScoringUserPrompt
	model := ""
	temp := 0.2
	maxTok := 800
	if prompts != nil {
		if p, err := prompts.GetEnabledByCode(ctx, PromptCodeScoring); err == nil && p != nil {
			if strings.TrimSpace(p.SystemPrompt) != "" {
				sys = p.SystemPrompt
			}
			if strings.TrimSpace(p.UserPrompt) != "" {
				usr = p.UserPrompt
			}
			model = p.Model
			if p.Temperature > 0 {
				temp = p.Temperature
			}
			if p.MaxTokens > 0 {
				maxTok = p.MaxTokens
			}
		}
	}
	inputJSON, err := json.Marshal(in)
	if err != nil {
		return nil, err
	}
	usr = aiprompt.ReplaceVariables(usr, map[string]string{
		"input":    string(inputJSON),
		"platform": platform,
		"country":  country,
	})
	resp, err := gw.Chat(ctx, ai.ChatRequest{
		Model:       model,
		Temperature: temp,
		MaxTokens:   maxTok,
		Messages: []ai.Message{
			{Role: "system", Content: sys},
			{Role: "user", Content: usr},
		},
		ResponseFormat: &ai.ResponseFormat{Type: "json_object"},
	})
	if err != nil {
		return nil, err
	}
	out, err := parseScoreJSON(resp.Content)
	if err != nil {
		return nil, err
	}
	out.Model = resp.Model
	return out, nil
}

// parseScoreJSON extracts and validates the LLM JSON output.
func parseScoreJSON(content string) (*ScoreResult, error) {
	s := strings.TrimSpace(content)
	// Tolerate fenced code blocks.
	if strings.HasPrefix(s, "```") {
		s = strings.TrimPrefix(s, "```json")
		s = strings.TrimPrefix(s, "```")
		s = strings.TrimSuffix(strings.TrimSpace(s), "```")
	}
	if i := strings.Index(s, "{"); i > 0 {
		s = s[i:]
	}
	if j := strings.LastIndex(s, "}"); j >= 0 {
		s = s[:j+1]
	}
	var out ScoreResult
	if err := json.Unmarshal([]byte(s), &out); err != nil {
		return nil, fmt.Errorf("selection scoring: parse llm json: %w", err)
	}
	if out.Score < 0 {
		out.Score = 0
	}
	if out.Score > 100 {
		out.Score = 100
	}
	return &out, nil
}

// ruleScore is the credential-free兜底: margin-driven score with sales and
// supply-quality adjustments, clamped to 0–100.
func ruleScore(in ScoreInput) *ScoreResult {
	margin := 0.0
	profit := 0.0
	if in.Profit != nil {
		margin = in.Profit.EstMarginPercent
		profit = in.Profit.EstProfit
	}
	// Margin dominates: 0% → 20分, 30% → 80分, linear, clamped.
	score := 20 + margin*2
	if score < 0 {
		score = 0
	}
	if score > 85 {
		score = 85
	}
	// Sales bonus up to +10.
	if in.MarketSales30d > 0 {
		bonus := float64(in.MarketSales30d) / 500
		if bonus > 10 {
			bonus = 10
		}
		score += bonus
	}
	// Supplier rating bonus up to +5.
	if in.SupplierRating > 0 {
		score += (in.SupplierRating - 3) * 2.5
	}
	if score < 0 {
		score = 0
	}
	if score > 100 {
		score = 100
	}
	risks := []string{}
	if profit <= 0 {
		risks = append(risks, "当前测算为亏损，需重估售价或换货源")
	}
	if in.Similarity > 0 && in.Similarity < 0.6 {
		risks = append(risks, "1688 同款匹配相似度较低，需人工核对")
	}
	if in.MOQ > 50 {
		risks = append(risks, fmt.Sprintf("起订量较高（MOQ=%d），首单占用资金多", in.MOQ))
	}
	return &ScoreResult{
		Score:    round2(score),
		Summary:  fmt.Sprintf("规则兜底评分：预估利润率 %.2f%%，月销 %d", margin, in.MarketSales30d),
		Risks:    risks,
		Fallback: true,
	}
}
