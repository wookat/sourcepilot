package operationdashboard

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/trademind-ai/trademind/backend/internal/modules/operationlog"
	"github.com/trademind-ai/trademind/backend/internal/modules/settings"
	"github.com/trademind-ai/trademind/backend/internal/pkg/adminperm"
	"github.com/trademind-ai/trademind/backend/internal/pkg/response"
)

// Settings group / item keys for the tenant-level big-screen card layout.
const (
	screenConfigGroup = "dashboard_screen"
	screenConfigKey   = "cards"
)

// Big-screen card keys (the configurable card pool). The default order
// matches the original fixed layout, so tenants without a saved config see
// the exact pre-R156 screen.
const (
	CardKPIOrders = "kpi_orders"
	CardKPISales  = "kpi_sales"
	CardKPIProfit = "kpi_profit"
	CardKPIAlerts = "kpi_alerts"
	CardTodos     = "todos"
	CardFunnel    = "funnel"
	CardTrend     = "trend"
	CardAlerts    = "alerts"
)

// ScreenCardDTO is one card of the big-screen layout config.
type ScreenCardDTO struct {
	Key     string `json:"key"`
	Title   string `json:"title"`
	Enabled bool   `json:"enabled"`
}

// ScreenConfigDTO is GET/PUT /api/v1/dashboard/screen/config.
type ScreenConfigDTO struct {
	Cards []ScreenCardDTO `json:"cards"`
}

var screenCardTitles = map[string]string{
	CardKPIOrders: "今日订单",
	CardKPISales:  "今日销售额",
	CardKPIProfit: "今日毛利",
	CardKPIAlerts: "当前告警",
	CardTodos:     "待办事项",
	CardFunnel:    "订单状态漏斗",
	CardTrend:     "24 小时订单趋势",
	CardAlerts:    "告警滚动列表",
}

var screenCardDefaultOrder = []string{
	CardKPIOrders, CardKPISales, CardKPIProfit, CardKPIAlerts,
	CardTodos, CardFunnel, CardTrend, CardAlerts,
}

// defaultScreenCards returns the default layout: every card enabled in the
// original order.
func defaultScreenCards() []ScreenCardDTO {
	out := make([]ScreenCardDTO, 0, len(screenCardDefaultOrder))
	for _, k := range screenCardDefaultOrder {
		out = append(out, ScreenCardDTO{Key: k, Title: screenCardTitles[k], Enabled: true})
	}
	return out
}

// storedScreenCard is the persisted shape (title stays server-owned).
type storedScreenCard struct {
	Key     string `json:"key"`
	Enabled bool   `json:"enabled"`
}

// normalizeScreenCards validates a card list against the pool: unknown /
// duplicate keys are rejected; cards missing from the list are appended in
// default order as enabled, so a config saved on an older version keeps
// showing newly added cards until the tenant explicitly turns them off.
func normalizeScreenCards(in []storedScreenCard) ([]ScreenCardDTO, error) {
	seen := map[string]bool{}
	out := make([]ScreenCardDTO, 0, len(screenCardDefaultOrder))
	for _, c := range in {
		key := strings.TrimSpace(c.Key)
		if screenCardTitles[key] == "" {
			return nil, fmt.Errorf("未知的大屏卡片：%s", c.Key)
		}
		if seen[key] {
			return nil, fmt.Errorf("大屏卡片重复：%s", key)
		}
		seen[key] = true
		out = append(out, ScreenCardDTO{Key: key, Title: screenCardTitles[key], Enabled: c.Enabled})
	}
	for _, k := range screenCardDefaultOrder {
		if !seen[k] {
			out = append(out, ScreenCardDTO{Key: k, Title: screenCardTitles[k], Enabled: true})
		}
	}
	return out, nil
}

// parseScreenCardsJSON parses the stored config; invalid / empty payloads
// fall back to the default layout (read paths must never fail on bad data).
func parseScreenCardsJSON(raw string) []ScreenCardDTO {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return defaultScreenCards()
	}
	var stored []storedScreenCard
	if err := json.Unmarshal([]byte(raw), &stored); err != nil || len(stored) == 0 {
		return defaultScreenCards()
	}
	cards, err := normalizeScreenCards(stored)
	if err != nil {
		return defaultScreenCards()
	}
	return cards
}

// enabledCardSet indexes the enabled cards for aggregation skipping.
func enabledCardSet(cards []ScreenCardDTO) map[string]bool {
	out := make(map[string]bool, len(cards))
	for _, c := range cards {
		if c.Enabled {
			out[c.Key] = true
		}
	}
	return out
}

// loadScreenCards resolves the tenant's saved card layout (default layout
// when unset / unreadable — the read path never fails on config problems).
func (h *Handler) loadScreenCards(ctx context.Context, tenantID int64) []ScreenCardDTO {
	if h == nil || h.Settings == nil {
		return defaultScreenCards()
	}
	m, err := h.Settings.PlainByGroup(ctx, tenantID, screenConfigGroup)
	if err != nil {
		return defaultScreenCards()
	}
	return parseScreenCardsJSON(m[screenConfigKey])
}

// GetScreenConfig GET /api/v1/dashboard/screen/config — readable by every
// role (readonly included): it only describes which cards the screen shows.
func (h *Handler) GetScreenConfig(c *gin.Context) {
	if h == nil || h.Svc == nil {
		response.Fail(c, 500, response.CodeInternalError, "dashboard unavailable")
		return
	}
	tenantID, err := adminperm.TenantIDFromGin(c)
	if err != nil {
		response.Fail(c, 403, response.CodePermissionDenied, "tenant context missing")
		return
	}
	response.OK(c, ScreenConfigDTO{Cards: h.loadScreenCards(c.Request.Context(), tenantID)})
}

type putScreenConfigBody struct {
	Cards []storedScreenCard `json:"cards" binding:"required,min=1,dive"`
}

// PutScreenConfig PUT /api/v1/dashboard/screen/config — settings.manage
// only (readonly / operator are rejected), tenant scoped.
func (h *Handler) PutScreenConfig(c *gin.Context) {
	if h == nil || h.Svc == nil || h.Settings == nil {
		response.Fail(c, 500, response.CodeInternalError, "dashboard unavailable")
		return
	}
	if !adminperm.RequireWrite(c, h.Svc.DB, adminperm.PermSettingsManage) {
		return
	}
	tenantID, err := adminperm.TenantIDFromGin(c)
	if err != nil {
		response.Fail(c, 403, response.CodePermissionDenied, "tenant context missing")
		return
	}
	var body putScreenConfigBody
	if err := c.ShouldBindJSON(&body); err != nil {
		response.Fail(c, 400, response.CodeBadRequest, "invalid body")
		return
	}
	cards, err := normalizeScreenCards(body.Cards)
	if err != nil {
		response.Fail(c, 400, response.CodeBadRequest, err.Error())
		return
	}
	enabled := 0
	stored := make([]storedScreenCard, 0, len(cards))
	for _, card := range cards {
		if card.Enabled {
			enabled++
		}
		stored = append(stored, storedScreenCard{Key: card.Key, Enabled: card.Enabled})
	}
	if enabled == 0 {
		response.Fail(c, 400, response.CodeBadRequest, "至少保留一张大屏卡片")
		return
	}
	raw, err := json.Marshal(stored)
	if err != nil {
		response.HandleError(c, err)
		return
	}
	items := []settings.PutItem{{
		TenantID: tenantID, GroupKey: screenConfigGroup, ItemKey: screenConfigKey,
		ItemValue: string(raw), ValueType: "json",
	}}
	if err := h.Settings.PutBulk(c.Request.Context(), items); err != nil {
		response.Fail(c, 400, response.CodeBadRequest, err.Error())
		return
	}
	if h.OpLog != nil {
		_ = h.OpLog.Write(c, operationlog.WriteOpts{
			Action:   "dashboard.screen_config.update",
			Resource: "dashboard",
			Status:   "success",
			Message:  fmt.Sprintf("cards=%d enabled=%d", len(cards), enabled),
		})
	}
	response.OK(c, ScreenConfigDTO{Cards: cards})
}
