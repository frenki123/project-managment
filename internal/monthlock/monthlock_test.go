package monthlock

import (
	"net/http/httptest"
	"testing"
	"time"
)

func TestLocked(t *testing.T) {
	now := time.Date(2026, 5, 2, 12, 0, 0, 0, time.UTC)
	april := time.Date(2026, 4, 1, 0, 0, 0, 0, time.UTC)
	march := time.Date(2026, 3, 1, 0, 0, 0, 0, time.UTC)
	if !Locked(april, now) || !Locked(march, now) {
		t.Fatal("all past months should be locked on 2 May")
	}
	if Locked(now, now) {
		t.Fatal("current month should be editable")
	}
	if Locked(now.AddDate(0, 1, 0), now) {
		t.Fatal("future month should be editable")
	}
	if !Locked(time.Date(2025, 12, 29, 0, 0, 0, 0, time.UTC), time.Date(2026, 1, 2, 0, 0, 0, 0, time.UTC)) {
		t.Fatal("December should be locked in January")
	}
}

func TestWeekLockedUsesMondayMonth(t *testing.T) {
	now := time.Date(2026, 5, 2, 0, 0, 0, 0, time.UTC)
	if !WeekLocked("2026-04-27", now) || !WeekLocked("2026-03-30", now) {
		t.Fatal("week starting in April should be locked in May")
	}
	if WeekLocked("2026-05-04", now) {
		t.Fatal("week starting in May should be open")
	}
}

func TestCookieValidatesServerSideExpiry(t *testing.T) {
	now := time.Date(2026, 5, 2, 12, 0, 0, 0, time.UTC)
	r := httptest.NewRequest("GET", "/", nil)
	w := httptest.NewRecorder()
	SetCookie(w, now)
	r.AddCookie(w.Result().Cookies()[0])
	if !CookieValid(r, now.Add(time.Hour)) {
		t.Fatal("unexpired cookie should unlock history")
	}
	if CookieValid(r, now.Add(2*time.Hour)) {
		t.Fatal("expired cookie should not unlock history")
	}
}
