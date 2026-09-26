package monthlockhandler

import (
	"net/http"
	"time"

	"cad-development/internal/app"
	"cad-development/internal/db"
	taskhandler "cad-development/internal/handlers/task"
	domain "cad-development/internal/monthlock"
)

func Register(mux *http.ServeMux, q *db.Queries) {
	mux.HandleFunc("GET /api/v1/month-locks", listJSON(q))
	mux.HandleFunc("POST /api/v1/month-locks/{yearMonth}/unlock", setJSON(q, true))
	mux.HandleFunc("POST /api/v1/month-locks/{yearMonth}/lock", setJSON(q, false))
	mux.HandleFunc("POST /month-locks/last/toggle", toggleLastMonth(q))
	mux.HandleFunc("POST /month-locks/unlock", unlockMonth(q))
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

func toggleLastMonth(q *db.Queries) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		currentTime := time.Now()
		toggleAndRender(w, r, q, domain.PreviousMonth(currentTime), currentTime)
	}
}

func unlockMonth(q *db.Queries) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		currentTime := time.Now()
		ym := domain.YearMonth(r.FormValue("year_month"))
		if ym == "" {
			app.WriteError(w, r, app.Invalid("year_month is required"))
			return
		}
		toggleAndRender(w, r, q, ym, currentTime)
	}
}

func toggleAndRender(w http.ResponseWriter, r *http.Request, q *db.Queries, ym domain.YearMonth, now time.Time) {
	if _, err := domain.Toggle(r.Context(), q, ym, now); err != nil {
		app.WriteError(w, r, err)
		return
	}
	if app.IsHTMX(r) {
		taskhandler.RenderGrid(w, r, q, now)
		return
	}
	app.RedirectWithFormFilter(w, r, "/", "project", "subproject")
}
