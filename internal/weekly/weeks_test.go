package weekly

import (
	"slices"
	"testing"
	"time"
)

func TestWeekStarts(t *testing.T) {
	start := time.Date(2026, 5, 6, 0, 0, 0, 0, time.UTC) // Wednesday
	end := time.Date(2026, 5, 20, 0, 0, 0, 0, time.UTC)  // Wednesday
	got := WeekStarts(start, end)
	want := []WeekStart{"2026-05-04", "2026-05-11", "2026-05-18"}
	if !slices.Equal(got, want) {
		t.Fatalf("got %v want %v", got, want)
	}
}

func TestParseMonday(t *testing.T) {
	if _, err := ParseMonday("2026-05-04"); err != nil {
		t.Fatal(err)
	}
	if _, err := ParseMonday("2026-05-05"); err == nil {
		t.Fatal("expected error for non-Monday")
	}
}
