package mcpserver

import (
	"context"
	"fmt"
	"strings"

	"github.com/google/uuid"
	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/trademind-ai/trademind/backend/internal/modules/mcptoken"
	"github.com/trademind-ai/trademind/backend/internal/modules/mcpwrite"
	"github.com/trademind-ai/trademind/backend/internal/modules/orderexception"
	"github.com/trademind-ai/trademind/backend/internal/modules/procurement"
	"gorm.io/gorm"
)

// R180 W2 whitelist actions: exception marking, purchase-order mark-placed
// and logistics tracking backfill. All three run through the W1 governed
// write pipeline (three gates, dry-run → one-time confirmation → execute,
// fail-closed audit, quotas) and answer cross-tenant / missing targets with
// 404 semantics. mark-paid is exposed separately as the R181 W3 action with
// its amount ceilings and preconditions (write_tools_r181.go).

// Exception mark actions accepted by exceptions_mark.
const (
	ExceptionActionHandle = "handle"
	ExceptionActionIgnore = "ignore"
	ExceptionActionUnmark = "unmark"
)

var exceptionSourceTypes = map[string]bool{
	orderexception.SourceOrder:                true,
	orderexception.SourceOrderItem:            true,
	orderexception.SourceOrderItemSKUMatch:    true,
	orderexception.SourceOrderInventoryEffect: true,
	orderexception.SourceInventorySyncTask:    true,
	orderexception.SourceOrderSyncTask:        true,
}

var exceptionTypes = map[string]bool{
	orderexception.TypeSKUUnmatched:           true,
	orderexception.TypeSKUAmbiguous:           true,
	orderexception.TypeInsufficientStock:      true,
	orderexception.TypeInventoryDeductFailed:  true,
	orderexception.TypeInventoryRestoreFailed: true,
	orderexception.TypeInventorySyncFailed:    true,
	orderexception.TypeOrderSyncPartialFailed: true,
	orderexception.TypeMissingOrderItem:       true,
	orderexception.TypeProcurementBlocked:     true,
	orderexception.TypeNegativeMargin:         true,
	orderexception.TypeUnknown:                true,
}

// registerWriteToolsR180 mounts the W2 whitelist actions (same preconditions
// as registerWriteTools).
func registerWriteToolsR180(srv *mcp.Server, d *Deps, tok *mcptoken.Token) {
	if d.Exceptions != nil {
		mcp.AddTool(srv, &mcp.Tool{
			Name:        ToolExceptionsMark,
			Description: "标记当前租户的一个异常来源：handle（已处理）/ ignore（已忽略）/ unmark（撤销标记），幂等；写白名单动作。必须先以 mode=dry_run 调用获取影响预览与一次性确认 token，再以 mode=execute 携 token 执行。",
		}, exceptionMarkWriteTool(d, tok))
	}
	if d.Procurement != nil {
		mcp.AddTool(srv, &mcp.Tool{
			Name:        ToolProcurementMarkPlaced,
			Description: "回填当前租户一张采购单的 1688 外部单号并标记为已下单（placing → placed 状态机校验；写白名单动作）。必须先以 mode=dry_run 调用获取影响预览与一次性确认 token，再以 mode=execute 携 token 执行。",
		}, procurementMarkPlacedTool(d, tok))
		mcp.AddTool(srv, &mcp.Tool{
			Name:        ToolProcurementFillLogistics,
			Description: "回填当前租户一张采购单的物流运单号并标记为已发货（paid → shipped 状态机校验；写白名单动作）。必须先以 mode=dry_run 调用获取影响预览与一次性确认 token，再以 mode=execute 携 token 执行。",
		}, procurementFillLogisticsTool(d, tok))
	}
}

func mcpCaller(tok *mcptoken.Token) mcpwrite.Caller {
	return mcpwrite.Caller{
		TenantID:    tok.TenantID,
		TokenID:     tok.ID,
		TokenName:   tok.Name,
		TokenMasked: tok.Masked(),
	}
}

// ExceptionMarkIn is the input of exceptions_mark. One source row per call.
type ExceptionMarkIn struct {
	SourceType        string `json:"sourceType" jsonschema:"异常来源类型（order / order_item / order_item_sku_match / order_inventory_effect / inventory_sync_task / order_sync_task，必填）"`
	SourceID          string `json:"sourceId" jsonschema:"异常来源行 ID（UUID，必填）"`
	ExceptionType     string `json:"exceptionType" jsonschema:"异常类型（如 sku_unmatched / insufficient_stock，必填）"`
	Action            string `json:"action" jsonschema:"handle（标记已处理）/ ignore（标记已忽略）/ unmark（撤销标记）"`
	Remark            string `json:"remark,omitempty" jsonschema:"备注（可选，最长 500 字符）"`
	Mode              string `json:"mode" jsonschema:"dry_run（预览并领取确认 token）或 execute（携确认 token 执行）"`
	ConfirmationToken string `json:"confirmationToken,omitempty" jsonschema:"execute 时必填：dry_run 返回的一次性确认 token"`
}

// ExceptionMarkPreview is the dry-run impact preview.
type ExceptionMarkPreview struct {
	SourceType    string `json:"sourceType"`
	SourceID      string `json:"sourceId"`
	ExceptionType string `json:"exceptionType"`
	CurrentMark   string `json:"currentMark"` // "" (open) / handled / ignored
	TargetMark    string `json:"targetMark"`  // "" (unmark) / handled / ignored
	// Change: mark / unmark / none (idempotent no-op).
	Change string `json:"change"`
}

// ExceptionMarkResult is the execute outcome.
type ExceptionMarkResult struct {
	SourceType    string `json:"sourceType"`
	SourceID      string `json:"sourceId"`
	ExceptionType string `json:"exceptionType"`
	Mark          string `json:"mark"` // resulting mark state: "" / handled / ignored
	// Removed reports how many mark rows an unmark deleted (0 = idempotent no-op).
	Removed int64 `json:"removed"`
}

func exceptionMarkWriteTool(d *Deps, tok *mcptoken.Token) mcp.ToolHandlerFor[ExceptionMarkIn, *mcpwrite.Result] {
	return func(ctx context.Context, _ *mcp.CallToolRequest, in ExceptionMarkIn) (*mcp.CallToolResult, *mcpwrite.Result, error) {
		st := strings.TrimSpace(in.SourceType)
		sid := strings.TrimSpace(in.SourceID)
		et := strings.TrimSpace(in.ExceptionType)
		action := strings.ToLower(strings.TrimSpace(in.Action))
		remark := strings.TrimSpace(in.Remark)
		if st == "" || sid == "" || et == "" {
			return nil, nil, fmt.Errorf("sourceType、sourceId 与 exceptionType 均为必填")
		}
		if !exceptionSourceTypes[st] {
			return nil, nil, fmt.Errorf("sourceType 非法")
		}
		if !exceptionTypes[et] {
			return nil, nil, fmt.Errorf("exceptionType 非法")
		}
		if action != ExceptionActionHandle && action != ExceptionActionIgnore && action != ExceptionActionUnmark {
			return nil, nil, fmt.Errorf("action 非法：可选 handle / ignore / unmark")
		}
		if len([]rune(remark)) > 500 {
			return nil, nil, fmt.Errorf("remark 过长（最长 500 字符）")
		}
		target := ""
		switch action {
		case ExceptionActionHandle:
			target = orderexception.MarkHandled
		case ExceptionActionIgnore:
			target = orderexception.MarkIgnored
		}
		req := mcpwrite.Request{
			Caller:            mcpCaller(tok),
			Tool:              ToolExceptionsMark,
			Mode:              strings.TrimSpace(in.Mode),
			ConfirmationToken: strings.TrimSpace(in.ConfirmationToken),
			ParamsCanonical: "action=" + action + "\nsourceType=" + st + "\nsourceId=" + sid +
				"\nexceptionType=" + et + "\nremark=" + remark,
			ParamsSummary: "action=" + action + " sourceType=" + st + " sourceId=" + sid + " exceptionType=" + et,
			DryRun: func(ctx context.Context, db *gorm.DB) (any, string, error) {
				current, err := d.Exceptions.MarkStateInTenant(ctx, db, tok.TenantID, st, sid)
				if err != nil {
					return nil, "", err
				}
				preview := ExceptionMarkPreview{
					SourceType: st, SourceID: sid, ExceptionType: et,
					CurrentMark: current, TargetMark: target, Change: "none",
				}
				var summary string
				switch {
				case action == ExceptionActionUnmark && current == "":
					summary = fmt.Sprintf("来源 %s/%s 当前无标记，执行为幂等空操作", st, sid)
				case action == ExceptionActionUnmark:
					preview.Change = "unmark"
					summary = fmt.Sprintf("将撤销来源 %s/%s 的「%s」标记", st, sid, current)
				case current == target:
					summary = fmt.Sprintf("来源 %s/%s 已标记为「%s」，执行为幂等空操作（备注仍会更新）", st, sid, target)
				default:
					preview.Change = "mark"
					summary = fmt.Sprintf("将把来源 %s/%s 标记为「%s」（异常类型 %s）", st, sid, target, et)
				}
				return preview, summary, nil
			},
			Execute: func(ctx context.Context, tx *gorm.DB) (any, string, error) {
				res := ExceptionMarkResult{SourceType: st, SourceID: sid, ExceptionType: et}
				if action == ExceptionActionUnmark {
					removed, err := d.Exceptions.DeleteMarksInTenant(ctx, tx, tok.TenantID, st, sid)
					if err != nil {
						return nil, "", err
					}
					res.Removed = removed
					return res, fmt.Sprintf("unmark removed=%d", removed), nil
				}
				if err := d.Exceptions.UpsertMarkInTenant(ctx, tx, tok.TenantID, et, st, sid, target, remark); err != nil {
					return nil, "", err
				}
				res.Mark = target
				return res, fmt.Sprintf("mark=%s", target), nil
			},
		}
		out, err := d.writes().Run(ctx, req)
		if err != nil {
			return nil, nil, err
		}
		return nil, out, nil
	}
}

// ProcurementMarkPlacedIn is the input of procurement_mark_placed.
type ProcurementMarkPlacedIn struct {
	PurchaseOrderID   string `json:"purchaseOrderId" jsonschema:"采购单 ID（UUID，必填）"`
	ExternalOrderID   string `json:"externalOrderId" jsonschema:"1688 外部订单号（必填，最长 64 字符）"`
	Mode              string `json:"mode" jsonschema:"dry_run（预览并领取确认 token）或 execute（携确认 token 执行）"`
	ConfirmationToken string `json:"confirmationToken,omitempty" jsonschema:"execute 时必填：dry_run 返回的一次性确认 token"`
}

// ProcurementWritePreview is the dry-run impact preview of both purchase-order
// write tools.
type ProcurementWritePreview struct {
	PurchaseOrderID string  `json:"purchaseOrderId"`
	SupplierName    string  `json:"supplierName,omitempty"`
	CurrentStatus   string  `json:"currentStatus"`
	TargetStatus    string  `json:"targetStatus"`
	TotalAmount     float64 `json:"totalAmount"`
	ExternalOrderID string  `json:"externalOrderId,omitempty"`
	TrackingNo      string  `json:"trackingNo,omitempty"`
}

// ProcurementWriteResult is the execute outcome of both purchase-order write
// tools.
type ProcurementWriteResult struct {
	PurchaseOrderID string `json:"purchaseOrderId"`
	Status          string `json:"status"`
	ExternalOrderID string `json:"externalOrderId,omitempty"`
	TrackingNo      string `json:"trackingNo,omitempty"`
}

func parsePOID(raw string) (uuid.UUID, error) {
	id, err := uuid.Parse(strings.TrimSpace(raw))
	if err != nil {
		// Unparsable ids answer the same 404 semantics as missing rows.
		return uuid.Nil, procurement.ErrPONotFoundInTenant
	}
	return id, nil
}

func procurementMarkPlacedTool(d *Deps, tok *mcptoken.Token) mcp.ToolHandlerFor[ProcurementMarkPlacedIn, *mcpwrite.Result] {
	return func(ctx context.Context, _ *mcp.CallToolRequest, in ProcurementMarkPlacedIn) (*mcp.CallToolResult, *mcpwrite.Result, error) {
		ext := strings.TrimSpace(in.ExternalOrderID)
		if strings.TrimSpace(in.PurchaseOrderID) == "" || ext == "" {
			return nil, nil, fmt.Errorf("purchaseOrderId 与 externalOrderId 均为必填")
		}
		if len(ext) > 64 {
			return nil, nil, fmt.Errorf("externalOrderId 过长（最长 64 字符）")
		}
		poID, err := parsePOID(in.PurchaseOrderID)
		if err != nil {
			return nil, nil, err
		}
		var executed *procurement.PurchaseOrder
		req := mcpwrite.Request{
			Caller:            mcpCaller(tok),
			Tool:              ToolProcurementMarkPlaced,
			Mode:              strings.TrimSpace(in.Mode),
			ConfirmationToken: strings.TrimSpace(in.ConfirmationToken),
			ParamsCanonical:   "purchaseOrderId=" + poID.String() + "\nexternalOrderId=" + ext,
			ParamsSummary:     "purchaseOrderId=" + poID.String() + " externalOrderId=" + ext,
			DryRun: func(ctx context.Context, db *gorm.DB) (any, string, error) {
				po, err := d.Procurement.FindPOInTenant(ctx, db, tok.TenantID, poID)
				if err != nil {
					return nil, "", err
				}
				if !procurement.CanTransition(po.Status, procurement.StatusPlaced) {
					return nil, "", fmt.Errorf("采购单当前状态 %s 不允许标记为已下单（需要 %s）", po.Status, procurement.StatusPlacing)
				}
				preview := ProcurementWritePreview{
					PurchaseOrderID: po.ID.String(),
					SupplierName:    po.SupplierName,
					CurrentStatus:   po.Status,
					TargetStatus:    procurement.StatusPlaced,
					TotalAmount:     po.TotalAmount,
					ExternalOrderID: ext,
				}
				summary := fmt.Sprintf("将把采购单 %s（%s）标记为已下单，外部单号 %s", po.ID, po.SupplierName, ext)
				return preview, summary, nil
			},
			Execute: func(ctx context.Context, tx *gorm.DB) (any, string, error) {
				po, err := d.Procurement.MarkPlacedInTenantTx(ctx, tx, tok.TenantID, poID, ext)
				if err != nil {
					return nil, "", err
				}
				executed = po
				res := ProcurementWriteResult{PurchaseOrderID: po.ID.String(), Status: po.Status, ExternalOrderID: po.ExternalOrderID}
				return res, fmt.Sprintf("status=%s externalOrderId=%s", po.Status, po.ExternalOrderID), nil
			},
		}
		out, err := d.writes().Run(ctx, req)
		if err != nil {
			return nil, nil, err
		}
		if executed != nil {
			d.Procurement.AfterMarkPlacedCommitted(ctx, executed)
		}
		return nil, out, nil
	}
}

// ProcurementFillLogisticsIn is the input of procurement_fill_logistics.
type ProcurementFillLogisticsIn struct {
	PurchaseOrderID   string `json:"purchaseOrderId" jsonschema:"采购单 ID（UUID，必填）"`
	TrackingNo        string `json:"trackingNo" jsonschema:"物流运单号（必填，最长 64 字符）"`
	Carrier           string `json:"carrier,omitempty" jsonschema:"承运商（可选，最长 64 字符）"`
	Mode              string `json:"mode" jsonschema:"dry_run（预览并领取确认 token）或 execute（携确认 token 执行）"`
	ConfirmationToken string `json:"confirmationToken,omitempty" jsonschema:"execute 时必填：dry_run 返回的一次性确认 token"`
}

func procurementFillLogisticsTool(d *Deps, tok *mcptoken.Token) mcp.ToolHandlerFor[ProcurementFillLogisticsIn, *mcpwrite.Result] {
	return func(ctx context.Context, _ *mcp.CallToolRequest, in ProcurementFillLogisticsIn) (*mcp.CallToolResult, *mcpwrite.Result, error) {
		tn := strings.TrimSpace(in.TrackingNo)
		carrier := strings.TrimSpace(in.Carrier)
		if strings.TrimSpace(in.PurchaseOrderID) == "" || tn == "" {
			return nil, nil, fmt.Errorf("purchaseOrderId 与 trackingNo 均为必填")
		}
		if len(tn) > 64 || len(carrier) > 64 {
			return nil, nil, fmt.Errorf("trackingNo / carrier 过长（最长 64 字符）")
		}
		poID, err := parsePOID(in.PurchaseOrderID)
		if err != nil {
			return nil, nil, err
		}
		var executed *procurement.PurchaseOrder
		req := mcpwrite.Request{
			Caller:            mcpCaller(tok),
			Tool:              ToolProcurementFillLogistics,
			Mode:              strings.TrimSpace(in.Mode),
			ConfirmationToken: strings.TrimSpace(in.ConfirmationToken),
			ParamsCanonical:   "purchaseOrderId=" + poID.String() + "\ntrackingNo=" + tn + "\ncarrier=" + carrier,
			ParamsSummary:     "purchaseOrderId=" + poID.String() + " trackingNo=" + tn,
			DryRun: func(ctx context.Context, db *gorm.DB) (any, string, error) {
				po, err := d.Procurement.FindPOInTenant(ctx, db, tok.TenantID, poID)
				if err != nil {
					return nil, "", err
				}
				if !procurement.CanTransition(po.Status, procurement.StatusShipped) {
					return nil, "", fmt.Errorf("采购单当前状态 %s 不允许回填运单号（需要 %s）", po.Status, procurement.StatusPaid)
				}
				preview := ProcurementWritePreview{
					PurchaseOrderID: po.ID.String(),
					SupplierName:    po.SupplierName,
					CurrentStatus:   po.Status,
					TargetStatus:    procurement.StatusShipped,
					TotalAmount:     po.TotalAmount,
					TrackingNo:      tn,
				}
				summary := fmt.Sprintf("将为采购单 %s（%s）回填运单号 %s 并标记为已发货", po.ID, po.SupplierName, tn)
				return preview, summary, nil
			},
			Execute: func(ctx context.Context, tx *gorm.DB) (any, string, error) {
				po, err := d.Procurement.FillLogisticsInTenantTx(ctx, tx, tok.TenantID, poID, tn, carrier)
				if err != nil {
					return nil, "", err
				}
				executed = po
				res := ProcurementWriteResult{PurchaseOrderID: po.ID.String(), Status: po.Status, TrackingNo: tn}
				return res, fmt.Sprintf("status=%s trackingNo=%s", po.Status, tn), nil
			},
		}
		out, err := d.writes().Run(ctx, req)
		if err != nil {
			return nil, nil, err
		}
		if executed != nil {
			d.Procurement.AfterFillLogisticsCommitted(ctx, executed, tn, carrier)
		}
		return nil, out, nil
	}
}
