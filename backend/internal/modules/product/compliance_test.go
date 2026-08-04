package product

import (
	"strings"
	"testing"
)

func TestBuildAvoidWordsInstruction(t *testing.T) {
	if buildAvoidWordsInstruction(nil) != "" {
		t.Fatal("empty word list must produce empty instruction")
	}
	instr := buildAvoidWordsInstruction([]string{"最佳", "第一"})
	if !strings.Contains(instr, "最佳") || !strings.Contains(instr, "第一") {
		t.Fatalf("instruction missing words: %s", instr)
	}
	if !strings.Contains(instr, "严禁") {
		t.Fatalf("instruction missing avoidance directive: %s", instr)
	}

	many := make([]string, maxAvoidWordsInPrompt+50)
	for i := range many {
		many[i] = "样例词条"
	}
	capped := buildAvoidWordsInstruction(many)
	if got := strings.Count(capped, "样例词条"); got != maxAvoidWordsInPrompt {
		t.Fatalf("expected cap at %d words, got %d", maxAvoidWordsInPrompt, got)
	}
}

func TestDescriptionRecheckTextJoinsAllParts(t *testing.T) {
	out := descriptionGenerateOutput{
		Description:     "主描述",
		Highlights:      []string{"亮点1", ""},
		Specifications:  []string{"规格1"},
		PackageIncludes: []string{"清单1"},
		Notes:           "备注",
	}
	text := descriptionRecheckText(out)
	for _, want := range []string{"主描述", "亮点1", "规格1", "清单1", "备注"} {
		if !strings.Contains(text, want) {
			t.Fatalf("recheck text missing %q: %s", want, text)
		}
	}
	if strings.Contains(text, "\n\n") {
		t.Fatalf("empty parts must be dropped: %q", text)
	}
}
