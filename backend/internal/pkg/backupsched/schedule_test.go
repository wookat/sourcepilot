package backupsched

import (
	"testing"
	"time"
)

func mustParse(t *testing.T, expr string) Schedule {
	t.Helper()
	s, err := Parse(expr)
	if err != nil {
		t.Fatalf("Parse(%q): %v", expr, err)
	}
	return s
}

func TestParseIntervalForms(t *testing.T) {
	base := time.Date(2026, 8, 6, 10, 0, 0, 0, time.UTC)
	for _, expr := range []string{"@every 6h", "6h", "90m"} {
		s := mustParse(t, expr)
		next := s.Next(base)
		if !next.After(base) {
			t.Fatalf("%q: next %v not after base", expr, next)
		}
	}
	if _, err := Parse("@every 5s"); err == nil {
		t.Fatal("sub-minute interval must be rejected")
	}
	if _, err := Parse(""); err == nil {
		t.Fatal("empty schedule must be rejected")
	}
}

func TestCronDailyAt3(t *testing.T) {
	s := mustParse(t, "0 3 * * *")
	got := s.Next(time.Date(2026, 8, 6, 10, 15, 0, 0, time.UTC))
	want := time.Date(2026, 8, 7, 3, 0, 0, 0, time.UTC)
	if !got.Equal(want) {
		t.Fatalf("got %v want %v", got, want)
	}
	// Just before today's fire time -> fires today.
	got = s.Next(time.Date(2026, 8, 6, 2, 59, 0, 0, time.UTC))
	want = time.Date(2026, 8, 6, 3, 0, 0, 0, time.UTC)
	if !got.Equal(want) {
		t.Fatalf("got %v want %v", got, want)
	}
}

func TestCronStepsListsRanges(t *testing.T) {
	s := mustParse(t, "*/15 8-18 * * 1-5")
	got := s.Next(time.Date(2026, 8, 8, 9, 0, 0, 0, time.UTC)) // Saturday
	want := time.Date(2026, 8, 10, 8, 0, 0, 0, time.UTC)       // Monday 08:00
	if !got.Equal(want) {
		t.Fatalf("got %v want %v", got, want)
	}
	s = mustParse(t, "0 4 * * 0")
	got = s.Next(time.Date(2026, 8, 6, 10, 0, 0, 0, time.UTC)) // Thursday
	want = time.Date(2026, 8, 9, 4, 0, 0, 0, time.UTC)         // Sunday 04:00
	if !got.Equal(want) {
		t.Fatalf("got %v want %v", got, want)
	}
	s = mustParse(t, "30 2,14 1 * *")
	got = s.Next(time.Date(2026, 8, 6, 10, 0, 0, 0, time.UTC))
	want = time.Date(2026, 9, 1, 2, 30, 0, 0, time.UTC)
	if !got.Equal(want) {
		t.Fatalf("got %v want %v", got, want)
	}
}

func TestCronDowSevenIsSunday(t *testing.T) {
	s := mustParse(t, "0 4 * * 7")
	got := s.Next(time.Date(2026, 8, 6, 10, 0, 0, 0, time.UTC))
	want := time.Date(2026, 8, 9, 4, 0, 0, 0, time.UTC)
	if !got.Equal(want) {
		t.Fatalf("got %v want %v", got, want)
	}
}

func TestCronRejectsInvalid(t *testing.T) {
	for _, expr := range []string{
		"0 3 * *",       // 4 fields
		"60 3 * * *",    // minute out of range
		"0 24 * * *",    // hour out of range
		"0 3 * * 1-x",   // bad range value
		"0 3 * * */0",   // zero step
		"0 3 5-2 * *",   // descending range
		"not a cron",    // garbage
		"@every nonsen", // bad duration
	} {
		if _, err := Parse(expr); err == nil {
			t.Fatalf("Parse(%q) must fail", expr)
		}
	}
}
