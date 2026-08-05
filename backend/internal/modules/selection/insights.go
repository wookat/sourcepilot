package selection

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"strings"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"

	"github.com/trademind-ai/trademind/backend/internal/modules/bannedwords"
	"github.com/trademind-ai/trademind/backend/internal/providers/markettrend"
)

// maxCompareCandidates limits one并排对比 request.
const maxCompareCandidates = 5

// benchmarkWindowDays is the站内经营数据统计窗口.
const benchmarkWindowDays = 90

// CollectedFacts groups采集所得数据 for one candidate. Nil pointers mean the
// field was never collected and the UI must show「未采集」explicitly.
type CollectedFacts struct {
	MarketPrice       *float64   `json:"marketPrice,omitempty"`
	MarketCurrency    string     `json:"marketCurrency,omitempty"`
	MarketSales30d    *int       `json:"marketSales30d,omitempty"`
	MarketReviewCount *int       `json:"marketReviewCount,omitempty"`
	SourcePrice       *float64   `json:"sourcePrice,omitempty"`
	SourceCurrency    string     `json:"sourceCurrency,omitempty"`
	SourceSales       *int       `json:"sourceSales,omitempty"`
	SourceReviewCount *int       `json:"sourceReviewCount,omitempty"`
	SourceCapturedAt  *time.Time `json:"sourceCapturedAt,omitempty"`
	// CollectCount counts successful collect tasks for the candidate source
	// URL (multiple captures enable the price trend view).
	CollectCount int64 `json:"collectCount"`
}

// CategoryBenchmark aggregates站内同类目经营数据 (drafts + orders).
type CategoryBenchmark struct {
	Category     string `json:"category"`
	ProductCount int64  `json:"productCount"`
	// AvgDraftMarginPercent averages (price-cost)/price over category draft
	// SKUs that carry both prices; nil when no SKU has cost data.
	AvgDraftMarginPercent *float64 `json:"avgDraftMarginPercent,omitempty"`
	WindowDays            int      `json:"windowDays"`
	OrderCount            int64    `json:"orderCount"`
	SoldQty               int64    `json:"soldQty"`
	Revenue               *float64 `json:"revenue,omitempty"`
	GrossProfit           *float64 `json:"grossProfit,omitempty"`
	GrossMarginPercent    *float64 `json:"grossMarginPercent,omitempty"`
}

// CandidateInsightsDTO is GET /selection/candidates/:id/insights.
type CandidateInsightsDTO struct {
	Candidate  SelectionCandidate         `json:"candidate"`
	Evaluation *SelectionEvaluation       `json:"evaluation,omitempty"`
	BestMatch  *SelectionSourceMatch      `json:"bestMatch,omitempty"`
	Collected  CollectedFacts             `json:"collected"`
	Benchmark  *CategoryBenchmark         `json:"benchmark,omitempty"`
	External   []markettrend.SourceStatus `json:"external"`
}

// TrendPoint is one collected price snapshot.
type TrendPoint struct {
	CapturedAt time.Time `json:"capturedAt"`
	Price      float64   `json:"price"`
	TaskID     uuid.UUID `json:"taskId"`
}

// PriceTrendDTO is GET /selection/candidates/:id/price-trend.
type PriceTrendDTO struct {
	SourceURL string       `json:"sourceUrl,omitempty"`
	Currency  string       `json:"currency,omitempty"`
	Points    []TrendPoint `json:"points"`
}

// SupplyReadiness describes货源档案匹配 for one candidate.
type SupplyReadiness struct {
	Ready        bool   `json:"ready"`
	SupplierName string `json:"supplierName,omitempty"`
	SourceStatus string `json:"sourceStatus,omitempty"`
}

// BannedRisk summarizes违禁词命中 on the candidate title.
type BannedRisk struct {
	ForbiddenCount int      `json:"forbiddenCount"`
	WarningCount   int      `json:"warningCount"`
	Words          []string `json:"words,omitempty"`
}

// CompareRowDTO is one column of the选品对比 matrix.
type CompareRowDTO struct {
	Candidate  SelectionCandidate    `json:"candidate"`
	Evaluation *SelectionEvaluation  `json:"evaluation,omitempty"`
	BestMatch  *SelectionSourceMatch `json:"bestMatch,omitempty"`
	Supply     SupplyReadiness       `json:"supply"`
	Banned     BannedRisk            `json:"banned"`
}

// getCandidateScoped loads one candidate enforcing tenant isolation (404 on
// cross-tenant access, indistinguishable from missing).
func (s *Service) getCandidateScoped(ctx context.Context, tenantID int64, id uuid.UUID) (*SelectionCandidate, error) {
	var cand SelectionCandidate
	if err := s.DB.WithContext(ctx).First(&cand, "id = ? AND tenant_id = ?", id, tenantID).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	return &cand, nil
}

func (s *Service) evaluationFor(ctx context.Context, candID uuid.UUID) (*SelectionEvaluation, error) {
	var eval SelectionEvaluation
	err := s.DB.WithContext(ctx).First(&eval, "candidate_id = ?", candID).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &eval, nil
}

func (s *Service) bestMatchFor(ctx context.Context, candID uuid.UUID, eval *SelectionEvaluation) (*SelectionSourceMatch, error) {
	if eval != nil && eval.BestMatchID != nil {
		var m SelectionSourceMatch
		if err := s.DB.WithContext(ctx).First(&m, "id = ?", *eval.BestMatchID).Error; err == nil {
			return &m, nil
		} else if !errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, err
		}
	}
	var m SelectionSourceMatch
	err := s.DB.WithContext(ctx).Where("candidate_id = ?", candID).Order("similarity DESC").First(&m).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &m, nil
}

// CandidateInsights builds the候选商品数据面板 payload.
func (s *Service) CandidateInsights(ctx context.Context, tenantID int64, id uuid.UUID) (*CandidateInsightsDTO, error) {
	if s == nil || s.DB == nil {
		return nil, fmt.Errorf("selection: no db")
	}
	cand, err := s.getCandidateScoped(ctx, tenantID, id)
	if err != nil {
		return nil, err
	}
	eval, err := s.evaluationFor(ctx, cand.ID)
	if err != nil {
		return nil, err
	}
	best, err := s.bestMatchFor(ctx, cand.ID, eval)
	if err != nil {
		return nil, err
	}
	out := &CandidateInsightsDTO{Candidate: *cand, Evaluation: eval, BestMatch: best}
	out.Collected = s.collectedFacts(ctx, tenantID, cand)
	if strings.TrimSpace(cand.Category) != "" {
		bm, err := s.categoryBenchmark(ctx, tenantID, cand.Category)
		if err != nil {
			return nil, err
		}
		out.Benchmark = bm
	}
	if s.Trend != nil {
		out.External = s.Trend.Status(ctx)
	} else {
		out.External = []markettrend.SourceStatus{}
	}
	return out, nil
}

// collectedFacts merges candidate market fields with the latest successful
// collect task for the source URL. Missing values stay nil (未采集).
func (s *Service) collectedFacts(ctx context.Context, tenantID int64, cand *SelectionCandidate) CollectedFacts {
	f := CollectedFacts{
		MarketPrice:    cand.MarketPrice,
		MarketCurrency: cand.MarketCurrency,
		MarketSales30d: cand.MarketSales30d,
	}
	if len(cand.MarketRaw) > 0 {
		if rc := extractIntFact(cand.MarketRaw, reviewKeys); rc != nil {
			f.MarketReviewCount = rc
		}
	}
	url := strings.TrimSpace(cand.SourceURL)
	if url == "" {
		return f
	}
	type row struct {
		RawResult  []byte
		FinishedAt *time.Time
	}
	var latest row
	err := s.DB.WithContext(ctx).Table("collect_tasks").
		Select("raw_result, finished_at").
		Where("tenant_id = ? AND source_url = ? AND status = ?", tenantID, url, "success").
		Order("finished_at DESC NULLS LAST").Limit(1).Scan(&latest).Error
	if err != nil {
		return f
	}
	if err := s.DB.WithContext(ctx).Table("collect_tasks").
		Where("tenant_id = ? AND source_url = ? AND status = ?", tenantID, url, "success").
		Count(&f.CollectCount).Error; err != nil {
		f.CollectCount = 0
	}
	if len(latest.RawResult) == 0 {
		return f
	}
	price, currency := extractPriceFact(latest.RawResult)
	f.SourcePrice = price
	f.SourceCurrency = currency
	f.SourceSales = extractIntFact(latest.RawResult, salesKeys)
	f.SourceReviewCount = extractIntFact(latest.RawResult, reviewKeys)
	f.SourceCapturedAt = latest.FinishedAt
	return f
}

// CandidatePriceTrend returns the collected price series of the candidate
// source URL (one point per successful collect task carrying a price).
func (s *Service) CandidatePriceTrend(ctx context.Context, tenantID int64, id uuid.UUID) (*PriceTrendDTO, error) {
	if s == nil || s.DB == nil {
		return nil, fmt.Errorf("selection: no db")
	}
	cand, err := s.getCandidateScoped(ctx, tenantID, id)
	if err != nil {
		return nil, err
	}
	out := &PriceTrendDTO{SourceURL: cand.SourceURL, Points: []TrendPoint{}}
	url := strings.TrimSpace(cand.SourceURL)
	if url == "" {
		return out, nil
	}
	type row struct {
		ID         uuid.UUID
		RawResult  []byte
		FinishedAt *time.Time
		CreatedAt  time.Time
	}
	var rows []row
	if err := s.DB.WithContext(ctx).Table("collect_tasks").
		Select("id, raw_result, finished_at, created_at").
		Where("tenant_id = ? AND source_url = ? AND status = ?", tenantID, url, "success").
		Order("finished_at ASC NULLS LAST").
		Limit(200).Scan(&rows).Error; err != nil {
		return nil, err
	}
	for _, r := range rows {
		price, currency := extractPriceFact(r.RawResult)
		if price == nil {
			continue
		}
		at := r.CreatedAt
		if r.FinishedAt != nil {
			at = *r.FinishedAt
		}
		if out.Currency == "" && currency != "" {
			out.Currency = currency
		}
		out.Points = append(out.Points, TrendPoint{CapturedAt: at, Price: *price, TaskID: r.ID})
	}
	return out, nil
}

// CompareCandidates builds the并排对比 payload for 2..5 candidates.
func (s *Service) CompareCandidates(ctx context.Context, tenantID int64, ids []uuid.UUID) ([]CompareRowDTO, error) {
	if s == nil || s.DB == nil {
		return nil, fmt.Errorf("selection: no db")
	}
	if len(ids) < 2 {
		return nil, fmt.Errorf("至少选择 2 个候选进行对比")
	}
	if len(ids) > maxCompareCandidates {
		return nil, fmt.Errorf("最多同时对比 %d 个候选", maxCompareCandidates)
	}
	var words []bannedwords.BannedWord
	if s.Banned != nil {
		if ws, err := s.Banned.ActiveWords(ctx, tenantID); err == nil {
			words = ws
		}
	}
	out := make([]CompareRowDTO, 0, len(ids))
	for _, id := range ids {
		cand, err := s.getCandidateScoped(ctx, tenantID, id)
		if err != nil {
			return nil, err
		}
		eval, err := s.evaluationFor(ctx, cand.ID)
		if err != nil {
			return nil, err
		}
		best, err := s.bestMatchFor(ctx, cand.ID, eval)
		if err != nil {
			return nil, err
		}
		row := CompareRowDTO{Candidate: *cand, Evaluation: eval, BestMatch: best}
		row.Supply = s.supplyReadiness(ctx, tenantID, cand, best)
		row.Banned = scanTitleRisk(cand.Title, words)
		out = append(out, row)
	}
	return out, nil
}

// supplyReadiness checks whether an active货源档案 (product_sources) already
// matches the candidate by offer id or source URL.
func (s *Service) supplyReadiness(ctx context.Context, tenantID int64, cand *SelectionCandidate, best *SelectionSourceMatch) SupplyReadiness {
	urls := make([]string, 0, 2)
	offerIDs := make([]string, 0, 1)
	if u := strings.TrimSpace(cand.SourceURL); u != "" {
		urls = append(urls, u)
	}
	if best != nil {
		if u := strings.TrimSpace(best.SourceURL); u != "" {
			urls = append(urls, u)
		}
		if o := strings.TrimSpace(best.SourceOfferID); o != "" {
			offerIDs = append(offerIDs, o)
		}
	}
	if len(urls) == 0 && len(offerIDs) == 0 {
		return SupplyReadiness{}
	}
	q := s.DB.WithContext(ctx).Table("product_sources ps").
		Select("ps.status AS source_status, sup.name AS supplier_name").
		Joins("LEFT JOIN suppliers sup ON sup.id = ps.supplier_id AND sup.deleted_at IS NULL").
		Where("ps.tenant_id = ? AND ps.deleted_at IS NULL AND ps.status <> ?", tenantID, "disabled")
	switch {
	case len(offerIDs) > 0 && len(urls) > 0:
		q = q.Where("ps.source_offer_id IN ? OR ps.source_url IN ?", offerIDs, urls)
	case len(offerIDs) > 0:
		q = q.Where("ps.source_offer_id IN ?", offerIDs)
	default:
		q = q.Where("ps.source_url IN ?", urls)
	}
	type row struct {
		SourceStatus string
		SupplierName string
	}
	var r row
	if err := q.Order("ps.is_primary DESC, ps.priority ASC").Limit(1).Scan(&r).Error; err != nil || r.SourceStatus == "" {
		return SupplyReadiness{}
	}
	return SupplyReadiness{Ready: true, SupplierName: r.SupplierName, SourceStatus: r.SourceStatus}
}

// scanTitleRisk runs the tenant banned-word library over the candidate title.
func scanTitleRisk(title string, words []bannedwords.BannedWord) BannedRisk {
	risk := BannedRisk{Words: []string{}}
	if strings.TrimSpace(title) == "" || len(words) == 0 {
		return risk
	}
	hits := bannedwords.Scan([]bannedwords.FieldText{{Field: "title", Label: "标题", Text: title}}, words)
	seen := map[string]bool{}
	for _, h := range hits {
		if h.Level == bannedwords.LevelForbidden {
			risk.ForbiddenCount++
		} else {
			risk.WarningCount++
		}
		if !seen[h.Word] && len(risk.Words) < 5 {
			risk.Words = append(risk.Words, h.Word)
			seen[h.Word] = true
		}
	}
	return risk
}

// categoryBenchmark aggregates站内同类目 draft margins and order动销 within
// benchmarkWindowDays. Category linkage: products already tied to selection
// candidates of the same category (imported product or converted draft).
func (s *Service) categoryBenchmark(ctx context.Context, tenantID int64, category string) (*CategoryBenchmark, error) {
	productIDs, err := s.categoryProductIDs(ctx, tenantID, category)
	if err != nil {
		return nil, err
	}
	bm := &CategoryBenchmark{Category: category, WindowDays: benchmarkWindowDays, ProductCount: int64(len(productIDs))}
	if len(productIDs) == 0 {
		return bm, nil
	}
	// Draft-side margin: SKUs carrying both price and cost.
	type marginRow struct {
		AvgMargin *float64
	}
	var mr marginRow
	if err := s.DB.WithContext(ctx).Table("product_skus").
		Select("AVG((price - cost_price) / NULLIF(price, 0) * 100) AS avg_margin").
		Where("product_id IN ? AND price IS NOT NULL AND price > 0 AND cost_price IS NOT NULL", productIDs).
		Scan(&mr).Error; err != nil {
		return nil, err
	}
	if mr.AvgMargin != nil {
		v := round2f(*mr.AvgMargin)
		bm.AvgDraftMarginPercent = &v
	}
	// Order-side动销 within the window; cancelled orders excluded. Gross
	// profit only over items whose SKU carries cost data.
	since := time.Now().AddDate(0, 0, -benchmarkWindowDays)
	type orderRow struct {
		OrderCount  int64
		SoldQty     *int64
		Revenue     *float64
		GrossProfit *float64
		CostRevenue *float64
	}
	var or orderRow
	if err := s.DB.WithContext(ctx).Table("order_items oi").
		Select(`COUNT(DISTINCT o.id) AS order_count,
			SUM(oi.quantity) AS sold_qty,
			SUM(oi.total_price) AS revenue,
			SUM(CASE WHEN sku.cost_price IS NOT NULL THEN oi.total_price - sku.cost_price * oi.quantity END) AS gross_profit,
			SUM(CASE WHEN sku.cost_price IS NOT NULL THEN oi.total_price END) AS cost_revenue`).
		Joins("JOIN orders o ON o.id = oi.order_id AND o.deleted_at IS NULL").
		Joins("LEFT JOIN product_skus sku ON sku.id = oi.product_sku_id").
		Where("oi.product_id IN ? AND o.tenant_id = ? AND o.status <> ? AND o.created_at >= ?", productIDs, tenantID, "cancelled", since).
		Scan(&or).Error; err != nil {
		return nil, err
	}
	bm.OrderCount = or.OrderCount
	if or.SoldQty != nil {
		bm.SoldQty = *or.SoldQty
	}
	if or.Revenue != nil {
		v := round2f(*or.Revenue)
		bm.Revenue = &v
	}
	if or.GrossProfit != nil {
		v := round2f(*or.GrossProfit)
		bm.GrossProfit = &v
		if or.CostRevenue != nil && *or.CostRevenue > 0 {
			m := round2f(*or.GrossProfit / *or.CostRevenue * 100)
			bm.GrossMarginPercent = &m
		}
	}
	return bm, nil
}

// categoryProductIDs resolves站内商品 of one selection category: products
// imported as candidates plus drafts converted from candidates of the same
// category (tenant scoped, soft-deleted products excluded).
func (s *Service) categoryProductIDs(ctx context.Context, tenantID int64, category string) ([]uuid.UUID, error) {
	var ids []uuid.UUID
	if err := s.DB.WithContext(ctx).Table("selection_candidates sc").
		Distinct("p.id").
		Joins("LEFT JOIN selection_evaluations se ON se.candidate_id = sc.id").
		Joins("JOIN products p ON (p.id = sc.product_id OR p.id = se.draft_product_id) AND p.deleted_at IS NULL").
		Where("sc.tenant_id = ? AND sc.category = ?", tenantID, category).
		Pluck("p.id", &ids).Error; err != nil {
		return nil, err
	}
	return ids, nil
}

var (
	salesKeys  = []string{"sales30d", "marketSales30d", "monthlySold", "monthSold", "soldCount", "saleCount", "salesCount", "sales"}
	reviewKeys = []string{"reviewCount", "reviewsCount", "reviews", "commentCount", "ratingCount"}
)

// extractIntFact searches a raw JSON blob (top level then one nested level
// under attributes/raw) for the first matching integer-like key.
func extractIntFact(raw []byte, keys []string) *int {
	var m map[string]any
	if err := json.Unmarshal(raw, &m); err != nil {
		return nil
	}
	if v := lookupInt(m, keys); v != nil {
		return v
	}
	for _, nest := range []string{"attributes", "raw"} {
		if sub, ok := m[nest].(map[string]any); ok {
			if v := lookupInt(sub, keys); v != nil {
				return v
			}
		}
	}
	return nil
}

func lookupInt(m map[string]any, keys []string) *int {
	for _, k := range keys {
		v, ok := m[k]
		if !ok {
			continue
		}
		switch n := v.(type) {
		case float64:
			i := int(n)
			return &i
		case string:
			var f float64
			if _, err := fmt.Sscanf(strings.TrimSpace(n), "%f", &f); err == nil {
				i := int(f)
				return &i
			}
		}
	}
	return nil
}

// extractPriceFact resolves the representative price of one collect raw
// result: min SKU price, falling back to a top-level price field.
func extractPriceFact(raw []byte) (*float64, string) {
	var m map[string]any
	if err := json.Unmarshal(raw, &m); err != nil {
		return nil, ""
	}
	currency, _ := m["currency"].(string)
	min := math.MaxFloat64
	found := false
	if skus, ok := m["skus"].([]any); ok {
		for _, it := range skus {
			sku, ok := it.(map[string]any)
			if !ok {
				continue
			}
			if p, ok := sku["price"].(float64); ok && p > 0 && p < min {
				min = p
				found = true
			}
		}
	}
	if !found {
		if p, ok := m["price"].(float64); ok && p > 0 {
			min = p
			found = true
		}
	}
	if !found {
		return nil, currency
	}
	v := round2f(min)
	return &v, currency
}

func round2f(v float64) float64 { return math.Round(v*100) / 100 }
