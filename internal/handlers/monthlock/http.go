package monthlockhandler

import (
	"net/http"
	"time"

	"cad-development/internal/app"
	"cad-development/internal/db"
	taskhandler "cad-development/internal/handlers/task"
	domain "cad-development/internal/monthlock"
	taskdomain "cad-development/internal/task"
)

func Register(mux *http.ServeMux, q *db.Queries) {
	mux.HandleFunc("GET /api/v1/month-locks", getJSON(q))
	mux.HandleFunc("POST /api/v1/month-locks/unlock", setJSON(q, true))
	mux.HandleFunc("POST /api/v1/month-locks/lock", setJSON(q, false))
	mux.HandleFunc("POST /month-locks/set", setHTML(q))
}

func getJSON(q *db.Queries) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		unlocked, err := domain.Unlocked(r.Context(), q)
		if err != nil {
			app.WriteError(w, r, err)
			return
		}
		app.JSON(w, http.StatusOK, domain.State{Unlocked: unlocked})
	}
}

func setJSON(q *db.Queries, unlocked bool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if err := domain.Set(r.Context(), q, unlocked); err != nil {
			app.WriteError(w, r, err)
			return
		}
		app.JSON(w, http.StatusOK, domain.State{Unlocked: unlocked})
	}
}

func setHTML(q *db.Queries) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		state := r.FormValue("unlocked")
		if state != "true" && state != "false" {
			app.WriteError(w, r, app.Invalid("unlocked must be true or false"))
			return
		}
		now := time.Now()
		pk, sid, err := taskhandler.ParseFilter(r)
		if err == nil {
			_, err = taskdomain.LoadGrid(r.Context(), q, pk, sid, now)
		}
		if err != nil {
			app.WriteError(w, r, err)
			return
		}
		if err := domain.Set(r.Context(), q, state == "true"); err != nil {
			app.WriteError(w, r, err)
			return
		}
		if app.IsHTMX(r) {
			taskhandler.RenderGrid(w, r, q, now)
			return
		}
		app.RedirectWithFormFilter(w, r, "/", "project", "subproject")
	}
}
