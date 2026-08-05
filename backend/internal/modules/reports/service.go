package reports

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/trademind-ai/trademind/backend/internal/modules/procurement"
	"github.com/trademind-ai/trademind/backend/internal/providers/fxrate"
	"gorm.io/gorm"
)

// Service aggregates deep reports. Proc resolves reference line costs (CNY);
// Settings resolves the tenant fx table and the configurable profit fee items.
type Service struct {
	DB       *gorm.DB
	Settings fxrate.SettingsReader
	Proc     *procurement.Service
}

const (
	rangeDefaultDays = 30
	rangeMaxDays     = 366
)

// DateRange is a resolved inclusive local-day range.
type DateRange struct {
	Start time.Time // local midnight of the first day
	End   time.Time // exclusive: local midnight after the last day
	Days  int
}

func (r DateRange) StartDate() string { return r.Start.Format("2006-01-02") }
func (r DateRange) EndDate() string   { return r.End.AddDate(0, 0, -1).Format("2006-01-02") }

// ResolveRange resolves days=N or start/end=YYYY-MM-DD (inclusive, local
// days) into a bounded range; invalid input falls back to the last 30 days.
func ResolveRange(days int, start, end string) (DateRange, error) {
	now := time.Now()
	todayStart := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
	if s, e := strings.TrimSpace(start), strings.TrimSpace(end); s != "" || e != "" {
		st, err1 := time.ParseInLocation("2006-01-02", s, now.Location())
		et, err2 := time.ParseInLocation("2006-01-02", e, now.Location())
		if err1 != nil || err2 != nil {
			return DateRange{}, fmt.Errorf("自定义区间格式应为 YYYY-MM-DD")
		}
		if et.Before(st) {
			return DateRange{}, fmt.Errorf("结束日期不能早于开始日期")
		}
		n := int(et.Sub(st).Hours()/24) + 1
		if n > rangeMaxDays {
			return DateRange{}, fmt.Errorf("自定义区间最长 %d 天", rangeMaxDays)
		}
		return DateRange{Start: st, End: et.AddDate(0, 0, 1), Days: n}, nil
	}
	if days <= 0 {
		days = rangeDefaultDays
	}
	if days > rangeMaxDays {
		days = rangeMaxDays
	}
	st := todayStart.AddDate(0, 0, -(days - 1))
	return DateRange{Start: st, End: todayStart.AddDate(0, 0, 1), Days: days}, nil
}

// fxTable resolves the tenant's report base currency / manual rate table;
// never fails (degrades to the default base with an empty table).
func (s *Service) fxTable(ctx context.Context, tenantID int64) *fxrate.Table {
	var reader fxrate.SettingsReader
	if s != nil {
		reader = s.Settings
	}
	p := &fxrate.ManualProvider{Settings: reader}
	tab, err := p.Table(ctx, tenantID)
	if err != nil || tab == nil {
		return fxrate.NewTable(fxrate.DefaultBaseCurrency, nil)
	}
	return tab
}

// Profit fee configuration (settings group report_profit, item fee_items):
// a JSON array of fee items applied on top of revenue and purchase cost.
const (
	FeeSettingsGroup = "report_profit"
	FeeItemsKey      = "fee_items"

	FeeModePercent       = "percent"         // value = percent of converted revenue
	FeeModeFixedPerOrder = "fixed_per_order" // value = base-currency amount per paid order
	maxFeeItems          = 20
)

// FeeItem is one configurable fee line.
type FeeItem struct {
	Name  string  `json:"name"`
	Mode  string  `json:"mode"`
	Value float64 `json:"value"`
}

// feeItems reads and validates the tenant fee configuration; invalid or
// missing configuration degrades to no fees (never fails a report).
func (s *Service) feeItems(ctx context.Context, tenantID int64) []FeeItem {
	if s == nil {
		return nil
	}
	return LoadFeeItems(ctx, s.Settings, tenantID)
}

// LoadFeeItems reads and validates the tenant profit fee configuration for
// any consumer that must apply the same estimated-fee口径 (e.g. the finance
// reconciliation report). Invalid or missing configuration degrades to nil.
func LoadFeeItems(ctx context.Context, settings fxrate.SettingsReader, tenantID int64) []FeeItem {
	if settings == nil {
		return nil
	}
	m, err := settings.PlainByGroup(ctx, tenantID, FeeSettingsGroup)
	if err != nil || strings.TrimSpace(m[FeeItemsKey]) == "" {
		return nil
	}
	var raw []FeeItem
	if err := json.Unmarshal([]byte(m[FeeItemsKey]), &raw); err != nil {
		return nil
	}
	out := make([]FeeItem, 0, len(raw))
	for _, it := range raw {
		it.Name = strings.TrimSpace(it.Name)
		if it.Name == "" || it.Value < 0 {
			continue
		}
		if it.Mode != FeeModePercent && it.Mode != FeeModeFixedPerOrder {
			continue
		}
		out = append(out, it)
		if len(out) >= maxFeeItems {
			break
		}
	}
	return out
}

// round2 rounds half-up to 2 decimals via exact decimal arithmetic.
func round2(v float64) float64 { return fxrate.Round2(fxrate.AmountRat(v)) }

// parseDBTime normalizes an aggregated (MIN/MAX) timestamp column scanned as
// a string: database/sql renders time.Time as RFC3339Nano; sqlite (tests)
// stores its own literal formats.
func parseDBTime(v string) (time.Time, bool) {
	for _, layout := range []string{time.RFC3339Nano, "2006-01-02 15:04:05.999999999-07:00", "2006-01-02 15:04:05.999999999", "2006-01-02 15:04:05"} {
		if parsed, err := time.Parse(layout, v); err == nil {
			return parsed, true
		}
	}
	return time.Time{}, false
}
