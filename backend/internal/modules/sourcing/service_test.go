package sourcing

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/glebarez/sqlite"
	"github.com/google/uuid"
	"github.com/trademind-ai/trademind/backend/internal/providers/sourceinfo"
	"gorm.io/gorm"
)

func openTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	dsn := fmt.Sprintf("file:sourcing_%s?mode=memory&cache=shared", uuid.NewString())
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Skipf("sqlite unavailable: %v", err)
	}
	if err := db.AutoMigrate(&Supplier{}, &ProductSource{}, &ProductSourceSKU{}, &SourcePriceHistory{}, &SourceSwitchEvent{}); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	return db
}

func newTestService(t *testing.T) *Service {
	return &Service{DB: openTestDB(t), Provider: &sourceinfo.Mock{}}
}

func mustBind(t *testing.T, svc *Service, productID uuid.UUID, name, offer string, priority int) *ProductSource {
	t.Helper()
	src, err := svc.BindSource(context.Background(), productID, BindSourceBody{
		SupplierName: name,
		SourceURL:    "https://detail.1688.com/offer/" + offer + ".html",
		Priority:     &priority,
	}, nil)
	if err != nil {
		t.Fatalf("bind source: %v", err)
	}
	return src
}

func TestBindSourceFirstBecomesPrimaryAndDuplicateRejected(t *testing.T) {
	svc := newTestService(t)
	productID := uuid.New()
	first := mustBind(t, svc, productID, "supplier-a", "111", 10)
	if !first.IsPrimary {
		t.Fatalf("first source should be primary")
	}
	second := mustBind(t, svc, productID, "supplier-b", "222", 20)
	if second.IsPrimary {
		t.Fatalf("second source should not be primary")
	}
	// same supplier + offer again → conflict
	p := 30
	_, err := svc.BindSource(context.Background(), productID, BindSourceBody{
		SupplierName: "supplier-a",
		SourceURL:    "https://detail.1688.com/offer/111.html",
		Priority:     &p,
	}, nil)
	if err == nil {
		t.Fatalf("duplicate bind should fail")
	}
}

func TestSetPrimaryWritesSwitchEvent(t *testing.T) {
	svc := newTestService(t)
	productID := uuid.New()
	a := mustBind(t, svc, productID, "supplier-a", "111", 10)
	b := mustBind(t, svc, productID, "supplier-b", "222", 20)

	out, err := svc.SetPrimary(context.Background(), b.ID, nil)
	if err != nil {
		t.Fatalf("set primary: %v", err)
	}
	if !out.IsPrimary {
		t.Fatalf("target should be primary")
	}
	var prev ProductSource
	if err := svc.DB.First(&prev, "id = ?", a.ID).Error; err != nil {
		t.Fatal(err)
	}
	if prev.IsPrimary {
		t.Fatalf("previous primary should be demoted")
	}
	var events []SourceSwitchEvent
	if err := svc.DB.Where("product_id = ?", productID).Find(&events).Error; err != nil {
		t.Fatal(err)
	}
	if len(events) != 1 || events[0].Reason != SwitchReasonManual || events[0].Mode != SwitchModeManual {
		t.Fatalf("expected 1 manual switch event, got %+v", events)
	}
}

func TestSaveSKUMappingsWritesPriceHistory(t *testing.T) {
	svc := newTestService(t)
	productID := uuid.New()
	src := mustBind(t, svc, productID, "supplier-a", "111", 10)
	localSKU := uuid.New()
	price := 12.5
	rows, err := svc.SaveSKUMappings(context.Background(), src.ID, []SKUMappingBody{
		{LocalSKUID: localSKU.String(), ExternalSKUID: "ext-1", CurrentPrice: &price},
	}, nil)
	if err != nil {
		t.Fatalf("save mappings: %v", err)
	}
	if len(rows) != 1 || rows[0].CurrentPrice == nil || *rows[0].CurrentPrice != 12.5 {
		t.Fatalf("unexpected mapping: %+v", rows)
	}
	// same price again should not append another history row
	if _, err := svc.SaveSKUMappings(context.Background(), src.ID, []SKUMappingBody{
		{LocalSKUID: localSKU.String(), ExternalSKUID: "ext-1", CurrentPrice: &price},
	}, nil); err != nil {
		t.Fatal(err)
	}
	hist, err := svc.PriceHistory(context.Background(), rows[0].ID, 90)
	if err != nil {
		t.Fatal(err)
	}
	if len(hist) != 1 || hist[0].CaptureSource != CaptureSourceManual {
		t.Fatalf("expected 1 manual history row, got %+v", hist)
	}
}

func TestApplySwitchRulesOutOfStockAutoSwitch(t *testing.T) {
	svc := newTestService(t)
	productID := uuid.New()
	a := mustBind(t, svc, productID, "supplier-a", "111", 10)
	b := mustBind(t, svc, productID, "supplier-b", "222", 20)

	if err := svc.DB.Model(&ProductSource{}).Where("id = ?", a.ID).Update("status", SourceStatusOutOfStock).Error; err != nil {
		t.Fatal(err)
	}
	sources, err := svc.ListProductSources(context.Background(), productID)
	if err != nil {
		t.Fatal(err)
	}
	switched, _, err := svc.applySwitchRules(context.Background(), productID, sources, defaultRuleConfig())
	if err != nil {
		t.Fatal(err)
	}
	if switched == nil || switched.ID != b.ID {
		t.Fatalf("expected auto switch to backup, got %+v", switched)
	}
	var ev SourceSwitchEvent
	if err := svc.DB.Where("product_id = ?", productID).First(&ev).Error; err != nil {
		t.Fatal(err)
	}
	if ev.Reason != SwitchReasonOutOfStock || ev.Mode != SwitchModeAuto {
		t.Fatalf("unexpected event %+v", ev)
	}
}

func TestApplySwitchRulesLockedPrimaryNotSwitched(t *testing.T) {
	svc := newTestService(t)
	productID := uuid.New()
	a := mustBind(t, svc, productID, "supplier-a", "111", 10)
	mustBind(t, svc, productID, "supplier-b", "222", 20)

	if err := svc.DB.Model(&ProductSource{}).Where("id = ?", a.ID).
		Updates(map[string]any{"status": SourceStatusOutOfStock, "locked": true}).Error; err != nil {
		t.Fatal(err)
	}
	sources, err := svc.ListProductSources(context.Background(), productID)
	if err != nil {
		t.Fatal(err)
	}
	switched, alerts, err := svc.applySwitchRules(context.Background(), productID, sources, defaultRuleConfig())
	if err != nil {
		t.Fatal(err)
	}
	if switched != nil {
		t.Fatalf("locked primary must not be auto-switched")
	}
	if len(alerts) == 0 {
		t.Fatalf("expected a lock alert")
	}
	var cnt int64
	if err := svc.DB.Model(&SourceSwitchEvent{}).Where("product_id = ?", productID).Count(&cnt).Error; err != nil {
		t.Fatal(err)
	}
	if cnt != 0 {
		t.Fatalf("no switch events expected for locked primary, got %d", cnt)
	}
}

func TestApplySwitchRulesPriceAlertSuggestsOnly(t *testing.T) {
	svc := newTestService(t)
	productID := uuid.New()
	a := mustBind(t, svc, productID, "supplier-a", "111", 10)
	b := mustBind(t, svc, productID, "supplier-b", "222", 20)

	if err := svc.DB.Model(&ProductSource{}).Where("id = ?", a.ID).Update("status", SourceStatusPriceAlert).Error; err != nil {
		t.Fatal(err)
	}
	sources, err := svc.ListProductSources(context.Background(), productID)
	if err != nil {
		t.Fatal(err)
	}
	switched, _, err := svc.applySwitchRules(context.Background(), productID, sources, defaultRuleConfig())
	if err != nil {
		t.Fatal(err)
	}
	if switched != nil {
		t.Fatalf("price alert should only suggest by default")
	}
	var ev SourceSwitchEvent
	if err := svc.DB.Where("product_id = ?", productID).First(&ev).Error; err != nil {
		t.Fatal(err)
	}
	if ev.Mode != SwitchModeSuggested || ev.Reason != SwitchReasonPriceIncrease || ev.ToSourceID != b.ID {
		t.Fatalf("unexpected suggestion event %+v", ev)
	}
}

func TestMockProviderDeterministic(t *testing.T) {
	fixed := time.Date(2026, 1, 2, 3, 0, 0, 0, time.UTC)
	m := &sourceinfo.Mock{Now: func() time.Time { return fixed }}
	q1, err := m.FetchOffer(context.Background(), "offer-1", []string{"sku-1", "sku-2"})
	if err != nil {
		t.Fatal(err)
	}
	q2, err := m.FetchOffer(context.Background(), "offer-1", []string{"sku-1", "sku-2"})
	if err != nil {
		t.Fatal(err)
	}
	if len(q1.SKUs) != 2 || len(q2.SKUs) != 2 {
		t.Fatalf("expected 2 skus")
	}
	for i := range q1.SKUs {
		if q1.SKUs[i] != q2.SKUs[i] {
			t.Fatalf("mock must be deterministic for a fixed time")
		}
	}
}

func TestExtractOfferID(t *testing.T) {
	if got := extractOfferID("https://detail.1688.com/offer/123456789.html"); got != "123456789" {
		t.Fatalf("got %q", got)
	}
	if got := extractOfferID("https://example.com/x"); got != "" {
		t.Fatalf("got %q", got)
	}
}
