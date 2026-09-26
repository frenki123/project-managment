package monthlock

import (
	"net/http"
	"time"

	"cad-development/internal/db"
	"cad-development/internal/httpx"
)

func Register(mux *http.ServeMux, q *db.Queries) {
	mux.HandleFunc("GET /api/v1/month-locks", listJSON(q))
	mux.HandleFunc("POST /api/v1/month-locks/{yearMonth}/unlock", setJSON(q, true))
	mux.HandleFunc("POST /api/v1/month-locks/{yearMonth}/lock", setJSON(q, false))
}

func listJSON(q *db.Queries) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		currentTime := time.Now()
		rows, err := q.ListMonthLocks(r.Context())
		if err != nil {
			httpx.Error(w, r, err)
			return
		}
		if rows == nil {
			rows = []db.MonthLock{}
		}
		httpx.JSON(w, http.StatusOK, map[string]any{"month_locks": rows, "last_month": PreviousMonth(currentTime)})
	}
}

func setJSON(q *db.Queries, unlocked bool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		currentTime := time.Now()
		ym := r.PathValue("yearMonth")
		if err := Set(r.Context(), q, ym, unlocked, currentTime); err != nil {
			httpx.Error(w, r, err)
			return
		}
		httpx.JSON(w, http.StatusOK, map[string]any{"year_month": ym, "unlocked": unlocked})
	}
}
