package historyaccess

import (
	"net/http/httptest"
	"testing"
	"time"
)

func TestCookieValidChecksServerSideExpiry(t *testing.T) {
	now := time.Date(2026, 5, 2, 12, 0, 0, 0, time.UTC)
	r := httptest.NewRequest("GET", "/", nil)
	w := httptest.NewRecorder()
	SetCookie(w, now)
	r.AddCookie(w.Result().Cookies()[0])
	if !CookieValid(r, now.Add(time.Hour)) {
		t.Fatal("unexpired cookie should allow historical editing")
	}
	if CookieValid(r, now.Add(2*time.Hour)) {
		t.Fatal("expired cookie should not allow historical editing")
	}
}
