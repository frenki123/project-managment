package taskhandler

import (
	"net/http"
	"time"

	"cad-development/internal/app"
	"cad-development/internal/db"
	"cad-development/internal/historyaccess"
	taskdomain "cad-development/internal/task"
	"cad-development/internal/views"
)

func gridPage(q *db.Queries) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		now := time.Now()
		RenderGrid(w, r, q, now, historyaccess.CookieValid(r, now))
	}
}

func RenderGrid(w http.ResponseWriter, r *http.Request, q *db.Queries, currentTime time.Time, allowHistoricalEditing bool) {
	filter, err := taskdomain.ParseFilter(r.FormValue("project"), r.FormValue("subproject"))
	if err != nil {
		app.WriteError(w, r, err)
		return
	}
	grid, err := taskdomain.LoadGrid(r.Context(), q, filter, currentTime, allowHistoricalEditing)
	if err != nil {
		app.WriteError(w, r, err)
		return
	}
	if !filter.All && !filter.Ideas {
		app.RememberProject(w, filter.ID)
	}
	if app.IsHTMX(r) {
		w.Header().Set("HX-Push-Url", "/?"+filter.Key())
		app.RenderFragment(w, r, http.StatusOK, views.Grid(grid))
		return
	}
	app.RenderPage(w, r, http.StatusOK, views.GridPage(grid))
}
