package weekly_test

import (
	"testing"
	"time"

	"cad-development/internal/db/testkit"
	"cad-development/internal/weekly"
)

func TestIsWeekLocked(t *testing.T) {
	now := time.Date(2026, 5, 2, 0, 0, 0, 0, time.UTC)
	if !testkit.MustWeek(t, "2026-04-27").IsLocked(now) || !testkit.MustWeek(t, "2026-03-30").IsLocked(now) || !testkit.MustWeek(t, "2026-01-05").IsLocked(now) {
		t.Fatal("previous months should be locked")
	}
	if _, err := weekly.Parse("not-a-date"); err == nil || !(weekly.WeekStart{}.IsLocked(now)) {
		t.Fatal("invalid or zero starts must be locked")
	}
	if testkit.MustWeek(t, "2026-05-04").IsLocked(now) || testkit.MustWeek(t, "2026-06-01").IsLocked(now) {
		t.Fatal("current and future weeks should be editable")
	}
	january := time.Date(2026, 1, 2, 12, 0, 0, 0, time.UTC)
	if !testkit.MustWeek(t, "2025-12-29").IsLocked(january) {
		t.Fatal("December week should be locked in January")
	}
	if testkit.MustWeek(t, "2026-01-05").IsLocked(january) {
		t.Fatal("January week should be editable in January")
	}
}
