package monthlockhandler

import (
	"net/http"
	"time"

	"cad-development/internal/app"
	"cad-development/internal/db"
	handlererrors "cad-development/internal/handlers/errors"
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
			handlererrors.WriteError(w, r, err)
			return
		}
		app.JSON(w, http.StatusOK, map[string]any{"month_locks": rows, "last_month": domain.PreviousMonth(currentTime)})
	}
}

func setJSON(q *db.Queries, unlocked bool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		currentTime := time.Now()
		ym := r.PathValue("yearMonth")
		if err := domain.Set(r.Context(), q, ym, unlocked, currentTime); err != nil {
			handlererrors.WriteError(w, r, err)
			return
		}
		app.JSON(w, http.StatusOK, map[string]any{"year_month": ym, "unlocked": unlocked})
	}
}

func toggleLastMonth(q *db.Queries) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		currentTime := time.Now()
		unlocked, err := domain.UnlockedSet(r.Context(), q)
		if err != nil {
			handlererrors.WriteError(w, r, err)
			return
		}
		last := domain.PreviousMonth(currentTime)
		if err := domain.Set(r.Context(), q, last, !unlocked[last], currentTime); err != nil {
			handlererrors.WriteError(w, r, err)
			return
		}
		app.Redirect(w, r, "/")
	}
}

func unlockMonth(q *db.Queries) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		currentTime := time.Now()
		if err := domain.Set(r.Context(), q, r.FormValue("year_month"), true, currentTime); err != nil {
			handlererrors.WriteError(w, r, err)
			return
		}
		app.Redirect(w, r, "/")
	}
}
