package bannedwords

import (
	"context"

	"github.com/trademind-ai/trademind/backend/internal/modules/product"
)

// AIComplianceAdvisor adapts the banned-word library to product.ComplianceAdvisor
// so AI text generation can avoid enabled words and recheck its output.
type AIComplianceAdvisor struct {
	Svc *Service
}

var _ product.ComplianceAdvisor = (*AIComplianceAdvisor)(nil)

// AvoidWords lists the tenant's enabled forbidden-level words.
func (a *AIComplianceAdvisor) AvoidWords(ctx context.Context, tenantID int64) ([]string, error) {
	words, err := a.Svc.ActiveWords(ctx, tenantID)
	if err != nil {
		return nil, err
	}
	out := make([]string, 0, len(words))
	for _, w := range words {
		if w.Level == LevelForbidden {
			out = append(out, w.Word)
		}
	}
	return out, nil
}

// CheckText scans one AI-generated text against the tenant's enabled library.
func (a *AIComplianceAdvisor) CheckText(ctx context.Context, tenantID int64, text string) ([]product.ComplianceHit, error) {
	words, err := a.Svc.ActiveWords(ctx, tenantID)
	if err != nil {
		return nil, err
	}
	hits := Scan([]FieldText{{Field: "aiOutput", Label: "AI 输出", Text: text}}, words)
	out := make([]product.ComplianceHit, 0, len(hits))
	for _, h := range hits {
		out = append(out, product.ComplianceHit{
			Word:          h.Word,
			Category:      h.Category,
			CategoryLabel: h.CategoryLabel,
			Level:         h.Level,
			LevelLabel:    h.LevelLabel,
			Suggestion:    h.Suggestion,
		})
	}
	return out, nil
}
