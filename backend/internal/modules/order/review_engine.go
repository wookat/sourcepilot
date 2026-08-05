package order

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// reviewEvalInput bundles the order attributes the rule engine evaluates.
type reviewEvalInput struct {
	OrderID       uuid.UUID
	TenantID      int64
	Platform      string
	ShopID        *uuid.UUID
	TotalAmount   float64
	Remark        string
	AddressText   string
	CustomerName  string
	CustomerPhone string
	TotalQty      int
	MaxLineQty    int
	CreatedAt     time.Time
}

// reviewRuleHit is one matched rule with a human-readable reason.
type reviewRuleHit struct {
	Rule   OrderReviewRule
	Reason string
}

// reviewEvalResult is the engine outcome for one order.
type reviewEvalResult struct {
	// Status is the resulting review status ("" when no rule matched).
	Status string
	Hits   []reviewRuleHit
}

func jsonStringList(raw json.RawMessage) []string {
	if len(raw) == 0 {
		return nil
	}
	var out []string
	if err := json.Unmarshal(raw, &out); err != nil {
		return nil
	}
	res := make([]string, 0, len(out))
	for _, v := range out {
		if t := strings.TrimSpace(v); t != "" {
			res = append(res, t)
		}
	}
	return res
}

// extractAddressText pulls a best-effort收货地址 text from the order RawData
// JSON (keys like address / receiverAddress / province / city, including one
// nested receiver/address object). Unknown address = address rules never match.
func extractAddressText(raw json.RawMessage) string {
	if len(raw) == 0 {
		return ""
	}
	var m map[string]any
	if err := json.Unmarshal(raw, &m); err != nil {
		return ""
	}
	keys := []string{"address", "receiverAddress", "shippingAddress", "province", "city", "district", "detailAddress"}
	var parts []string
	collect := func(obj map[string]any) {
		for _, k := range keys {
			if v, ok := obj[k].(string); ok {
				if t := strings.TrimSpace(v); t != "" {
					parts = append(parts, t)
				}
			}
		}
	}
	collect(m)
	for _, nested := range []string{"receiver", "address", "shippingAddress"} {
		if obj, ok := m[nested].(map[string]any); ok {
			collect(obj)
		}
	}
	return strings.Join(parts, " ")
}

func containsAnyKeyword(text string, keywords []string) (string, bool) {
	t := strings.ToLower(strings.TrimSpace(text))
	if t == "" {
		return "", false
	}
	for _, kw := range keywords {
		if kw == "" {
			continue
		}
		if strings.Contains(t, strings.ToLower(kw)) {
			return kw, true
		}
	}
	return "", false
}

func listContainsFold(list []string, v string) bool {
	v = strings.TrimSpace(v)
	for _, item := range list {
		if strings.EqualFold(strings.TrimSpace(item), v) {
			return true
		}
	}
	return false
}

// countRecentReceiverOrders counts non-cancelled orders of the same tenant
// with the same收件人 (name, plus phone when present) within the window,
// including the order being evaluated when already persisted.
func countRecentReceiverOrders(tx *gorm.DB, in reviewEvalInput, windowDays int) (int64, error) {
	if strings.TrimSpace(in.CustomerName) == "" {
		return 0, nil
	}
	if windowDays <= 0 {
		windowDays = 7
	}
	since := in.CreatedAt.Add(-time.Duration(windowDays) * 24 * time.Hour)
	q := tx.Model(&Order{}).
		Where("tenant_id = ? AND customer_name = ? AND status <> ? AND created_at >= ? AND deleted_at IS NULL",
			in.TenantID, strings.TrimSpace(in.CustomerName), StatusCancelled, since)
	if p := strings.TrimSpace(in.CustomerPhone); p != "" {
		q = q.Where("customer_phone = ?", p)
	}
	var cnt int64
	if err := q.Count(&cnt).Error; err != nil {
		return 0, err
	}
	return cnt, nil
}

// matchReviewRule evaluates one rule against the input (AND semantics; a rule
// with no conditions never matches to avoid an accidental catch-all blocking
// every order — at least one condition must be set and satisfied).
func matchReviewRule(tx *gorm.DB, r OrderReviewRule, in reviewEvalInput) (string, bool, error) {
	if !r.Enabled {
		return "", false, nil
	}
	var reasons []string

	conditioned := false
	if platforms := jsonStringList(json.RawMessage(r.Platforms)); len(platforms) > 0 {
		conditioned = true
		if !listContainsFold(platforms, in.Platform) {
			return "", false, nil
		}
		reasons = append(reasons, fmt.Sprintf("平台命中（%s）", in.Platform))
	}
	if shopIDs := jsonStringList(json.RawMessage(r.ShopIDs)); len(shopIDs) > 0 {
		conditioned = true
		if in.ShopID == nil || !listContainsFold(shopIDs, in.ShopID.String()) {
			return "", false, nil
		}
		reasons = append(reasons, "店铺命中")
	}
	if r.MinAmount != nil || r.MaxAmount != nil {
		conditioned = true
		if r.MinAmount != nil && in.TotalAmount < *r.MinAmount {
			return "", false, nil
		}
		if r.MaxAmount != nil && in.TotalAmount > *r.MaxAmount {
			return "", false, nil
		}
		reasons = append(reasons, fmt.Sprintf("订单金额 %.2f 落入阈值区间", in.TotalAmount))
	}
	if kws := jsonStringList(json.RawMessage(r.AddressKeywords)); len(kws) > 0 {
		conditioned = true
		kw, ok := containsAnyKeyword(in.AddressText, kws)
		if !ok {
			return "", false, nil
		}
		reasons = append(reasons, fmt.Sprintf("收货地址含关键词「%s」", kw))
	}
	if kws := jsonStringList(json.RawMessage(r.RemarkKeywords)); len(kws) > 0 {
		conditioned = true
		kw, ok := containsAnyKeyword(in.Remark, kws)
		if !ok {
			return "", false, nil
		}
		reasons = append(reasons, fmt.Sprintf("买家备注含关键词「%s」", kw))
	}
	if r.MaxTotalQuantity != nil {
		conditioned = true
		if in.TotalQty <= *r.MaxTotalQuantity {
			return "", false, nil
		}
		reasons = append(reasons, fmt.Sprintf("商品总数量 %d 超过 %d", in.TotalQty, *r.MaxTotalQuantity))
	}
	if r.MaxSKUQuantity != nil {
		conditioned = true
		if in.MaxLineQty <= *r.MaxSKUQuantity {
			return "", false, nil
		}
		reasons = append(reasons, fmt.Sprintf("单 SKU 数量 %d 超过 %d", in.MaxLineQty, *r.MaxSKUQuantity))
	}
	if r.RepeatReceiverMinOrders != nil && *r.RepeatReceiverMinOrders > 0 {
		conditioned = true
		windowDays := 7
		if r.RepeatReceiverWindowDays != nil && *r.RepeatReceiverWindowDays > 0 {
			windowDays = *r.RepeatReceiverWindowDays
		}
		cnt, err := countRecentReceiverOrders(tx, in, windowDays)
		if err != nil {
			return "", false, err
		}
		if cnt < int64(*r.RepeatReceiverMinOrders) {
			return "", false, nil
		}
		reasons = append(reasons, fmt.Sprintf("同收件人 %d 天内已有 %d 单", windowDays, cnt))
	}
	if !conditioned {
		return "", false, nil
	}
	return strings.Join(reasons, "；"), true, nil
}

// reviewStatusForAction maps a rule action to the resulting review status.
func reviewStatusForAction(action string) string {
	switch action {
	case ReviewActionPass:
		return ReviewStatusAutoPassed
	case ReviewActionReview:
		return ReviewStatusPending
	case ReviewActionHold:
		return ReviewStatusHeld
	default:
		return ReviewStatusNone
	}
}

// evaluateReviewRules runs all enabled tenant rules against the input by
// ascending priority. The first match decides the final status; all matches
// are returned as hits so operators can see every triggered rule.
func evaluateReviewRules(tx *gorm.DB, tenantID int64, in reviewEvalInput) (*reviewEvalResult, error) {
	var rules []OrderReviewRule
	if err := tx.Where("tenant_id = ? AND enabled = TRUE", tenantID).
		Order("priority ASC, created_at ASC").Find(&rules).Error; err != nil {
		return nil, err
	}
	res := &reviewEvalResult{Status: ReviewStatusNone}
	for _, r := range rules {
		reason, ok, err := matchReviewRule(tx, r, in)
		if err != nil {
			return nil, err
		}
		if !ok {
			continue
		}
		res.Hits = append(res.Hits, reviewRuleHit{Rule: r, Reason: reason})
	}
	if len(res.Hits) > 0 {
		res.Status = reviewStatusForAction(res.Hits[0].Rule.Action)
	}
	return res, nil
}

// buildReviewInput shapes engine input from a persisted order and its items.
func buildReviewInput(o *Order, items []OrderItem) reviewEvalInput {
	in := reviewEvalInput{
		OrderID:       o.ID,
		TenantID:      o.TenantID,
		Platform:      o.Platform,
		ShopID:        o.ShopID,
		TotalAmount:   o.TotalAmount,
		Remark:        o.Remark,
		AddressText:   extractAddressText(json.RawMessage(o.RawData)),
		CustomerName:  o.CustomerName,
		CustomerPhone: o.CustomerPhone,
		CreatedAt:     o.CreatedAt,
	}
	if in.CreatedAt.IsZero() {
		in.CreatedAt = time.Now().UTC()
	}
	for _, it := range items {
		in.TotalQty += it.Quantity
		if it.Quantity > in.MaxLineQty {
			in.MaxLineQty = it.Quantity
		}
	}
	return in
}

// runReviewOnCreate evaluates rules inside the create transaction and
// persists the review status plus hits. It never fails order creation on
// pure evaluation problems other than DB errors.
func runReviewOnCreate(tx *gorm.DB, o *Order, items []OrderItem) error {
	if o == nil {
		return nil
	}
	res, err := evaluateReviewRules(tx, o.TenantID, buildReviewInput(o, items))
	if err != nil {
		return err
	}
	if res.Status == ReviewStatusNone {
		return nil
	}
	for i, h := range res.Hits {
		hit := OrderReviewHit{
			TenantID: o.TenantID,
			OrderID:  o.ID,
			RuleID:   h.Rule.ID,
			RuleName: h.Rule.Name,
			Action:   h.Rule.Action,
			Reason:   h.Reason,
			Decisive: i == 0,
		}
		if err := tx.Create(&hit).Error; err != nil {
			return err
		}
	}
	if err := tx.Model(&Order{}).Where("id = ?", o.ID).
		Update("review_status", res.Status).Error; err != nil {
		return err
	}
	o.ReviewStatus = res.Status
	return nil
}
