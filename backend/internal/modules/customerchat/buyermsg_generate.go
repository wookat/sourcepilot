package customerchat

import (
	"context"
	"encoding/json"
	"fmt"
	"regexp"
	"strings"

	"github.com/google/uuid"
	"github.com/trademind-ai/trademind/backend/internal/modules/order"
	"github.com/trademind-ai/trademind/backend/internal/modules/shop"
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
	q := s.DB.WithContext(ctx).Model(&order.Order{}).
		Where("orders.tenant_id = ?", rule.TenantID)
	q = buyerMsgNodeCondition(q, rule.Node)
	if platforms := jsonToStrings(rule.Platforms); len(platforms) > 0 {
		q = q.Where("orders.platform IN ?", platforms)
	}
	if shopIDs := jsonToStrings(rule.ShopIDs); len(shopIDs) > 0 {
		q = q.Where("orders.shop_id IN ?", shopIDs)
	}
	q = q.Where("NOT EXISTS (SELECT 1 FROM buyer_message_drafts d WHERE d.tenant_id = orders.tenant_id AND d.order_id = orders.id AND d.node = ?)", rule.Node)

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
