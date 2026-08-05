package order

import (
	"fmt"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/trademind-ai/trademind/backend/internal/pkg/adminperm"
	"gorm.io/gorm"
)

// ErrOrderReviewBlocked is returned when a待审/挂起 order enters a blocked flow.
var ErrOrderReviewBlocked = fmt.Errorf("订单待审核或已挂起，放行后才能继续操作")

// ReviewWorkbenchQuery GET /order-review
type ReviewWorkbenchQuery struct {
	Page         int
	PageSize     int
	ReviewStatus string // pending_review | held | approved | rejected | auto_passed，空=待处理（pending_review+held）
	Keyword      string
}

// ReviewOrderRow is one审单工作台 row.
type ReviewOrderRow struct {
	ID           uuid.UUID        `json:"id"`
	OrderNo      string           `json:"orderNo"`
	Platform     string           `json:"platform"`
	ShopID       *uuid.UUID       `json:"shopId,omitempty"`
	ShopName     string           `json:"shopName,omitempty"`
	CustomerName string           `json:"customerName"`
	Status       string           `json:"status"`
	ReviewStatus string           `json:"reviewStatus"`
	Currency     string           `json:"currency"`
	TotalAmount  float64          `json:"totalAmount"`
	ItemCount    int              `json:"itemCount"`
	CreatedAt    time.Time        `json:"createdAt"`
	Hits         []OrderReviewHit `json:"hits"`
}

// ReviewWorkbenchResult pagination bundle.
type ReviewWorkbenchResult struct {
	Items      []ReviewOrderRow `json:"items"`
	Total      int64            `json:"total"`
	Page       int              `json:"page"`
	PageSize   int              `json:"pageSize"`
	TotalPages int              `json:"totalPages"`
	// PendingTotal counts orders still waiting (pending_review + held).
	PendingTotal int64 `json:"pendingTotal"`
}

func validWorkbenchReviewStatus(v string) bool {
	switch v {
	case ReviewStatusPending, ReviewStatusHeld, ReviewStatusApproved,
		ReviewStatusRejected, ReviewStatusAutoPassed:
		return true
	}
	return false
}

// ListReviewWorkbench pages orders in the review flow with their rule hits.
func (s *Service) ListReviewWorkbench(c *gin.Context, q ReviewWorkbenchQuery) (*ReviewWorkbenchResult, error) {
	if s == nil || s.DB == nil {
		return nil, fmt.Errorf("order: no db")
	}
	page := q.Page
	if page < 1 {
		page = 1
	}
	ps := q.PageSize
	if ps < 1 {
		ps = 20
	}
	if ps > 100 {
		ps = 100
	}
	ctx := c.Request.Context()
	tx := s.DB.WithContext(ctx).Model(&Order{})
	if st := strings.TrimSpace(q.ReviewStatus); st != "" {
		if !validWorkbenchReviewStatus(st) {
			return nil, fmt.Errorf("无效的审核状态：%s", st)
		}
		tx = tx.Where("review_status = ?", st)
	} else {
		tx = tx.Where("review_status IN ?", []string{ReviewStatusPending, ReviewStatusHeld})
	}
	if kw := strings.TrimSpace(q.Keyword); kw != "" {
		like := "%" + kw + "%"
		tx = tx.Where("order_no ILIKE ? OR customer_name ILIKE ?", like, like)
	}
	scoped, tid, err := adminperm.ApplyTenantScope(c, tx)
	if err != nil {
		return nil, err
	}
	tx = scoped
	if scoped, err := adminperm.ApplyStoreScope(c, s.DB, tx, "shop_id"); err != nil {
		return nil, err
	} else {
		tx = scoped
	}
	var total int64
	if err := tx.Count(&total).Error; err != nil {
		return nil, err
	}
	var rows []Order
	if err := tx.Order("created_at DESC, id DESC").
		Offset((page - 1) * ps).Limit(ps).Find(&rows).Error; err != nil {
		return nil, err
	}

	pendingTx := s.DB.WithContext(ctx).Model(&Order{}).
		Where("tenant_id = ? AND review_status IN ?", tid,
			[]string{ReviewStatusPending, ReviewStatusHeld})
	var pendingTotal int64
	if err := pendingTx.Count(&pendingTotal).Error; err != nil {
		return nil, err
	}

	res := &ReviewWorkbenchResult{
		Items: []ReviewOrderRow{}, Total: total, Page: page, PageSize: ps,
		TotalPages: pagesOf(total, ps), PendingTotal: pendingTotal,
	}
	if len(rows) == 0 {
		return res, nil
	}
	ids := make([]uuid.UUID, len(rows))
	shopIDs := make([]uuid.UUID, 0)
	for i, r := range rows {
		ids[i] = r.ID
		if r.ShopID != nil {
			shopIDs = append(shopIDs, *r.ShopID)
		}
	}
	var hits []OrderReviewHit
	if err := s.DB.WithContext(ctx).Where("order_id IN ?", ids).
		Order("decisive DESC, created_at ASC").Find(&hits).Error; err != nil {
		return nil, err
	}
	hitsBy := map[uuid.UUID][]OrderReviewHit{}
	for _, h := range hits {
		hitsBy[h.OrderID] = append(hitsBy[h.OrderID], h)
	}
	var itemCounts []struct {
		OrderID uuid.UUID
		Cnt     int
	}
	_ = s.DB.WithContext(ctx).Model(&OrderItem{}).
		Select("order_id, COUNT(*) as cnt").Where("order_id IN ?", ids).
		Group("order_id").Scan(&itemCounts).Error
	cntBy := map[uuid.UUID]int{}
	for _, ic := range itemCounts {
		cntBy[ic.OrderID] = ic.Cnt
	}
	shopNames := map[uuid.UUID]string{}
	if len(shopIDs) > 0 && s.Shops != nil {
		if sm, err := s.Shops.BatchSummaries(c, shopIDs); err == nil {
			for id, ssum := range sm {
				shopNames[id] = ssum.ShopName
			}
		}
	}
	for _, r := range rows {
		row := ReviewOrderRow{
			ID: r.ID, OrderNo: r.OrderNo, Platform: r.Platform, ShopID: r.ShopID,
			CustomerName: r.CustomerName, Status: r.Status, ReviewStatus: r.ReviewStatus,
			Currency: r.Currency, TotalAmount: r.TotalAmount,
			ItemCount: cntBy[r.ID], CreatedAt: r.CreatedAt,
			Hits: hitsBy[r.ID],
		}
		if row.Hits == nil {
			row.Hits = []OrderReviewHit{}
		}
		if r.ShopID != nil {
			row.ShopName = shopNames[*r.ShopID]
		}
		res.Items = append(res.Items, row)
	}
	return res, nil
}

// ReviewDecisionBody POST /order-review/approve | /order-review/reject.
type ReviewDecisionBody struct {
	OrderIDs []string `json:"orderIds"`
	Remark   string   `json:"remark,omitempty"`
}

// ReviewDecisionRowResult reports one order's outcome in a batch decision.
type ReviewDecisionRowResult struct {
	OrderID string `json:"orderId"`
	OrderNo string `json:"orderNo,omitempty"`
	OK      bool   `json:"ok"`
	Error   string `json:"error,omitempty"`
}

// ReviewDecisionResult aggregates a batch approve / reject run.
type ReviewDecisionResult struct {
	Total   int                       `json:"total"`
	Done    int                       `json:"done"`
	Failed  int                       `json:"failed"`
	Results []ReviewDecisionRowResult `json:"results"`
}

const maxReviewBatch = 100

func parseReviewOrderIDs(body ReviewDecisionBody) ([]uuid.UUID, error) {
	if len(body.OrderIDs) == 0 {
		return nil, fmt.Errorf("orderIds is required")
	}
	if len(body.OrderIDs) > maxReviewBatch {
		return nil, fmt.Errorf("单次最多处理 %d 个订单", maxReviewBatch)
	}
	out := make([]uuid.UUID, 0, len(body.OrderIDs))
	for _, raw := range body.OrderIDs {
		u, err := uuid.Parse(strings.TrimSpace(raw))
		if err != nil {
			return nil, fmt.Errorf("无效的订单 ID：%s", raw)
		}
		out = append(out, u)
	}
	return out, nil
}

// decideReview applies approve / reject to a batch of待审/挂起 orders.
// approve 放行回正常流；reject 拒绝并进入取消动线（订单状态置为 cancelled）。
func (s *Service) decideReview(c *gin.Context, body ReviewDecisionBody, adminID *uuid.UUID, approve bool) (*ReviewDecisionResult, error) {
	if s == nil || s.DB == nil {
		return nil, fmt.Errorf("order: no db")
	}
	ids, err := parseReviewOrderIDs(body)
	if err != nil {
		return nil, err
	}
	tid, err := adminperm.TenantIDFromGin(c)
	if err != nil {
		return nil, err
	}
	res := &ReviewDecisionResult{Total: len(ids)}
	action := "order_review.approve"
	if !approve {
		action = "order_review.reject"
	}
	for _, oid := range ids {
		row := ReviewDecisionRowResult{OrderID: oid.String()}
		err := s.DB.WithContext(c.Request.Context()).Transaction(func(tx *gorm.DB) error {
			var o Order
			if err := tx.First(&o, "id = ? AND tenant_id = ? AND deleted_at IS NULL", oid, tid).Error; err != nil {
				return fmt.Errorf("订单不存在")
			}
			// Store scope must gate the decision itself: the workbench list is
			// scoped, but ids arrive from the client and an operator may only
			// approve / reject orders of the stores granted to the account.
			if err := adminperm.EnsureStoreVisible(c, s.DB, o.ShopID); err != nil {
				return fmt.Errorf("订单不存在")
			}
			row.OrderNo = o.OrderNo
			if !reviewBlocked(o.ReviewStatus) {
				return fmt.Errorf("订单不在待审核或挂起状态")
			}
			updates := map[string]any{"review_status": ReviewStatusApproved}
			if !approve {
				updates["review_status"] = ReviewStatusRejected
				updates["status"] = StatusCancelled
			}
			return tx.Model(&Order{}).Where("id = ?", o.ID).Updates(updates).Error
		})
		if err != nil {
			row.Error = err.Error()
			res.Failed++
		} else {
			row.OK = true
			res.Done++
			if approve {
				// 放行后订单回到正常流：已付款订单补触发 order_paid 自动化
				// （待审/挂起期间安全边界拦截过的规则此时才允许执行）。
				var released Order
				if e := s.DB.WithContext(c.Request.Context()).
					First(&released, "id = ? AND tenant_id = ?", oid, tid).Error; e == nil &&
					released.PaymentStatus == PaymentPaid {
					s.FireOrderEvent(c.Request.Context(), tid, oid, AutomationEventOrderPaid)
				}
			}
		}
		res.Results = append(res.Results, row)
	}
	s.logReview(c, adminID, action, "",
		fmt.Sprintf("total=%d done=%d failed=%d", res.Total, res.Done, res.Failed))
	return res, nil
}

// ApproveReviewOrders 批量放行.
func (s *Service) ApproveReviewOrders(c *gin.Context, body ReviewDecisionBody, adminID *uuid.UUID) (*ReviewDecisionResult, error) {
	return s.decideReview(c, body, adminID, true)
}

// RejectReviewOrders 批量拒绝（订单进入取消动线）.
func (s *Service) RejectReviewOrders(c *gin.Context, body ReviewDecisionBody, adminID *uuid.UUID) (*ReviewDecisionResult, error) {
	return s.decideReview(c, body, adminID, false)
}

// guardReviewNotBlocked returns ErrOrderReviewBlocked when the order is待审/挂起.
func guardReviewNotBlocked(o *Order) error {
	if o != nil && reviewBlocked(o.ReviewStatus) {
		if o.ReviewStatus == ReviewStatusHeld {
			return fmt.Errorf("订单已被审单规则挂起，不能发货；请先在审单工作台放行")
		}
		return fmt.Errorf("订单待人工审核，不能发货；请先在审单工作台放行")
	}
	return nil
}
