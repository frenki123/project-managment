package taskhandler

import (
	"net/http"
	"time"

	"cad-development/internal/db"
	"cad-development/internal/historyaccess"
	"cad-development/internal/project"
	"cad-development/internal/task"
	"cad-development/internal/views"
	"cad-development/internal/web"
)

func gridPage(q *db.Queries) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		now := time.Now()
		RenderGrid(w, r, q, now, historyaccess.CookieValid(r, now))
	}
}

func RenderGrid(w http.ResponseWriter, r *http.Request, q *db.Queries, currentTime time.Time, allowHistoricalEditing bool) {
	filter, err := task.ParseFilter(r.FormValue("project"), r.FormValue("subproject"))
	if err != nil {
		web.WriteError(w, r, err)
		return
	}
	resolved, err := task.ResolveFilter(r.Context(), q, filter)
	if err != nil {
		web.WriteError(w, r, err)
		return
	}
	grid, err := task.LoadResolvedGrid(r.Context(), q, resolved, currentTime, allowHistoricalEditing)
	if err != nil {
		web.WriteError(w, r, err)
		return
	}
	if !filter.All && !filter.Ideas {
		project.RememberProject(w, filter.ID)
	}
	if web.IsHTMX(r) {
		w.Header().Set("HX-Push-Url", "/?"+task.FilterKey(filter))
		web.RenderFragment(w, r, http.StatusOK, views.Grid(grid))
		return
	}
	web.RenderPage(w, r, http.StatusOK, views.GridPage(grid))
}
