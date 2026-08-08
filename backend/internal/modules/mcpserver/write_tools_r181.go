package mcpserver

import (
	"context"
	"errors"
	"fmt"
	"math"
	"strconv"
	"strings"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/trademind-ai/trademind/backend/internal/modules/mcptoken"
	"github.com/trademind-ai/trademind/backend/internal/modules/mcpwrite"
	"github.com/trademind-ai/trademind/backend/internal/modules/procurement"
	"github.com/trademind-ai/trademind/backend/internal/modules/settings"
	"gorm.io/gorm"
)

// R181 W3 whitelist action: procurement_mark_paid (placed → paid manual
// payment backfill). It runs through the W1 governed pipeline and adds three
// fail-closed preconditions on top (per the decision brief D1/W5):
//  1. tenant amount ceilings — per-call limit and daily cumulative limit,
//     both default 0 = tool unavailable until an admin configures them;
//  2. the dry-run preview must echo the amount, currency and the purchase
//     order line details;
//  3. the caller-supplied amount/currency must exactly match the purchase
//     order — any mismatch is rejected before anything mutates.
//
// No real money moves: the tool only records that the operator already paid
// the 1688 order by hand (same semantics as the admin mark-paid route).

// ToolProcurementMarkPaid is the W3 write tool name.
const ToolProcurementMarkPaid = "procurement_mark_paid"

// Tenant settings keys (group mcp, admin-only PUT) holding the mark-paid
// amount ceilings. Absent, unparsable or non-positive values keep the tool
// unavailable (fail closed).
const (
	SettingsKeyMarkPaidSingleLimit = "mark_paid_single_limit"
	SettingsKeyMarkPaidDailyLimit  = "mark_paid_daily_limit"
)

// Mark-paid rejections (403 semantics on the MCP surface, audited).
var (
	ErrMarkPaidNotConfigured = errors.New("mark-paid 金额上限未配置（需管理员在租户设置 mcp/mark_paid_single_limit 与 mcp/mark_paid_daily_limit 配置正数上限后方可使用）")
	ErrMarkPaidOverSingle    = errors.New("mark-paid 金额超过租户单笔上限")
	ErrMarkPaidOverDaily     = errors.New("mark-paid 金额将超过租户当日累计上限")
	ErrMarkPaidAmountBad     = errors.New("amount 非法：必须为大于 0、至多两位小数的有效金额")
	ErrMarkPaidMismatch      = errors.New("金额或币种与采购单不一致，已拒绝（请以采购单实际金额与币种重新调用）")
)

// ProcurementMarkPaidIn is the input of procurement_mark_paid. The caller
// must restate the purchase order's amount and currency; a mismatch rejects.
type ProcurementMarkPaidIn struct {
	PurchaseOrderID   string  `json:"purchaseOrderId" jsonschema:"采购单 ID（UUID，必填）"`
	Amount            float64 `json:"amount" jsonschema:"支付金额（必填，必须与采购单总金额完全一致，至多两位小数）"`
	Currency          string  `json:"currency" jsonschema:"币种（必填，必须与采购单币种一致，如 CNY）"`
	PayChannel        string  `json:"payChannel,omitempty" jsonschema:"支付渠道（可选，默认 manual，最长 32 字符）"`
	Mode              string  `json:"mode" jsonschema:"dry_run（预览并领取确认 token）或 execute（携确认 token 执行）"`
	ConfirmationToken string  `json:"confirmationToken,omitempty" jsonschema:"execute 时必填：dry_run 返回的一次性确认 token"`
}

// MarkPaidItemPreview is one purchase order line echoed in the dry-run
// preview so the human confirms exactly what the payment covers.
type MarkPaidItemPreview struct {
	ProductTitle string   `json:"productTitle,omitempty"`
	SKUName      string   `json:"skuName,omitempty"`
	Quantity     int      `json:"quantity"`
	UnitPrice    *float64 `json:"unitPrice,omitempty"`
}

// MarkPaidPreview is the dry-run impact preview: amount, currency, ceilings
// and the full order line details (brief precondition 2).
type MarkPaidPreview struct {
	PurchaseOrderID string                `json:"purchaseOrderId"`
	SupplierName    string                `json:"supplierName,omitempty"`
	ExternalOrderID string                `json:"externalOrderId,omitempty"`
	CurrentStatus   string                `json:"currentStatus"`
	TargetStatus    string                `json:"targetStatus"`
	Amount          float64               `json:"amount"`
	Currency        string                `json:"currency"`
	PayChannel      string                `json:"payChannel"`
	SingleLimit     float64               `json:"singleLimit"`
	DailyLimit      float64               `json:"dailyLimit"`
	DailyUsed       float64               `json:"dailyUsed"`
	Items           []MarkPaidItemPreview `json:"items"`
}

// MarkPaidResult is the execute outcome.
type MarkPaidResult struct {
	PurchaseOrderID string  `json:"purchaseOrderId"`
	Status          string  `json:"status"`
	PayStatus       string  `json:"payStatus"`
	PayChannel      string  `json:"payChannel"`
	Amount          float64 `json:"amount"`
	Currency        string  `json:"currency"`
}

// registerWriteToolsR181 mounts the W3 whitelist action (same preconditions
// as registerWriteTools).
func registerWriteToolsR181(srv *mcp.Server, d *Deps, tok *mcptoken.Token) {
	if d.Procurement == nil {
		return
	}
	mcp.AddTool(srv, &mcp.Tool{
		Name:        ToolProcurementMarkPaid,
		Description: "登记当前租户一张采购单为已支付（placed → paid 状态机校验；不动真实资金，仅回填人工付款事实；写白名单动作）。前提：管理员已配置租户单笔与日累计金额上限；amount/currency 必须与采购单完全一致。必须先以 mode=dry_run 调用获取金额与订单明细预览及一次性确认 token，再以 mode=execute 携 token 执行。",
	}, procurementMarkPaidTool(d, tok))
}

// amountCents converts a money value to integer cents for exact comparison;
// ok is false when the value is not a valid two-decimal amount.
func amountCents(v float64) (int64, bool) {
	if math.IsNaN(v) || math.IsInf(v, 0) || v <= 0 || v > 1e10 {
		return 0, false
	}
	scaled := v * 100
	rounded := math.Round(scaled)
	if math.Abs(scaled-rounded) > 1e-6 {
		return 0, false
	}
	return int64(rounded), true
}

// markPaidLimits reads the tenant mark-paid ceilings on the caller's handle
// (the execute transaction re-reads them, so they cannot drift between
// preview and execute). Absent, unparsable or non-positive values report the
// tool as not configured (fail closed). The two keys are plain numeric
// values; rows flagged encrypted are treated as unconfigured.
func markPaidLimits(ctx context.Context, db *gorm.DB, tenantID int64) (single, daily float64, err error) {
	var rows []settings.Setting
	if err := db.WithContext(ctx).
		Where("tenant_id = ? AND group_key = ? AND item_key IN ? AND is_encrypted = ?",
			tenantID, mcpwrite.SettingsGroupMCP,
			[]string{SettingsKeyMarkPaidSingleLimit, SettingsKeyMarkPaidDailyLimit}, false).
		Find(&rows).Error; err != nil {
		return 0, 0, fmt.Errorf("%w（租户配置读取失败）", ErrMarkPaidNotConfigured)
	}
	vals := make(map[string]string, len(rows))
	for _, row := range rows {
		vals[row.ItemKey] = row.ItemValue
	}
	parse := func(key string) (float64, bool) {
		v, perr := strconv.ParseFloat(strings.TrimSpace(vals[key]), 64)
		if perr != nil || math.IsNaN(v) || math.IsInf(v, 0) || v <= 0 {
			return 0, false
		}
		return v, true
	}
	single, okS := parse(SettingsKeyMarkPaidSingleLimit)
	daily, okD := parse(SettingsKeyMarkPaidDailyLimit)
	if !okS || !okD {
		return 0, 0, ErrMarkPaidNotConfigured
	}
	return single, daily, nil
}

func procurementMarkPaidTool(d *Deps, tok *mcptoken.Token) mcp.ToolHandlerFor[ProcurementMarkPaidIn, *mcpwrite.Result] {
	return func(ctx context.Context, _ *mcp.CallToolRequest, in ProcurementMarkPaidIn) (*mcp.CallToolResult, *mcpwrite.Result, error) {
		currency := strings.ToUpper(strings.TrimSpace(in.Currency))
		payChannel := strings.TrimSpace(in.PayChannel)
		if payChannel == "" {
			payChannel = "manual"
		}
		if strings.TrimSpace(in.PurchaseOrderID) == "" || currency == "" {
			return nil, nil, fmt.Errorf("purchaseOrderId、amount 与 currency 均为必填")
		}
		if len(payChannel) > 32 {
			return nil, nil, fmt.Errorf("payChannel 过长（最长 32 字符）")
		}
		inCents, ok := amountCents(in.Amount)
		if !ok {
			return nil, nil, ErrMarkPaidAmountBad
		}
		poID, err := parsePOID(in.PurchaseOrderID)
		if err != nil {
			return nil, nil, err
		}
		amountStr := strconv.FormatFloat(float64(inCents)/100, 'f', 2, 64)
		// validate re-runs every precondition on the given handle: it is
		// called in dry_run for the preview and again inside the execute
		// transaction, so nothing checked at preview time can drift by
		// execute time (ceilings, order state, amount, currency).
		validate := func(ctx context.Context, db *gorm.DB) (*procurement.PurchaseOrder, float64, float64, float64, error) {
			single, daily, err := markPaidLimits(ctx, db, tok.TenantID)
			if err != nil {
				return nil, 0, 0, 0, err
			}
			po, err := d.Procurement.FindPOInTenant(ctx, db, tok.TenantID, poID)
			if err != nil {
				return nil, 0, 0, 0, err
			}
			if !procurement.CanTransition(po.Status, procurement.StatusPaid) {
				return nil, 0, 0, 0, fmt.Errorf("采购单当前状态 %s 不允许登记支付（需要 %s）", po.Status, procurement.StatusPlaced)
			}
			poCents, ok := amountCents(po.TotalAmount)
			if !ok || poCents != inCents || currency != strings.ToUpper(strings.TrimSpace(po.Currency)) {
				return nil, 0, 0, 0, ErrMarkPaidMismatch
			}
			singleCents, ok := amountCents(single)
			if !ok || inCents > singleCents {
				return nil, 0, 0, 0, fmt.Errorf("%w（单笔上限 %.2f，本笔 %s）", ErrMarkPaidOverSingle, single, amountStr)
			}
			since := time.Now().UTC().Add(-24 * time.Hour)
			used, err := d.Audits.SumExecuteAmountByTenantTool(db, tok.TenantID, ToolProcurementMarkPaid, since)
			if err != nil {
				return nil, 0, 0, 0, fmt.Errorf("mark-paid 日累计核算失败，已拒绝（fail closed）: %w", err)
			}
			if usedCents := int64(math.Round(used * 100)); usedCents+inCents > int64(math.Round(daily*100)) {
				return nil, 0, 0, 0, fmt.Errorf("%w（日累计上限 %.2f，已用 %.2f，本笔 %s）", ErrMarkPaidOverDaily, daily, used, amountStr)
			}
			return po, single, daily, used, nil
		}
		var executed *procurement.PurchaseOrder
		req := mcpwrite.Request{
			Caller:            mcpCaller(tok),
			Tool:              ToolProcurementMarkPaid,
			Mode:              strings.TrimSpace(in.Mode),
			ConfirmationToken: strings.TrimSpace(in.ConfirmationToken),
			ParamsCanonical: "purchaseOrderId=" + poID.String() + "\namount=" + amountStr +
				"\ncurrency=" + currency + "\npayChannel=" + payChannel,
			ParamsSummary: "purchaseOrderId=" + poID.String() + " amount=" + amountStr +
				" currency=" + currency + " payChannel=" + payChannel,
			Amount: float64(inCents) / 100,
			DryRun: func(ctx context.Context, db *gorm.DB) (any, string, error) {
				po, single, daily, used, err := validate(ctx, db)
				if err != nil {
					return nil, "", err
				}
				var items []procurement.PurchaseOrderItem
				if err := db.WithContext(ctx).
					Where("purchase_order_id = ? AND tenant_id = ?", po.ID, tok.TenantID).
					Find(&items).Error; err != nil {
					return nil, "", err
				}
				preview := MarkPaidPreview{
					PurchaseOrderID: po.ID.String(),
					SupplierName:    po.SupplierName,
					ExternalOrderID: po.ExternalOrderID,
					CurrentStatus:   po.Status,
					TargetStatus:    procurement.StatusPaid,
					Amount:          float64(inCents) / 100,
					Currency:        currency,
					PayChannel:      payChannel,
					SingleLimit:     single,
					DailyLimit:      daily,
					DailyUsed:       used,
					Items:           make([]MarkPaidItemPreview, 0, len(items)),
				}
				for _, it := range items {
					price := it.ExpectedPrice
					if it.ActualPrice != nil {
						price = it.ActualPrice
					}
					preview.Items = append(preview.Items, MarkPaidItemPreview{
						ProductTitle: it.ProductTitle,
						SKUName:      it.SKUName,
						Quantity:     it.Quantity,
						UnitPrice:    price,
					})
				}
				summary := fmt.Sprintf("将把采购单 %s（%s）登记为已支付：金额 %s %s，%d 个明细行，渠道 %s",
					po.ID, po.SupplierName, amountStr, currency, len(items), payChannel)
				return preview, summary, nil
			},
			Execute: func(ctx context.Context, tx *gorm.DB) (any, string, error) {
				if _, _, _, _, err := validate(ctx, tx); err != nil {
					return nil, "", err
				}
				po, err := d.Procurement.MarkPaidInTenantTx(ctx, tx, tok.TenantID, poID, payChannel)
				if err != nil {
					return nil, "", err
				}
				executed = po
				res := MarkPaidResult{
					PurchaseOrderID: po.ID.String(),
					Status:          po.Status,
					PayStatus:       po.PayStatus,
					PayChannel:      po.PayChannel,
					Amount:          float64(inCents) / 100,
					Currency:        currency,
				}
				return res, fmt.Sprintf("status=%s amount=%s currency=%s payChannel=%s", po.Status, amountStr, currency, po.PayChannel), nil
			},
		}
		out, err := d.writes().Run(ctx, req)
		if err != nil {
			return nil, nil, err
		}
		if executed != nil {
			d.Procurement.AfterMarkPaidCommitted(ctx, executed)
		}
		return nil, out, nil
	}
}
