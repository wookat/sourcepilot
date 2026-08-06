package customerchat

import (
	"context"
	"encoding/json"
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/trademind-ai/trademind/backend/internal/modules/order"
	"github.com/trademind-ai/trademind/backend/internal/modules/shop"
	"github.com/trademind-ai/trademind/backend/internal/pkg/adminperm"
	"gorm.io/datatypes"
	"gorm.io/gorm"
)

var buyerMsgVarRe = regexp.MustCompile(`\{([^{}]+)\}`)

// FillBuyerMsgTemplate replaces {变量} placeholders with order context values;
// missing variables keep the raw placeholder and are reported (与前端
// fillReplyTemplate 口径一致).
func FillBuyerMsgTemplate(content string, vars map[string]string) (string, []string) {
	var missing []string
	seen := map[string]bool{}
	text := buyerMsgVarRe.ReplaceAllStringFunc(content, func(raw string) string {
		name := strings.TrimSpace(raw[1 : len(raw)-1])
		if v, ok := vars[name]; ok && strings.TrimSpace(v) != "" {
			return v
		}
		if !seen[name] {
			seen[name] = true
			missing = append(missing, name)
		}
		return raw
	})
	return text, missing
}

// buyerMsgNodeCondition applies the SQL condition for one node on an orders query.
func buyerMsgNodeCondition(db *gorm.DB, node string) *gorm.DB {
	switch node {
	case BuyerMsgNodePaid:
		return db.Where("payment_status = ? OR status IN ?", order.PaymentPaid,
			[]string{order.StatusPaid, order.StatusProcessing, order.StatusShipped, order.StatusDelivered})
	case BuyerMsgNodeShipped:
		return db.Where("status IN ? OR shipped_at IS NOT NULL",
			[]string{order.StatusShipped, order.StatusDelivered})
	case BuyerMsgNodeDelivered:
		return db.Where("status = ? OR delivered_at IS NOT NULL", order.StatusDelivered)
	case BuyerMsgNodeLogisticsException:
		return db.Where("EXISTS (SELECT 1 FROM order_shipments sh WHERE sh.order_id = orders.id AND sh.status = ? AND sh.deleted_at IS NULL)",
			order.ShipmentException)
	case BuyerMsgNodeRefunded:
		return db.Where("status = ? OR payment_status IN ?", order.StatusRefunded,
			[]string{order.PaymentRefunded, order.PaymentPartiallyRefunded})
	default:
		return db.Where("1 = 0")
	}
}

// buyerMsgEffectiveCondition restricts an orders query to node events that
// happened at/after eff（规则生效时间）。事件时间取节点对应时间戳，缺失时
// 退回 orders.created_at（无事件时间戳的存量订单不回溯）。
func buyerMsgEffectiveCondition(db *gorm.DB, node string, eff time.Time) *gorm.DB {
	switch node {
	case BuyerMsgNodePaid:
		return db.Where("COALESCE(orders.paid_at, orders.created_at) >= ?", eff)
	case BuyerMsgNodeShipped:
		return db.Where("COALESCE(orders.shipped_at, orders.created_at) >= ?", eff)
	case BuyerMsgNodeDelivered:
		return db.Where("COALESCE(orders.delivered_at, orders.created_at) >= ?", eff)
	case BuyerMsgNodeLogisticsException:
		return db.Where("EXISTS (SELECT 1 FROM order_shipments se WHERE se.order_id = orders.id AND se.status = ? AND se.updated_at >= ?)",
			order.ShipmentException, eff)
	case BuyerMsgNodeRefunded:
		// 退款无独立时间戳：退款状态变更会刷新 updated_at，以此判定事件时间。
		return db.Where("orders.updated_at >= ?", eff)
	default:
		return db.Where("1 = 0")
	}
}

// buyerMsgOrdersQuery builds the shared orders query for draft generation and
// backfill estimation: node + platform/shop filters, minus orders that already
// have a draft for the node; effectiveFrom (when non-nil) excludes存量事件。
func (s *Service) buyerMsgOrdersQuery(ctx context.Context, tenantID int64, node string, platforms, shopIDs []string, effectiveFrom *time.Time) *gorm.DB {
	q := s.DB.WithContext(ctx).Model(&order.Order{}).
		Where("orders.tenant_id = ?", tenantID)
	q = buyerMsgNodeCondition(q, node)
	if effectiveFrom != nil {
		q = buyerMsgEffectiveCondition(q, node, *effectiveFrom)
	}
	if len(platforms) > 0 {
		q = q.Where("orders.platform IN ?", platforms)
	}
	if len(shopIDs) > 0 {
		q = q.Where("orders.shop_id IN ?", shopIDs)
	}
	return q.Where("NOT EXISTS (SELECT 1 FROM buyer_message_drafts d WHERE d.tenant_id = orders.tenant_id AND d.order_id = orders.id AND d.node = ?)", node)
}

// EstimateBuyerMsgBackfill counts存量订单 that would get a draft if a rule
// with the given node / filters ran with「回溯存量」开启 (no effective-from
// cutoff). Used by the admin UI to show the confirmation estimate.
func (s *Service) EstimateBuyerMsgBackfill(c *gin.Context, node string, platforms, shopIDs []string) (int64, error) {
	if s == nil || s.DB == nil {
		return 0, fmt.Errorf("customerchat: no db")
	}
	tid, err := adminperm.TenantIDFromGin(c)
	if err != nil {
		return 0, err
	}
	if !IsValidBuyerMsgNode(node) {
		return 0, fmt.Errorf("订单节点不合法，可选值：%s", strings.Join(BuyerMsgNodes, "/"))
	}
	var n int64
	if err := s.buyerMsgOrdersQuery(c.Request.Context(), tid, node, platforms, shopIDs, nil).
		Count(&n).Error; err != nil {
		return 0, err
	}
	return n, nil
}

type buyerMsgOrderCtx struct {
	trackingNo   string
	productTitle string
	shopName     string
}

func (s *Service) buyerMsgOrderContext(ctx context.Context, o *order.Order) buyerMsgOrderCtx {
	out := buyerMsgOrderCtx{}
	var ship order.OrderShipment
	if err := s.DB.WithContext(ctx).Where("order_id = ?", o.ID).
		Order("created_at DESC").First(&ship).Error; err == nil {
		out.trackingNo = ship.TrackingNo
	}
	var item order.OrderItem
	if err := s.DB.WithContext(ctx).Where("order_id = ?", o.ID).
		Order("created_at ASC").First(&item).Error; err == nil {
		out.productTitle = item.ProductTitle
	}
	if o.ShopID != nil {
		var sh shop.Shop
		if err := s.DB.WithContext(ctx).Where("id = ?", *o.ShopID).First(&sh).Error; err == nil {
			out.shopName = sh.ShopName
		}
	}
	return out
}

// GenerateBuyerMsgDrafts scans tenant orders against enabled node rules and
// creates missing pending drafts. Idempotent: at most one draft per
// (tenant, order, node); existing drafts (any status) are never overwritten.
// Drafts are never sent anywhere by the system.
func (s *Service) GenerateBuyerMsgDrafts(ctx context.Context, tenantID int64) (int, error) {
	if s == nil || s.DB == nil {
		return 0, fmt.Errorf("customerchat: no db")
	}
	var rules []BuyerMessageRule
	if err := s.DB.WithContext(ctx).
		Where("tenant_id = ? AND enabled = ?", tenantID, true).
		Order("created_at ASC").Find(&rules).Error; err != nil {
		return 0, err
	}
	if len(rules) == 0 {
		return 0, nil
	}
	templateIDs := make([]uuid.UUID, 0, len(rules))
	for _, r := range rules {
		templateIDs = append(templateIDs, r.TemplateID)
	}
	var templates []CustomerReplyTemplate
	if err := s.DB.WithContext(ctx).
		Where("tenant_id = ? AND id IN ? AND enabled = ?", tenantID, templateIDs, true).
		Find(&templates).Error; err != nil {
		return 0, err
	}
	tplByID := map[uuid.UUID]CustomerReplyTemplate{}
	for _, t := range templates {
		tplByID[t.ID] = t
	}

	created := 0
	for _, rule := range rules {
		tpl, ok := tplByID[rule.TemplateID]
		if !ok {
			continue // template deleted / disabled: rule is inert, not an error
		}
		n, err := s.generateForRule(ctx, rule, tpl)
		if err != nil {
			return created, err
		}
		created += n
	}
	return created, nil
}

const buyerMsgScanBatch = 200

func (s *Service) generateForRule(ctx context.Context, rule BuyerMessageRule, tpl CustomerReplyTemplate) (int, error) {
	q := s.buyerMsgOrdersQuery(ctx, rule.TenantID, rule.Node,
		jsonToStrings(rule.Platforms), jsonToStrings(rule.ShopIDs), rule.EffectiveFrom)

	var orders []order.Order
	if err := q.Order("orders.created_at ASC").Limit(buyerMsgScanBatch).Find(&orders).Error; err != nil {
		return 0, err
	}
	created := 0
	for i := range orders {
		o := &orders[i]
		octx := s.buyerMsgOrderContext(ctx, o)
		vars := map[string]string{
			"买家昵称": o.CustomerName,
			"订单号":  o.OrderNo,
			"物流单号": octx.trackingNo,
			"商品名":  octx.productTitle,
			"店铺名":  octx.shopName,
		}
		content, missing := FillBuyerMsgTemplate(tpl.Content, vars)
		var missingJSON datatypes.JSON
		if len(missing) > 0 {
			if b, err := json.Marshal(missing); err == nil {
				missingJSON = datatypes.JSON(b)
			}
		}
		draft := BuyerMessageDraft{
			TenantID:       rule.TenantID,
			OrderID:        o.ID,
			Node:           rule.Node,
			RuleID:         rule.ID,
			TemplateID:     tpl.ID,
			TemplateName:   tpl.Name,
			Platform:       o.Platform,
			ShopID:         o.ShopID,
			OrderNo:        o.OrderNo,
			CustomerName:   o.CustomerName,
			Content:        content,
			MissingVars:    missingJSON,
			Status:         BuyerMsgDraftPending,
			ConversationID: s.buyerMsgConversationID(ctx, rule.TenantID, o.ID),
		}
		if err := s.DB.WithContext(ctx).Create(&draft).Error; err != nil {
			// Unique-index race with a concurrent scan: skip, keep going.
			continue
		}
		created++
	}
	return created, nil
}

func (s *Service) buyerMsgConversationID(ctx context.Context, tenantID int64, orderID uuid.UUID) *uuid.UUID {
	var conv CustomerConversation
	if err := s.DB.WithContext(ctx).
		Where("tenant_id = ? AND order_id = ?", tenantID, orderID).
		Order("created_at DESC").First(&conv).Error; err != nil {
		return nil
	}
	id := conv.ID
	return &id
}

// BuyerMsgTenantIDs returns distinct tenant ids that have enabled rules
// (used by the periodic scanner).
func (s *Service) BuyerMsgTenantIDs(ctx context.Context) ([]int64, error) {
	if s == nil || s.DB == nil {
		return nil, fmt.Errorf("customerchat: no db")
	}
	var ids []int64
	if err := s.DB.WithContext(ctx).Model(&BuyerMessageRule{}).
		Where("enabled = ?", true).
		Distinct("tenant_id").Pluck("tenant_id", &ids).Error; err != nil {
		return nil, err
	}
	return ids, nil
}
