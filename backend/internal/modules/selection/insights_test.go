package selection

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"gorm.io/gorm"

	"github.com/trademind-ai/trademind/backend/internal/modules/bannedwords"
	"github.com/trademind-ai/trademind/backend/internal/modules/collect"
	"github.com/trademind-ai/trademind/backend/internal/modules/order"
	"github.com/trademind-ai/trademind/backend/internal/modules/product"
	"github.com/trademind-ai/trademind/backend/internal/modules/sourcing"
	"github.com/trademind-ai/trademind/backend/internal/pkg/ctxkey"
	"github.com/trademind-ai/trademind/backend/internal/providers/markettrend"
)

func newInsightsDB(t *testing.T) *gorm.DB {
	t.Helper()
	db := newGuardDB(t)
	if err := db.AutoMigrate(
		&collect.CollectTask{},
		&sourcing.Supplier{}, &sourcing.ProductSource{},
		&bannedwords.BannedWord{}, &bannedwords.BannedWordCategoryState{},
		&product.Product{}, &product.ProductSKU{},
		&order.Order{}, &order.OrderItem{},
	); err != nil {
		t.Fatal(err)
	}
	return db
}

func newEngineWith(uid uuid.UUID, tenantID int64, svc *Service) http.Handler {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(func(c *gin.Context) {
		c.Set(ctxkey.AdminID, uid.String())
		c.Set(ctxkey.TenantID, tenantID)
		c.Next()
	})
	Register(r.Group("/api/v1"), &Handler{Svc: svc})
	return r
}

func newInsightsRouter(db *gorm.DB, uid uuid.UUID, tenantID int64) *httptest.Server {
	return httptest.NewServer(newEngineWith(uid, tenantID, &Service{DB: db}))
}

func seedScoredCandidate(t *testing.T, db *gorm.DB, tenantID int64, sourceURL string) *SelectionCandidate {
	t.Helper()
	task := seedTask(t, db, tenantID)
	price := 19.99
	sales := 320
	cand := &SelectionCandidate{
		TenantID:       tenantID,
		TaskID:         task.ID,
		Title:          "insight cand",
		Category:       "宠物用品",
		SourceURL:      sourceURL,
		MarketPrice:    &price,
		MarketCurrency: "USD",
		MarketSales30d: &sales,
		Status:         CandidateScored,
	}
	if err := db.Create(cand).Error; err != nil {
		t.Fatal(err)
	}
	return cand
}

func getEnv(t *testing.T, srv *httptest.Server, path string, out any) int {
	t.Helper()
	resp, err := http.Get(srv.URL + path)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	var env struct {
		Data json.RawMessage `json:"data"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&env); err != nil {
		t.Fatal(err)
	}
	if out != nil && len(env.Data) > 0 {
		if err := json.Unmarshal(env.Data, out); err != nil {
			t.Fatal(err)
		}
	}
	return resp.StatusCode
}

// insights must expose collected facts, keep missing values nil (未采集) and
// stay tenant-scoped (cross tenant candidate → 404).
func TestCandidateInsightsTenantScopedAndNotCollected(t *testing.T) {
	db := newInsightsDB(t)
	uid := seedAdmin(t, db, "admin")
	cand := seedScoredCandidate(t, db, 1, "")
	srv := newInsightsRouter(db, uid, 1)
	defer srv.Close()

	var dto CandidateInsightsDTO
	if code := getEnv(t, srv, "/api/v1/selection/candidates/"+cand.ID.String()+"/insights", &dto); code != http.StatusOK {
		t.Fatalf("insights: got %d", code)
	}
	if dto.Collected.MarketPrice == nil || *dto.Collected.MarketPrice != 19.99 {
		t.Fatalf("market price: %+v", dto.Collected.MarketPrice)
	}
	if dto.Collected.MarketReviewCount != nil || dto.Collected.SourcePrice != nil {
		t.Fatalf("uncollected fields must stay nil: %+v", dto.Collected)
	}
	if dto.Collected.CollectCount != 0 {
		t.Fatalf("collect count: %d", dto.Collected.CollectCount)
	}

	// cross tenant → 404
	srv2 := newInsightsRouter(db, uid, 2)
	defer srv2.Close()
	if code := getEnv(t, srv2, "/api/v1/selection/candidates/"+cand.ID.String()+"/insights", nil); code != http.StatusNotFound {
		t.Fatalf("cross tenant: got %d, want 404", code)
	}
}

// external source slots must degrade to configured=false without credentials.
func TestCandidateInsightsExternalDegrades(t *testing.T) {
	db := newInsightsDB(t)
	uid := seedAdmin(t, db, "admin")
	cand := seedScoredCandidate(t, db, 1, "")
	srv := httptest.NewServer(newEngineWith(uid, 1, &Service{DB: db, Trend: markettrend.NewRegistry()}))
	defer srv.Close()

	var dto CandidateInsightsDTO
	if code := getEnv(t, srv, "/api/v1/selection/candidates/"+cand.ID.String()+"/insights", &dto); code != http.StatusOK {
		t.Fatalf("insights: got %d", code)
	}
	if len(dto.External) == 0 {
		t.Fatal("expected declared external source slots")
	}
	for _, s := range dto.External {
		if s.Configured {
			t.Fatalf("no provider should be configured: %+v", s)
		}
		if s.Message == "" {
			t.Fatalf("degraded slot needs a message: %+v", s)
		}
	}
}

// price trend must return one point per successful collect capture, in time
// order, and skip captures without a parsable price.
func TestCandidatePriceTrendFromCollectHistory(t *testing.T) {
	db := newInsightsDB(t)
	uid := seedAdmin(t, db, "admin")
	url := "https://detail.1688.com/offer/1.html"
	cand := seedScoredCandidate(t, db, 1, url)

	mk := func(price float64, at time.Time, status string) {
		raw, _ := json.Marshal(map[string]any{
			"currency": "CNY",
			"skus":     []map[string]any{{"price": price}},
		})
		task := &collect.CollectTask{
			TenantID:   1,
			Source:     "1688",
			SourceURL:  url,
			Status:     status,
			RawResult:  raw,
			FinishedAt: &at,
		}
		if err := db.Create(task).Error; err != nil {
			t.Fatal(err)
		}
	}
	base := time.Now().Add(-72 * time.Hour)
	mk(12.5, base, collect.StatusSuccess)
	mk(11.8, base.Add(24*time.Hour), collect.StatusSuccess)
	mk(99.9, base.Add(30*time.Hour), collect.StatusFailed) // ignored
	mk(13.2, base.Add(48*time.Hour), collect.StatusSuccess)
	// other tenant capture must not leak in
	other := &collect.CollectTask{TenantID: 2, Source: "1688", SourceURL: url, Status: collect.StatusSuccess, FinishedAt: &base}
	if err := db.Create(other).Error; err != nil {
		t.Fatal(err)
	}

	srv := newInsightsRouter(db, uid, 1)
	defer srv.Close()
	var dto PriceTrendDTO
	if code := getEnv(t, srv, "/api/v1/selection/candidates/"+cand.ID.String()+"/price-trend", &dto); code != http.StatusOK {
		t.Fatalf("trend: got %d", code)
	}
	if len(dto.Points) != 3 {
		t.Fatalf("points: got %d, want 3 (%+v)", len(dto.Points), dto.Points)
	}
	if dto.Points[0].Price != 12.5 || dto.Points[2].Price != 13.2 {
		t.Fatalf("point order/prices wrong: %+v", dto.Points)
	}
	if dto.Currency != "CNY" {
		t.Fatalf("currency: %q", dto.Currency)
	}
}

// compare must reject <2 ids, join supply readiness from货源档案 and report
// banned-word risk on titles.
func TestCompareCandidates(t *testing.T) {
	db := newInsightsDB(t)
	uid := seedAdmin(t, db, "admin")
	url := "https://detail.1688.com/offer/2.html"
	c1 := seedScoredCandidate(t, db, 1, url)
	c2 := seedScoredCandidate(t, db, 1, "")
	c2.Title = "最强 顶级 宠物玩具"
	if err := db.Save(c2).Error; err != nil {
		t.Fatal(err)
	}

	sup := &sourcing.Supplier{TenantID: 1, Name: "深圳供应商", Status: sourcing.SupplierStatusActive}
	if err := db.Create(sup).Error; err != nil {
		t.Fatal(err)
	}
	src := &sourcing.ProductSource{TenantID: 1, ProductID: uuid.New(), SupplierID: sup.ID, SourceURL: url, Status: sourcing.SourceStatusActive}
	if err := db.Create(src).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Create(&bannedwords.BannedWord{TenantID: 1, Word: "最强", Category: "general", Level: bannedwords.LevelForbidden, Enabled: true}).Error; err != nil {
		t.Fatal(err)
	}

	srv := httptest.NewServer(newEngineWith(uid, 1, &Service{DB: db, Banned: &bannedwords.Service{DB: db}}))
	defer srv.Close()

	// <2 ids → 400
	if code := getEnv(t, srv, "/api/v1/selection/compare?ids="+c1.ID.String(), nil); code != http.StatusBadRequest {
		t.Fatalf("single id: got %d, want 400", code)
	}

	var rows []CompareRowDTO
	if code := getEnv(t, srv, "/api/v1/selection/compare?ids="+c1.ID.String()+","+c2.ID.String(), &rows); code != http.StatusOK {
		t.Fatalf("compare: got %d", code)
	}
	if len(rows) != 2 {
		t.Fatalf("rows: %d", len(rows))
	}
	if !rows[0].Supply.Ready || rows[0].Supply.SupplierName != "深圳供应商" {
		t.Fatalf("c1 supply: %+v", rows[0].Supply)
	}
	if rows[1].Supply.Ready {
		t.Fatalf("c2 supply should not be ready: %+v", rows[1].Supply)
	}
	if rows[1].Banned.ForbiddenCount < 1 {
		t.Fatalf("c2 banned: %+v", rows[1].Banned)
	}
	if rows[0].Banned.ForbiddenCount != 0 {
		t.Fatalf("c1 banned: %+v", rows[0].Banned)
	}
}

// facts extraction: prices from SKUs, sales/review keys tolerant, garbage safe.
func TestExtractFacts(t *testing.T) {
	raw := []byte(`{"currency":"CNY","skus":[{"price":10.5},{"price":8.2}],"attributes":{"monthlySold":"1200","reviewCount":56}}`)
	price, currency := extractPriceFact(raw)
	if price == nil || *price != 8.2 || currency != "CNY" {
		t.Fatalf("price: %v %q", price, currency)
	}
	if v := extractIntFact(raw, salesKeys); v == nil || *v != 1200 {
		t.Fatalf("sales: %v", v)
	}
	if v := extractIntFact(raw, reviewKeys); v == nil || *v != 56 {
		t.Fatalf("review: %v", v)
	}
	if p, _ := extractPriceFact([]byte("not-json")); p != nil {
		t.Fatal("garbage must yield nil price")
	}
	if v := extractIntFact([]byte(`{}`), salesKeys); v != nil {
		t.Fatal("missing keys must yield nil (未采集)")
	}
}
