package historyaccess

import (
	"net/http"
	"strconv"
	"time"
)

const (
	HistoricalEditingCookieName = "historical_editing_until"
	historicalEditingDuration   = 2 * time.Hour
)

func CookieValid(r *http.Request, now time.Time) bool {
	cookie, err := r.Cookie(HistoricalEditingCookieName)
	if err != nil {
		return false
	}
	expires, err := strconv.ParseInt(cookie.Value, 10, 64)
	return err == nil && expires > now.Unix()
}

func SetCookie(w http.ResponseWriter, now time.Time) {
	expires := now.Add(historicalEditingDuration)
	http.SetCookie(w, &http.Cookie{ //nolint:gosec // Secure is omitted: localhost HTTP never transmits Secure cookies
		Name:     HistoricalEditingCookieName,
		Value:    strconv.FormatInt(expires.Unix(), 10),
		Path:     "/",
		Expires:  expires,
		MaxAge:   int(historicalEditingDuration / time.Second),
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
	})
}

func ClearCookie(w http.ResponseWriter) {
	http.SetCookie(w, &http.Cookie{ //nolint:gosec // Secure is omitted: localhost HTTP never transmits Secure cookies
		Name:     HistoricalEditingCookieName,
		Value:    "",
		Path:     "/",
		Expires:  time.Unix(1, 0),
		MaxAge:   -1,
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
	})
}
