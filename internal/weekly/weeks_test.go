package weekly

import (
	"testing"
	"time"
)

func TestWeekStarts(t *testing.T) {
	start := time.Date(2026, 5, 6, 0, 0, 0, 0, time.UTC) // Wednesday
	end := time.Date(2026, 5, 20, 0, 0, 0, 0, time.UTC)  // Wednesday
	got := WeekStarts(start, end)
	for i, want := range []string{"2026-05-04", "2026-05-11", "2026-05-18"} {
		if got[i].String() != want {
			t.Fatalf("got %q want %q", got[i].String(), want)
		}
	}
}

func TestIsCurrent(t *testing.T) {
	tests := []struct {
		week string
		now  time.Time
		want bool
	}{
		{"2026-09-14", time.Date(2026, 9, 16, 0, 0, 0, 0, time.UTC), true},
		{"2026-09-14", time.Date(2026, 9, 10, 0, 0, 0, 0, time.UTC), false},
		{"2026-09-14", time.Date(2026, 9, 21, 0, 0, 0, 0, time.UTC), false},
		{"2025-12-29", time.Date(2025, 12, 31, 0, 0, 0, 0, time.UTC), true},
		{"2025-12-22", time.Date(2025, 12, 31, 0, 0, 0, 0, time.UTC), false},
		{"2025-12-29", time.Date(2026, 1, 5, 0, 0, 0, 0, time.UTC), false},
	}
	for _, tt := range tests {
		ws, err := Parse(tt.week)
		if err != nil {
			t.Fatalf("Parse(%q): %v", tt.week, err)
		}
		if got := ws.IsCurrent(tt.now); got != tt.want {
			t.Fatalf("IsCurrent(%q, %v) = %v, want %v", tt.week, tt.now, got, tt.want)
		}
	}
}

func TestIsCurrentMatchesMondayOnOrBefore(t *testing.T) {
	for _, now := range []time.Time{
		time.Date(2026, 9, 16, 12, 0, 0, 0, time.UTC),
		time.Date(2025, 12, 31, 12, 0, 0, 0, time.UTC),
		time.Date(2026, 1, 1, 12, 0, 0, 0, time.UTC),
	} {
		monday, err := Parse(MondayOnOrBefore(now).Format(time.DateOnly))
		if err != nil {
			t.Fatalf("Parse(MondayOnOrBefore(%v)): %v", now, err)
		}
		if !monday.IsCurrent(now) {
			t.Fatalf("MondayOnOrBefore(%v) = %q, want current", now, monday)
		}
		if monday.Next().IsCurrent(now) {
			t.Fatalf("next week after %q is current for %v", monday, now)
		}
	}
}

func TestParse(t *testing.T) {
	if ws, err := Parse("2026-05-04"); err != nil || ws.String() != "2026-05-04" {
		t.Fatalf("got %q %v", ws, err)
	}
	for _, s := range []string{"2026-05-05", "not-a-date"} {
		ws, err := Parse(s)
		if err == nil {
			t.Fatalf("expected %q to fail", s)
		}
		if !ws.IsZero() {
			t.Fatalf("failed parse %q returned a non-zero week", s)
		}
	}
}
