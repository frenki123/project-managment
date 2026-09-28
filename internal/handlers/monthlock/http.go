package monthlockhandler

import (
	"net/http"
	"time"

	"cad-development/internal/app"
	"cad-development/internal/db"
	taskhandler "cad-development/internal/handlers/task"
	"cad-development/internal/monthlock"
	taskdomain "cad-development/internal/task"
)

func Register(mux *http.ServeMux, q *db.Queries) {
	mux.HandleFunc("POST /month-locks/set", setHTML(q))
}

func setHTML(q *db.Queries) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		state := r.FormValue("unlocked")
		if state != "true" && state != "false" {
			app.WriteError(w, r, app.Invalid("unlocked must be true or false"))
			return
		}
		now := time.Now()
		pk, sid, err := taskdomain.ParseFilter(r.FormValue("project"), r.FormValue("subproject"))
		if err == nil {
			err = taskdomain.ValidateFilter(r.Context(), q, pk, sid)
		}
		if err != nil {
			app.WriteError(w, r, err)
			return
		}
		unlocked := state == "true"
		if unlocked {
			monthlock.SetCookie(w, now)
		} else {
			monthlock.ClearCookie(w)
		}
		if app.IsHTMX(r) {
			taskhandler.RenderGrid(w, r, q, now, unlocked)
			return
		}
		app.RedirectWithFormFilter(w, r, "/", "project", "subproject")
	}
}
