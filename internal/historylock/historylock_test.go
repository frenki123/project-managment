package historylock

import (
	"net/http/httptest"
	"testing"
	"time"
)

func TestIsPastMonth(t *testing.T) {
	now := time.Date(2026, 5, 2, 12, 0, 0, 0, time.UTC)
	april := time.Date(2026, 4, 1, 0, 0, 0, 0, time.UTC)
	march := time.Date(2026, 3, 1, 0, 0, 0, 0, time.UTC)
	if !IsPastMonth(april, now) || !IsPastMonth(march, now) {
		t.Fatal("all past months should be locked on 2 May")
	}
	if IsPastMonth(now, now) {
		t.Fatal("current month should be editable")
	}
	if IsPastMonth(now.AddDate(0, 1, 0), now) {
		t.Fatal("future month should be editable")
	}
	if !IsPastMonth(time.Date(2025, 12, 29, 0, 0, 0, 0, time.UTC), time.Date(2026, 1, 2, 0, 0, 0, 0, time.UTC)) {
		t.Fatal("December should be locked in January")
	}
}

func TestIsWeekLockedUsesMondayMonth(t *testing.T) {
	now := time.Date(2026, 5, 2, 0, 0, 0, 0, time.UTC)
	if !IsWeekLocked("2026-04-27", now) || !IsWeekLocked("2026-03-30", now) {
		t.Fatal("week starting in April should be locked in May")
	}
	if IsWeekLocked("2026-05-04", now) {
		t.Fatal("week starting in May should be open")
	}
}

func TestHistoricalEditingCookieValidatesServerSideExpiry(t *testing.T) {
	now := time.Date(2026, 5, 2, 12, 0, 0, 0, time.UTC)
	r := httptest.NewRequest("GET", "/", nil)
	w := httptest.NewRecorder()
	SetHistoricalEditingCookie(w, now)
	r.AddCookie(w.Result().Cookies()[0])
	if !HistoricalEditingCookieValid(r, now.Add(time.Hour)) {
		t.Fatal("unexpired cookie should unlock history")
	}
	if HistoricalEditingCookieValid(r, now.Add(2*time.Hour)) {
		t.Fatal("expired cookie should not unlock history")
	}
}
