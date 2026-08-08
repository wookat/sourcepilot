package mcpserver

import (
	"context"
	"fmt"
	"strings"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/trademind-ai/trademind/backend/internal/modules/mcptoken"
	"github.com/trademind-ai/trademind/backend/internal/modules/mcpwrite"
	"github.com/trademind-ai/trademind/backend/internal/modules/order"
	"gorm.io/gorm"
)

// Whitelisted write tool names. Anything not in this set is unreachable as a
// write; message sending and external-platform actions are permanently
// excluded from MCP (绝不自动外发).
const (
	ToolOrdersAddTag             = "orders_add_tag"
	ToolOrdersRemoveTag          = "orders_remove_tag"
	ToolExceptionsMark           = "exceptions_mark"
	ToolProcurementMarkPlaced    = "procurement_mark_placed"
	ToolProcurementFillLogistics = "procurement_fill_logistics"
)

// isWriteTool reports whether a tool audits inside the write pipeline.
func isWriteTool(name string) bool {
	switch name {
	case ToolOrdersAddTag, ToolOrdersRemoveTag,
		ToolExceptionsMark, ToolProcurementMarkPlaced, ToolProcurementFillLogistics,
		ToolProcurementMarkPaid:
		return true
	}
	return false
}

// OrderTagWriteIn is the input of orders_add_tag / orders_remove_tag: one
// order (business order no), one existing tag (name), the mandatory mode and
// the confirmation token for execute. One target object per call.
type OrderTagWriteIn struct {
	OrderNo string `json:"orderNo" jsonschema:"订单号（业务单号，必填）"`
	TagName string `json:"tagName" jsonschema:"标签名称（必须是已存在的租户标签，必填）"`
	// Mode must be dry_run first; execute requires the confirmation token
	// returned by the dry_run.
	Mode              string `json:"mode" jsonschema:"dry_run（预览并领取确认 token）或 execute（携确认 token 执行）"`
	ConfirmationToken string `json:"confirmationToken,omitempty" jsonschema:"execute 时必填：dry_run 返回的一次性确认 token"`
}

// OrderTagWritePreview is the dry-run impact preview.
type OrderTagWritePreview struct {
	OrderNo     string   `json:"orderNo"`
	TagName     string   `json:"tagName"`
	CurrentTags []string `json:"currentTags"`
	// Change describes the effective mutation: add / remove / none (idempotent no-op).
	Change string `json:"change"`
}

// OrderTagWriteResult is the execute outcome.
type OrderTagWriteResult struct {
	OrderNo string `json:"orderNo"`
	TagName string `json:"tagName"`
	// Applied / Removed report whether a link was actually written / deleted
	// (0 = idempotent no-op).
	Applied int64    `json:"applied"`
	Removed int64    `json:"removed"`
	Tags    []string `json:"tags"`
}

// registerWriteTools mounts the whitelisted write tools for one write:ops
// token. Only called when the env gate is open and the token carries
// write:ops; the tenant gate and quotas are enforced per call.
func registerWriteTools(srv *mcp.Server, d *Deps, tok *mcptoken.Token) {
	registerWriteToolsR180(srv, d, tok)
	registerWriteToolsR181(srv, d, tok)
	if d.Orders == nil {
		return
	}
	mcp.AddTool(srv, &mcp.Tool{
		Name:        ToolOrdersAddTag,
		Description: "为当前租户的一个订单添加一个已存在的标签（幂等；写白名单动作）。必须先以 mode=dry_run 调用获取影响预览与一次性确认 token，再以 mode=execute 携 token 执行。",
	}, orderTagWriteTool(d, tok, true))
	mcp.AddTool(srv, &mcp.Tool{
		Name:        ToolOrdersRemoveTag,
		Description: "移除当前租户一个订单上的一个标签（幂等；写白名单动作）。必须先以 mode=dry_run 调用获取影响预览与一次性确认 token，再以 mode=execute 携 token 执行。",
	}, orderTagWriteTool(d, tok, false))
}

func orderTagWriteTool(d *Deps, tok *mcptoken.Token, add bool) mcp.ToolHandlerFor[OrderTagWriteIn, *mcpwrite.Result] {
	toolName := ToolOrdersRemoveTag
	if add {
		toolName = ToolOrdersAddTag
	}
	return func(ctx context.Context, _ *mcp.CallToolRequest, in OrderTagWriteIn) (*mcp.CallToolResult, *mcpwrite.Result, error) {
		orderNo := strings.TrimSpace(in.OrderNo)
		tagName := strings.TrimSpace(in.TagName)
		if orderNo == "" || tagName == "" {
			return nil, nil, fmt.Errorf("orderNo 与 tagName 均为必填")
		}
		resolve := func(ctx context.Context, db *gorm.DB) (*order.Order, *order.OrderTag, error) {
			o, err := d.Orders.FindOrderByNoInTenant(ctx, db, tok.TenantID, orderNo)
			if err != nil {
				return nil, nil, err
			}
			tag, err := d.Orders.FindOrderTagByNameInTenant(ctx, db, tok.TenantID, tagName)
			if err != nil {
				return nil, nil, err
			}
			return o, tag, nil
		}
		req := mcpwrite.Request{
			Caller: mcpwrite.Caller{
				TenantID:    tok.TenantID,
				TokenID:     tok.ID,
				TokenName:   tok.Name,
				TokenMasked: tok.Masked(),
			},
			Tool:              toolName,
			Mode:              strings.TrimSpace(in.Mode),
			ConfirmationToken: strings.TrimSpace(in.ConfirmationToken),
			ParamsCanonical:   "orderNo=" + orderNo + "\ntagName=" + tagName,
			ParamsSummary:     "orderNo=" + orderNo + " tag=" + tagName,
			DryRun: func(ctx context.Context, db *gorm.DB) (any, string, error) {
				o, tag, err := resolve(ctx, db)
				if err != nil {
					return nil, "", err
				}
				current, err := d.Orders.OrderTagNamesInTenant(ctx, db, tok.TenantID, o.ID)
				if err != nil {
					return nil, "", err
				}
				has := false
				for _, n := range current {
					if n == tag.Name {
						has = true
						break
					}
				}
				preview := OrderTagWritePreview{OrderNo: o.OrderNo, TagName: tag.Name, CurrentTags: current, Change: "none"}
				var summary string
				switch {
				case add && !has:
					preview.Change = "add"
					summary = fmt.Sprintf("将为订单 %s 添加标签「%s」", o.OrderNo, tag.Name)
				case add && has:
					summary = fmt.Sprintf("订单 %s 已有标签「%s」，执行为幂等空操作", o.OrderNo, tag.Name)
				case !add && has:
					preview.Change = "remove"
					summary = fmt.Sprintf("将移除订单 %s 的标签「%s」", o.OrderNo, tag.Name)
				default:
					summary = fmt.Sprintf("订单 %s 没有标签「%s」，执行为幂等空操作", o.OrderNo, tag.Name)
				}
				return preview, summary, nil
			},
			Execute: func(ctx context.Context, tx *gorm.DB) (any, string, error) {
				o, tag, err := resolve(ctx, tx)
				if err != nil {
					return nil, "", err
				}
				res := OrderTagWriteResult{OrderNo: o.OrderNo, TagName: tag.Name}
				if add {
					res.Applied, err = d.Orders.AttachOrderTagInTenant(ctx, tx, tok.TenantID, o, tag)
				} else {
					res.Removed, err = d.Orders.DetachOrderTagInTenant(ctx, tx, tok.TenantID, o, tag)
				}
				if err != nil {
					return nil, "", err
				}
				if res.Tags, err = d.Orders.OrderTagNamesInTenant(ctx, tx, tok.TenantID, o.ID); err != nil {
					return nil, "", err
				}
				var summary string
				if add {
					summary = fmt.Sprintf("applied=%d tags=%s", res.Applied, strings.Join(res.Tags, ","))
				} else {
					summary = fmt.Sprintf("removed=%d tags=%s", res.Removed, strings.Join(res.Tags, ","))
				}
				return res, summary, nil
			},
		}
		out, err := d.writes().Run(ctx, req)
		if err != nil {
			return nil, nil, err
		}
		return nil, out, nil
	}
}
