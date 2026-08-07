package order

import (
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/trademind-ai/trademind/backend/internal/modules/waybill"
	"github.com/trademind-ai/trademind/backend/internal/pkg/adminperm"
	"github.com/trademind-ai/trademind/backend/internal/pkg/repository"
	"github.com/trademind-ai/trademind/backend/internal/pkg/response"
	"gorm.io/gorm"
)

// MarkPrintedBody is POST /orders/print/mark.
type MarkPrintedBody struct {
	IDs []string `json:"ids"`
}

// MarkPrinted stamps waybill_printed_at on tenant + store visible orders. It
// is a bookkeeping flag only and never changes order / shipment status.
func (s *Service) MarkPrinted(c *gin.Context, ids []uuid.UUID, adminID *uuid.UUID) (int, error) {
	if s == nil || s.DB == nil {
		return 0, fmt.Errorf("order: no db")
	}
	if len(ids) == 0 {
		return 0, fmt.Errorf("ids required")
	}
	if len(ids) > maxPrintSheets {
		return 0, fmt.Errorf("too many orders (max %d)", maxPrintSheets)
	}
	tid, err := adminperm.TenantIDFromGin(c)
	if err != nil {
		return 0, err
	}
	now := time.Now().UTC()
	marked := 0
	for _, id := range ids {
		var o Order
		if err := repository.FindByID(c.Request.Context(), s.DB, &o, tid, id); err != nil {
			return 0, err
		}
		if o.ShopID != nil {
			if err := adminperm.EnsureStoreOperable(c, s.DB, o.ShopID); err != nil {
				return 0, err
			}
		}
		if err := s.DB.WithContext(c.Request.Context()).Model(&Order{}).
			Where("id = ?", o.ID).Update("waybill_printed_at", now).Error; err != nil {
			return 0, err
		}
		marked++
	}
	return marked, nil
}

// PostMarkPrinted POST /orders/print/mark — marks selected orders as printed
// (打单状态), independent from the shipping state machine.
func (h *Handler) PostMarkPrinted(c *gin.Context) {
	if h == nil || h.Svc == nil {
		response.Fail(c, http.StatusInternalServerError, response.CodeInternalError, "orders unavailable")
		return
	}
	var body MarkPrintedBody
	if err := c.ShouldBindJSON(&body); err != nil {
		response.Fail(c, http.StatusBadRequest, response.CodeBadRequest, "invalid json body")
		return
	}
	ids := make([]uuid.UUID, 0, len(body.IDs))
	for _, raw := range body.IDs {
		id, err := uuid.Parse(strings.TrimSpace(raw))
		if err != nil {
			response.Fail(c, http.StatusBadRequest, response.CodeBadRequest, "invalid ids")
			return
		}
		ids = append(ids, id)
	}
	marked, err := h.Svc.MarkPrinted(c, ids, adminUUID(c))
	if err != nil {
		if errors.Is(err, ErrNotFound) || errors.Is(err, gorm.ErrRecordNotFound) {
			response.Fail(c, http.StatusNotFound, response.CodeNotFound, "not found")
			return
		}
		if errors.Is(err, adminperm.ErrStoreNotOperable) {
			response.Fail(c, http.StatusForbidden, response.CodeStorePermissionDenied, "店铺无操作权限")
			return
		}
		response.Fail(c, http.StatusBadRequest, response.CodeBadRequest, err.Error())
		return
	}
	response.OK(c, gin.H{"marked": marked})
}

// ShippingRecommendationItem asks for one order's carrier recommendation.
// Province / weight are optional operator-supplied attributes the order does
// not carry; platform and amount come from the order itself.
type ShippingRecommendationItem struct {
	Key      string   `json:"key"`
	OrderID  string   `json:"orderId"`
	OrderNo  string   `json:"orderNo"`
	Province string   `json:"province"`
	WeightKg *float64 `json:"weightKg"`
}

// ShippingRecommendationsBody is POST /orders/shipping-recommendations.
type ShippingRecommendationsBody struct {
	Items []ShippingRecommendationItem `json:"items"`
}

// ShippingRecommendationResult carries one per-order recommendation.
type ShippingRecommendationResult struct {
	Key         string `json:"key"`
	OrderID     string `json:"orderId,omitempty"`
	Matched     bool   `json:"matched"`
	RuleID      string `json:"ruleId,omitempty"`
	RuleName    string `json:"ruleName,omitempty"`
	CarrierCode string `json:"carrierCode,omitempty"`
	CarrierName string `json:"carrierName,omitempty"`
	Message     string `json:"message,omitempty"`
}

// RecommendShipping evaluates the tenant's shipping rules per order. Failed
// lookups are reported per item; the batch never fails as a whole.
func (s *Service) RecommendShipping(c *gin.Context, body ShippingRecommendationsBody) ([]ShippingRecommendationResult, error) {
	if s == nil || s.DB == nil {
		return nil, fmt.Errorf("order: no db")
	}
	if s.Waybill == nil {
		return nil, fmt.Errorf("shipping rules unavailable")
	}
	if len(body.Items) == 0 {
		return nil, fmt.Errorf("items required")
	}
	if len(body.Items) > batchShipmentsMaxItems {
		return nil, fmt.Errorf("too many items (max %d)", batchShipmentsMaxItems)
	}
	tid, err := adminperm.TenantIDFromGin(c)
	if err != nil {
		return nil, err
	}
	out := make([]ShippingRecommendationResult, 0, len(body.Items))
	for _, it := range body.Items {
		res := ShippingRecommendationResult{Key: strings.TrimSpace(it.Key)}
		o, msg := s.lookupRecommendationOrder(c, tid, it)
		if msg != "" {
			res.Message = msg
			out = append(out, res)
			continue
		}
		if res.Key == "" {
			res.Key = o.OrderNo
		}
		res.OrderID = o.ID.String()
		amount := o.TotalAmount
		rec, err := s.Waybill.Recommend(c, waybill.MatchAttrs{
			Province: strings.TrimSpace(it.Province),
			Platform: o.Platform,
			WeightKg: it.WeightKg,
			Amount:   &amount,
		})
		if err != nil {
			return nil, err
		}
		res.Matched = rec.Matched
		res.RuleID = rec.RuleID
		res.RuleName = rec.RuleName
		res.CarrierCode = rec.CarrierCode
		res.CarrierName = rec.CarrierName
		if !rec.Matched {
			res.Message = "没有命中任何发货规则，可手动选择物流商"
		}
		out = append(out, res)
	}
	return out, nil
}

func (s *Service) lookupRecommendationOrder(c *gin.Context, tid int64, it ShippingRecommendationItem) (*Order, string) {
	if raw := strings.TrimSpace(it.OrderID); raw != "" {
		id, err := uuid.Parse(raw)
		if err != nil {
			return nil, "订单 ID 无效"
		}
		var o Order
		if err := repository.FindByID(c.Request.Context(), s.DB, &o, tid, id); err != nil {
			return nil, "没有找到该订单"
		}
		if err := adminperm.EnsureStoreVisible(c, s.DB, o.ShopID); err != nil {
			return nil, "没有找到该订单"
		}
		return &o, ""
	}
	no := strings.TrimSpace(it.OrderNo)
	if no == "" {
		return nil, "缺少订单 ID 或订单号"
	}
	tx, err := adminperm.ApplyStoreScope(c, s.DB, s.DB.WithContext(c.Request.Context()).Model(&Order{}), "shop_id")
	if err != nil {
		return nil, "查询订单失败"
	}
	var orders []Order
	if err := tx.Where("tenant_id = ? AND order_no = ?", tid, no).Limit(2).Find(&orders).Error; err != nil {
		return nil, "查询订单失败"
	}
	switch len(orders) {
	case 0:
		return nil, "没有找到该订单号对应的销售订单"
	case 1:
		return &orders[0], ""
	default:
		return nil, "该订单号匹配到多个销售订单"
	}
}

// PostShippingRecommendations POST /orders/shipping-recommendations.
func (h *Handler) PostShippingRecommendations(c *gin.Context) {
	if h == nil || h.Svc == nil {
		response.Fail(c, http.StatusInternalServerError, response.CodeInternalError, "orders unavailable")
		return
	}
	var body ShippingRecommendationsBody
	if err := c.ShouldBindJSON(&body); err != nil {
		response.Fail(c, http.StatusBadRequest, response.CodeBadRequest, "invalid json body")
		return
	}
	items, err := h.Svc.RecommendShipping(c, body)
	if err != nil {
		response.Fail(c, http.StatusBadRequest, response.CodeBadRequest, err.Error())
		return
	}
	response.OK(c, gin.H{"items": items})
}
