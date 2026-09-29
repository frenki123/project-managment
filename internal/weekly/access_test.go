package weekly

import (
	"testing"
	"time"
)

func TestIsWeekLockedUsesWeekStartMonth(t *testing.T) {
	now := time.Date(2026, 5, 2, 0, 0, 0, 0, time.UTC)
	if !IsWeekLocked("2026-04-27", now) || !IsWeekLocked("2026-03-30", now) {
		t.Fatal("weeks starting in previous months should be locked")
	}
	if IsWeekLocked("2026-05-04", now) || IsWeekLocked("2026-06-01", now) {
		t.Fatal("current and future weeks should be editable")
	}
	if !IsWeekLocked("2026-01-04", now) {
		t.Fatal("non-Monday week starts should be locked")
	}
	if !IsWeekLocked("not-a-date", now) {
		t.Fatal("invalid week starts should be locked")
	}
}

func TestIsWeekLockedHandlesYearBoundary(t *testing.T) {
	now := time.Date(2026, 1, 2, 12, 0, 0, 0, time.UTC)
	if !IsWeekLocked("2025-12-29", now) {
		t.Fatal("December week should be locked in January")
	}
	if IsWeekLocked("2026-01-05", now) {
		t.Fatal("January week should be editable in January")
	}
}
