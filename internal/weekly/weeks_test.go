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

func TestParse(t *testing.T) {
	if ws, err := Parse("2026-05-04"); err != nil || ws.String() != "2026-05-04" {
		t.Fatalf("got %q %v", ws, err)
	}
	for _, s := range []string{"2026-05-05", "not-a-date"} {
		if _, err := Parse(s); err == nil {
			t.Fatalf("expected %q to fail", s)
		}
	}
}
