package product

import (
	"context"
	"fmt"
	"strings"
)

// ComplianceHit is one banned word detected in AI-generated text.
type ComplianceHit struct {
	Word          string `json:"word"`
	Category      string `json:"category"`
	CategoryLabel string `json:"categoryLabel"`
	Level         string `json:"level"`
	LevelLabel    string `json:"levelLabel"`
	Suggestion    string `json:"suggestion,omitempty"`
}

// ComplianceAdvisor supplies the tenant's enabled banned words for AI prompt
// avoidance and rechecks generated text for residual hits.
type ComplianceAdvisor interface {
	// AvoidWords lists enabled forbidden-level words the model must not output.
	AvoidWords(ctx context.Context, tenantID int64) ([]string, error)
	// CheckText scans one text against the tenant's enabled word library.
	CheckText(ctx context.Context, tenantID int64, text string) ([]ComplianceHit, error)
}

const maxAvoidWordsInPrompt = 200

func buildAvoidWordsInstruction(words []string) string {
	if len(words) == 0 {
		return ""
	}
	if len(words) > maxAvoidWordsInPrompt {
		words = words[:maxAvoidWordsInPrompt]
	}
	return fmt.Sprintf("合规要求：输出中严禁出现以下违禁词，若源文案包含请改写为合规表述：%s。", strings.Join(words, "、"))
}

// complianceAvoidWords is best-effort: library errors never block AI generation.
func (s *Service) complianceAvoidWords(ctx context.Context, tenantID int64) []string {
	if s == nil || s.Compliance == nil {
		return nil
	}
	words, err := s.Compliance.AvoidWords(ctx, tenantID)
	if err != nil {
		return nil
	}
	return words
}

// descriptionRecheckText joins every user-visible piece of a generated
// description so the residual banned-word recheck covers all of it.
func descriptionRecheckText(out descriptionGenerateOutput) string {
	parts := []string{out.Description}
	parts = append(parts, out.Highlights...)
	parts = append(parts, out.Specifications...)
	parts = append(parts, out.PackageIncludes...)
	parts = append(parts, out.Notes)
	nonEmpty := make([]string, 0, len(parts))
	for _, p := range parts {
		if strings.TrimSpace(p) != "" {
			nonEmpty = append(nonEmpty, p)
		}
	}
	return strings.Join(nonEmpty, "\n")
}

// complianceRecheck is best-effort: scan failures return no hits.
func (s *Service) complianceRecheck(ctx context.Context, tenantID int64, text string) []ComplianceHit {
	if s == nil || s.Compliance == nil || strings.TrimSpace(text) == "" {
		return nil
	}
	hits, err := s.Compliance.CheckText(ctx, tenantID, text)
	if err != nil {
		return nil
	}
	return hits
}
