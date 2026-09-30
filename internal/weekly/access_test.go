package weekly

import (
	"testing"
	"time"
)

func TestIsWeekLocked(t *testing.T) {
	now := time.Date(2026, 5, 2, 0, 0, 0, 0, time.UTC)
	if !WeekStart("2026-04-27").IsLocked(now) || !WeekStart("2026-03-30").IsLocked(now) || !WeekStart("2026-01-04").IsLocked(now) || !WeekStart("not-a-date").IsLocked(now) {
		t.Fatal("previous months and invalid starts should be locked")
	}
	if WeekStart("2026-05-04").IsLocked(now) || WeekStart("2026-06-01").IsLocked(now) {
		t.Fatal("current and future weeks should be editable")
	}
	january := time.Date(2026, 1, 2, 12, 0, 0, 0, time.UTC)
	if !WeekStart("2025-12-29").IsLocked(january) {
		t.Fatal("December week should be locked in January")
	}
	if WeekStart("2026-01-05").IsLocked(january) {
		t.Fatal("January week should be editable in January")
	}
}
