package bannedwords

import (
	"sort"
	"strings"
)

// FieldText is one scannable text field of a product draft.
type FieldText struct {
	// Field is the code, e.g. title / aiTitle / description / aiDescription.
	Field string `json:"field"`
	// Label is the Chinese label shown in results.
	Label string `json:"label"`
	Text  string `json:"text"`
}

// HitPosition is one occurrence of a word inside a field (rune offsets).
type HitPosition struct {
	Start int `json:"start"`
	End   int `json:"end"`
}

// Hit is one banned word matched inside one field.
type Hit struct {
	Word          string        `json:"word"`
	Field         string        `json:"field"`
	FieldLabel    string        `json:"fieldLabel"`
	Category      string        `json:"category"`
	CategoryLabel string        `json:"categoryLabel"`
	Level         string        `json:"level"`
	LevelLabel    string        `json:"levelLabel"`
	Suggestion    string        `json:"suggestion,omitempty"`
	Positions     []HitPosition `json:"positions"`
}

const maxPositionsPerHit = 50

// Scan matches every enabled word against every field, returning hits with
// rune-based positions so the frontend can highlight matches precisely.
func Scan(fields []FieldText, words []BannedWord) []Hit {
	out := make([]Hit, 0)
	for _, f := range fields {
		text := f.Text
		if strings.TrimSpace(text) == "" {
			continue
		}
		lower := strings.ToLower(text)
		lowerRunes := []rune(lower)
		for _, w := range words {
			word := strings.ToLower(strings.TrimSpace(w.Word))
			if word == "" {
				continue
			}
			positions := findRunePositions(lowerRunes, []rune(word))
			if len(positions) == 0 {
				continue
			}
			out = append(out, Hit{
				Word:          w.Word,
				Field:         f.Field,
				FieldLabel:    f.Label,
				Category:      w.Category,
				CategoryLabel: CategoryLabel(w.Category),
				Level:         w.Level,
				LevelLabel:    LevelLabel(w.Level),
				Suggestion:    w.Suggestion,
				Positions:     positions,
			})
		}
	}
	sort.SliceStable(out, func(i, j int) bool {
		if out[i].Level != out[j].Level {
			return out[i].Level == LevelForbidden
		}
		if out[i].Field != out[j].Field {
			return out[i].Field < out[j].Field
		}
		return out[i].Word < out[j].Word
	})
	return out
}

func findRunePositions(text, word []rune) []HitPosition {
	if len(word) == 0 || len(text) < len(word) {
		return nil
	}
	var out []HitPosition
	for i := 0; i+len(word) <= len(text); i++ {
		match := true
		for j := range word {
			if text[i+j] != word[j] {
				match = false
				break
			}
		}
		if match {
			out = append(out, HitPosition{Start: i, End: i + len(word)})
			if len(out) >= maxPositionsPerHit {
				break
			}
		}
	}
	return out
}
