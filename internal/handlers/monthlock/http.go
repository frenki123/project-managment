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
	mux.HandleFunc("GET /api/v1/month-locks", listJSON(q))
	mux.HandleFunc("POST /api/v1/month-locks/{yearMonth}/unlock", setJSON(q, true))
	mux.HandleFunc("POST /api/v1/month-locks/{yearMonth}/lock", setJSON(q, false))
	mux.HandleFunc("POST /month-locks/last/set", setLastMonth(q))
	mux.HandleFunc("POST /month-locks/set", setMonth(q))
}

func listJSON(q *db.Queries) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		currentTime := time.Now()
		rows, err := domain.List(r.Context(), q)
		if err != nil {
			app.WriteError(w, r, err)
			return
		}
		app.JSON(w, http.StatusOK, domain.LocksResponse{MonthLocks: rows, LastMonth: domain.PreviousMonth(currentTime)})
	}
}

func setJSON(q *db.Queries, unlocked bool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		currentTime := time.Now()
		ym := r.PathValue("yearMonth")
		if err := domain.SetMonth(r.Context(), q, domain.YearMonth(ym), unlocked, currentTime); err != nil {
			app.WriteError(w, r, err)
			return
		}
		app.JSON(w, http.StatusOK, domain.SetResponse{YearMonth: domain.YearMonth(ym), Unlocked: unlocked})
	}
}

func setLastMonth(q *db.Queries) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		currentTime := time.Now()
		setAndRender(w, r, q, domain.PreviousMonth(currentTime), currentTime)
	}
}

func setMonth(q *db.Queries) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		currentTime := time.Now()
		ym := domain.YearMonth(r.FormValue("year_month"))
		if ym == "" {
			app.WriteError(w, r, app.Invalid("year_month is required"))
			return
		}
		setAndRender(w, r, q, ym, currentTime)
	}
}

func setAndRender(w http.ResponseWriter, r *http.Request, q *db.Queries, ym domain.YearMonth, now time.Time) {
	state := r.FormValue("unlocked")
	if state != "true" && state != "false" {
		app.WriteError(w, r, app.Invalid("unlocked must be true or false"))
		return
	}
	pk, sid, err := taskhandler.ParseFilter(r)
	if err == nil {
		_, err = taskdomain.LoadGrid(r.Context(), q, pk, sid, now)
	}
	if err != nil {
		app.WriteError(w, r, err)
		return
	}
	if err := domain.SetMonth(r.Context(), q, ym, state == "true", now); err != nil {
		app.WriteError(w, r, err)
		return
	}
	if app.IsHTMX(r) {
		taskhandler.RenderGrid(w, r, q, now)
		return
	}
	app.RedirectWithFormFilter(w, r, "/", "project", "subproject")
}
