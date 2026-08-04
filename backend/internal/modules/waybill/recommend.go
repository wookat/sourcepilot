package waybill

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/trademind-ai/trademind/backend/internal/pkg/adminperm"
)

// MatchAttrs are the known order attributes a rule is evaluated against.
// Nil / empty attributes are "unknown": a rule requiring them cannot match.
type MatchAttrs struct {
	Province string   `json:"province"`
	Platform string   `json:"platform"`
	WeightKg *float64 `json:"weightKg"`
	Amount   *float64 `json:"amount"`
}

// Recommendation is the outcome of evaluating rules for one attrs set.
type Recommendation struct {
	Matched     bool   `json:"matched"`
	RuleID      string `json:"ruleId,omitempty"`
	RuleName    string `json:"ruleName,omitempty"`
	CarrierCode string `json:"carrierCode,omitempty"`
	CarrierName string `json:"carrierName,omitempty"`
}

func jsonList(raw []byte) []string {
	if len(raw) == 0 {
		return nil
	}
	var out []string
	_ = json.Unmarshal(raw, &out)
	return out
}

func listMatches(list []string, value string) bool {
	if len(list) == 0 {
		return true // no condition
	}
	v := strings.TrimSpace(value)
	if v == "" {
		return false // condition set but attribute unknown
	}
	for _, item := range list {
		if strings.EqualFold(strings.TrimSpace(item), v) {
			return true
		}
		// Province names are often pasted with/without suffix (广东 vs 广东省).
		if strings.HasPrefix(v, strings.TrimSpace(item)) || strings.HasPrefix(strings.TrimSpace(item), v) {
			return true
		}
	}
	return false
}

func rangeMatches(minV, maxV *float64, value *float64) bool {
	if minV == nil && maxV == nil {
		return true
	}
	if value == nil {
		return false
	}
	if minV != nil && *value < *minV {
		return false
	}
	if maxV != nil && *value > *maxV {
		return false
	}
	return true
}

// RuleMatches reports whether one enabled rule matches the attrs.
func RuleMatches(r *ShippingRule, attrs MatchAttrs) bool {
	if r == nil || !r.Enabled {
		return false
	}
	if !listMatches(jsonList(r.Provinces), attrs.Province) {
		return false
	}
	if !listMatches(jsonList(r.Platforms), attrs.Platform) {
		return false
	}
	if !rangeMatches(r.MinWeightKg, r.MaxWeightKg, attrs.WeightKg) {
		return false
	}
	if !rangeMatches(r.MinAmount, r.MaxAmount, attrs.Amount) {
		return false
	}
	return true
}

// Evaluate returns the first matching rule by priority, or an unmatched
// recommendation when no rule applies.
func Evaluate(rules []ShippingRule, attrs MatchAttrs) Recommendation {
	for i := range rules {
		if RuleMatches(&rules[i], attrs) {
			return Recommendation{
				Matched:     true,
				RuleID:      rules[i].ID.String(),
				RuleName:    rules[i].Name,
				CarrierCode: rules[i].CarrierCode,
			}
		}
	}
	return Recommendation{}
}

// Recommend evaluates the tenant's enabled rules against attrs and enriches
// the matched carrier name (skipping rules whose carrier is disabled/missing).
func (s *Service) Recommend(c *gin.Context, attrs MatchAttrs) (*Recommendation, error) {
	if s == nil || s.DB == nil {
		return nil, fmt.Errorf("waybill: no db")
	}
	tid, err := adminperm.TenantIDFromGin(c)
	if err != nil {
		return nil, err
	}
	var rules []ShippingRule
	if err := s.DB.WithContext(c.Request.Context()).
		Where("tenant_id = ? AND enabled = ?", tid, true).
		Order("priority ASC, created_at ASC").Find(&rules).Error; err != nil {
		return nil, err
	}
	for i := range rules {
		if !RuleMatches(&rules[i], attrs) {
			continue
		}
		rec := Recommendation{
			Matched:     true,
			RuleID:      rules[i].ID.String(),
			RuleName:    rules[i].Name,
			CarrierCode: rules[i].CarrierCode,
		}
		if s.Carriers != nil {
			cr, err := s.Carriers.ResolveEnabled(c, rules[i].CarrierCode)
			if err != nil {
				// Rule points at a disabled / removed carrier: keep
				// evaluating lower-priority rules instead of failing.
				continue
			}
			rec.CarrierName = cr.Name
		}
		return &rec, nil
	}
	return &Recommendation{}, nil
}
