package monthlock

import (
	"net/http"
	"strconv"
	"time"
)

const (
	UnlockCookieName = "history_unlock_until"
	unlockDuration   = 2 * time.Hour
)

func CookieValid(r *http.Request, now time.Time) bool {
	cookie, err := r.Cookie(UnlockCookieName)
	if err != nil {
		return false
	}
	expires, err := strconv.ParseInt(cookie.Value, 10, 64)
	return err == nil && expires > now.Unix()
}

func SetCookie(w http.ResponseWriter, now time.Time) {
	expires := now.Add(unlockDuration)
	http.SetCookie(w, &http.Cookie{
		Name:     UnlockCookieName,
		Value:    strconv.FormatInt(expires.Unix(), 10),
		Path:     "/",
		Expires:  expires,
		MaxAge:   int(unlockDuration / time.Second),
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
	})
}

func ClearCookie(w http.ResponseWriter) {
	http.SetCookie(w, &http.Cookie{
		Name:     UnlockCookieName,
		Value:    "",
		Path:     "/",
		Expires:  time.Unix(1, 0),
		MaxAge:   -1,
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
	})
}
