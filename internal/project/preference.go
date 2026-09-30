package project

import (
	"net/http"
	"strconv"
)

const LastProjectCookie = "cad_last_project"

func PreferredProjectID(r *http.Request, ids []int64) int64 {
	if cookie, err := r.Cookie(LastProjectCookie); err == nil {
		if id, err := strconv.ParseInt(cookie.Value, 10, 64); err == nil {
			for _, candidate := range ids {
				if candidate == id {
					return id
				}
			}
		}
	}
	if len(ids) == 0 {
		return 0
	}
	return ids[0]
}

func RememberProject(w http.ResponseWriter, id int64) {
	http.SetCookie(w, &http.Cookie{
		Name:     LastProjectCookie,
		Value:    strconv.FormatInt(id, 10),
		Path:     "/",
		MaxAge:   60 * 60 * 24 * 365,
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
	})
}
