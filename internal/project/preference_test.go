package project

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestPreferredProjectIDUsesValidCookieOrFirstProject(t *testing.T) {
	ids := []int64{3, 7}

	check := func(name, value string, want int64) {
		t.Helper()
		r := httptest.NewRequest("GET", "/", nil)
		if value != "" {
			r.AddCookie(&http.Cookie{Name: LastProjectCookie, Value: value}) //nolint:gosec // test request cookie; Secure is not applicable to a localhost request cookie
		}
		if got := PreferredProjectID(r, ids); got != want {
			t.Fatalf("%s: preferred project = %d, want %d", name, got, want)
		}
	}
	check("missing", "", 3)
	check("valid", "7", 7)
	check("stale", "99", 3)
	check("invalid", "nope", 3)
}
