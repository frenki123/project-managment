package monthlock

import (
	"testing"
	"time"
)

func TestLocked(t *testing.T) {
	now := time.Date(2026, 5, 2, 12, 0, 0, 0, time.UTC)
	unlocked := NewSet()
	if !Locked("2026-04", now, unlocked) {
		t.Fatal("April should be locked on 2 May")
	}
	if Locked("2026-05", now, unlocked) {
		t.Fatal("current month should be editable")
	}
	if Locked("2026-06", now, unlocked) {
		t.Fatal("future month should be editable")
	}
	unlocked = NewSet("2026-04")
	if Locked("2026-04", now, unlocked) {
		t.Fatal("unlocked April should be editable")
	}
}

func TestPreviousMonth(t *testing.T) {
	now := time.Date(2026, 1, 15, 0, 0, 0, 0, time.UTC)
	if got := PreviousMonth(now); got != "2025-12" {
		t.Fatalf("got %s", got)
	}
}

func TestWeekLockedUsesMondayMonth(t *testing.T) {
	now := time.Date(2026, 5, 2, 0, 0, 0, 0, time.UTC)
	if !WeekLocked("2026-04-27", now, nil) {
		t.Fatal("week starting in April should be locked in May")
	}
	if WeekLocked("2026-05-04", now, nil) {
		t.Fatal("week starting in May should be open")
	}
}
