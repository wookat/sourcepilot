// Package csvsafe neutralizes spreadsheet formula injection (CSV injection)
// in exported CSV cells. Excel / WPS / Numbers evaluate any cell starting with
// =, +, -, @ or a leading control character as a formula, so user-controlled
// text (product titles, customer names, imported raw rows) must be escaped
// before it is written into a downloadable report.
package csvsafe

import "strings"

// dangerousPrefixes are the characters that make a spreadsheet treat the cell
// as a formula. Tab / CR are included because they are stripped by spreadsheet
// parsers, exposing the character behind them.
const dangerousPrefixes = "=+-@\t\r"

// Cell escapes one CSV cell: a leading formula trigger is prefixed with a
// single quote, which spreadsheets render as plain text. Values that are safe
// (including plain negative numbers) are returned unchanged.
func Cell(v string) string {
	if v == "" {
		return v
	}
	if !strings.ContainsRune(dangerousPrefixes, rune(v[0])) {
		return v
	}
	if isNumeric(v) {
		return v
	}
	return "'" + v
}

// Row escapes every cell of a CSV record in place-free fashion.
func Row(cells []string) []string {
	out := make([]string, len(cells))
	for i, c := range cells {
		out[i] = Cell(c)
	}
	return out
}

// isNumeric reports whether v is a plain decimal number (optionally signed),
// which spreadsheets treat as a value rather than a formula.
func isNumeric(v string) bool {
	i := 0
	if v[0] == '-' || v[0] == '+' {
		i = 1
	}
	if i >= len(v) {
		return false
	}
	dot := false
	for ; i < len(v); i++ {
		switch {
		case v[i] >= '0' && v[i] <= '9':
		case v[i] == '.' && !dot:
			dot = true
		default:
			return false
		}
	}
	return true
}
