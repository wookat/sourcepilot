package settings

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/trademind-ai/trademind/backend/internal/modules/operationlog"
	"github.com/trademind-ai/trademind/backend/internal/pkg/response"
	"github.com/trademind-ai/trademind/backend/internal/providers/fxrate"
	"gorm.io/gorm"
)

// EnsureReportCurrencyDefaults inserts report currency keys when missing
// (idempotent). Rates default to an empty table: currencies without a manual
// rate are reported as "unconverted" instead of silently mis-converted.
func EnsureReportCurrencyDefaults(ctx context.Context, db *gorm.DB) error {
	if db == nil {
		return nil
	}
	defs := []struct{ key, val string }{
		{fxrate.KeyProvider, fxrate.ProviderManual},
		{fxrate.KeyBaseCurrency, fxrate.DefaultBaseCurrency},
		{fxrate.KeyRates, "{}"},
	}
	for _, d := range defs {
		var n int64
		if err := db.WithContext(ctx).Model(&Setting{}).
			Where("tenant_id = ? AND group_key = ? AND item_key = ?", 0, fxrate.SettingsGroup, d.key).
			Count(&n).Error; err != nil {
			return err
		}
		if n > 0 {
			continue
		}
		row := Setting{TenantID: 0, GroupKey: fxrate.SettingsGroup, ItemKey: d.key,
			ItemValue: d.val, ValueType: "string", IsEncrypted: false}
		if err := db.WithContext(ctx).Create(&row).Error; err != nil {
			return err
		}
	}
	return nil
}

// ReportCurrencyRate is one manual rate row (1 currency = rate base units).
type ReportCurrencyRate struct {
	Currency string `json:"currency"`
	Rate     string `json:"rate"`
}

// ReportCurrencyDTO is GET/PUT /api/v1/settings/report-currency.
type ReportCurrencyDTO struct {
	Provider     string               `json:"provider"`
	BaseCurrency string               `json:"baseCurrency"`
	Rates        []ReportCurrencyRate `json:"rates"`
}

// GetReportCurrency GET /api/v1/settings/report-currency
func (h *Handler) GetReportCurrency(c *gin.Context) {
	if !h.requireSettingsManage(c) {
		return
	}
	m, err := h.Svc.PlainByGroup(c.Request.Context(), 0, fxrate.SettingsGroup)
	if err != nil {
		response.HandleError(c, err)
		return
	}
	response.OK(c, reportCurrencyDTOFromPlain(m))
}

func reportCurrencyDTOFromPlain(m map[string]string) ReportCurrencyDTO {
	out := ReportCurrencyDTO{
		Provider:     strings.TrimSpace(m[fxrate.KeyProvider]),
		BaseCurrency: strings.ToUpper(strings.TrimSpace(m[fxrate.KeyBaseCurrency])),
		Rates:        []ReportCurrencyRate{},
	}
	if out.Provider == "" {
		out.Provider = fxrate.ProviderManual
	}
	if !fxrate.ValidCurrencyCode(out.BaseCurrency) {
		out.BaseCurrency = fxrate.DefaultBaseCurrency
	}
	rates := fxrate.ParseRatesJSON(m[fxrate.KeyRates])
	codes := make([]string, 0, len(rates))
	for code := range rates {
		codes = append(codes, code)
	}
	sort.Strings(codes)
	for _, code := range codes {
		out.Rates = append(out.Rates, ReportCurrencyRate{Currency: code, Rate: fxrate.TrimRate(rates[code])})
	}
	return out
}

type putReportCurrencyBody struct {
	BaseCurrency string               `json:"baseCurrency" binding:"required,max=8"`
	Rates        []ReportCurrencyRate `json:"rates" binding:"omitempty,dive"`
}

// PutReportCurrency PUT /api/v1/settings/report-currency
func (h *Handler) PutReportCurrency(c *gin.Context) {
	if !h.requireSettingsManage(c) {
		return
	}
	var body putReportCurrencyBody
	if err := c.ShouldBindJSON(&body); err != nil {
		response.Fail(c, 400, response.CodeBadRequest, "invalid body")
		return
	}
	base := strings.ToUpper(strings.TrimSpace(body.BaseCurrency))
	if !fxrate.ValidCurrencyCode(base) {
		response.Fail(c, 400, response.CodeBadRequest, "本位币必须是 3 位货币代码（如 CNY/USD）")
		return
	}
	if len(body.Rates) > fxrate.MaxRates {
		response.Fail(c, 400, response.CodeBadRequest, fmt.Sprintf("汇率表最多 %d 条", fxrate.MaxRates))
		return
	}
	rates := map[string]string{}
	for _, r := range body.Rates {
		code := strings.ToUpper(strings.TrimSpace(r.Currency))
		if !fxrate.ValidCurrencyCode(code) {
			response.Fail(c, 400, response.CodeBadRequest, fmt.Sprintf("币种代码不合法：%s", r.Currency))
			return
		}
		if code == base {
			response.Fail(c, 400, response.CodeBadRequest, "本位币无需配置汇率（恒为 1）")
			return
		}
		if _, dup := rates[code]; dup {
			response.Fail(c, 400, response.CodeBadRequest, fmt.Sprintf("币种重复：%s", code))
			return
		}
		if _, ok := fxrate.ParseRate(r.Rate); !ok {
			response.Fail(c, 400, response.CodeBadRequest, fmt.Sprintf("汇率不合法（需为正的十进制数）：%s=%s", code, r.Rate))
			return
		}
		rates[code] = strings.TrimSpace(r.Rate)
	}
	raw, err := json.Marshal(rates)
	if err != nil {
		response.HandleError(c, err)
		return
	}
	items := []PutItem{
		{TenantID: 0, GroupKey: fxrate.SettingsGroup, ItemKey: fxrate.KeyProvider, ItemValue: fxrate.ProviderManual, ValueType: "string"},
		{TenantID: 0, GroupKey: fxrate.SettingsGroup, ItemKey: fxrate.KeyBaseCurrency, ItemValue: base, ValueType: "string"},
		{TenantID: 0, GroupKey: fxrate.SettingsGroup, ItemKey: fxrate.KeyRates, ItemValue: string(raw), ValueType: "string"},
	}
	if err := h.Svc.PutBulk(c.Request.Context(), items); err != nil {
		response.Fail(c, 400, response.CodeBadRequest, err.Error())
		return
	}
	if h.OpLog != nil {
		_ = h.OpLog.Write(c, operationlog.WriteOpts{
			Action:   "settings.report_currency.update",
			Resource: "settings",
			Status:   "success",
			Message:  fmt.Sprintf("base=%s rates=%d", base, len(rates)),
		})
	}
	m, err := h.Svc.PlainByGroup(c.Request.Context(), 0, fxrate.SettingsGroup)
	if err != nil {
		response.HandleError(c, err)
		return
	}
	response.OK(c, reportCurrencyDTOFromPlain(m))
}
