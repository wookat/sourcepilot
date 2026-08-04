package csvsafe

import "testing"

func TestCellNeutralizesFormulaPrefixes(t *testing.T) {
	cases := map[string]string{
		"=cmd|'/C calc'!A1": "'=cmd|'/C calc'!A1",
		"+SUM(1+1)":         "'+SUM(1+1)",
		"@formula":          "'@formula",
		"\tlead-tab":        "'\tlead-tab",
		"\rlead-cr":         "'\rlead-cr",
		"-2+3":              "'-2+3",
		"":                  "",
		"正常文本":              "正常文本",
		"-12.5":             "-12.5",
		"+42":               "+42",
		"12.5":              "12.5",
	}
	for in, want := range cases {
		if got := Cell(in); got != want {
			t.Errorf("Cell(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestRowNeutralizesEveryCell(t *testing.T) {
	got := Row([]string{"=A1", "ok", "@x"})
	want := []string{"'=A1", "ok", "'@x"}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("Row()[%d] = %q, want %q", i, got[i], want[i])
		}
	}
}
