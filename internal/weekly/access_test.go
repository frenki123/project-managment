package weekly

import (
	"testing"
	"time"
)

func mustWeek(t *testing.T, s string) WeekStart {
	t.Helper()
	ws, err := Parse(s)
	if err != nil {
		t.Fatal(err)
	}
	return ws
}

func TestIsWeekLocked(t *testing.T) {
	now := time.Date(2026, 5, 2, 0, 0, 0, 0, time.UTC)
	if !mustWeek(t, "2026-04-27").IsLocked(now) || !mustWeek(t, "2026-03-30").IsLocked(now) || !mustWeek(t, "2026-01-05").IsLocked(now) {
		t.Fatal("previous months should be locked")
	}
	if _, err := Parse("not-a-date"); err == nil || !(WeekStart{}.IsLocked(now)) {
		t.Fatal("invalid or zero starts must be locked")
	}
	if mustWeek(t, "2026-05-04").IsLocked(now) || mustWeek(t, "2026-06-01").IsLocked(now) {
		t.Fatal("current and future weeks should be editable")
	}
	january := time.Date(2026, 1, 2, 12, 0, 0, 0, time.UTC)
	if !mustWeek(t, "2025-12-29").IsLocked(january) {
		t.Fatal("December week should be locked in January")
	}
	if mustWeek(t, "2026-01-05").IsLocked(january) {
		t.Fatal("January week should be editable in January")
	}
}
