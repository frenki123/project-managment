package taskhandler

import (
	"net/http"
	"strconv"
	"time"

	"cad-development/internal/app"
	"cad-development/internal/db"
	"cad-development/internal/project"
	taskdomain "cad-development/internal/task"
	"cad-development/internal/views"
)

func gridPage(q *db.Queries) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("project") == "" && r.URL.Query().Get("subproject") == "" {
			projects, err := project.List(r.Context(), q)
			if err != nil {
				app.WriteError(w, r, err)
				return
			}
			ids := make([]int64, 0, len(projects))
			for _, p := range projects {
				ids = append(ids, p.ID)
			}
			if id := app.PreferredProjectID(r, ids); id > 0 {
				app.Redirect(w, r, "/?project="+strconv.FormatInt(id, 10))
				return
			}
		}
		RenderGrid(w, r, q, time.Now())
	}
}

func RenderGrid(w http.ResponseWriter, r *http.Request, q *db.Queries, currentTime time.Time) {
	pk, sid, err := taskdomain.ParseFilter(r.FormValue("project"), r.FormValue("subproject"))
	if err != nil {
		app.WriteError(w, r, err)
		return
	}
	normalized, err := taskdomain.NormalizeSubprojectFilter(r.Context(), q, pk, sid)
	if err != nil {
		app.WriteError(w, r, err)
		return
	}
	if sid != nil && normalized == nil {
		app.Redirect(w, r, "/?project="+pk)
		return
	}
	sid = normalized
	if pk != "ideas" {
		if id, parseErr := strconv.ParseInt(pk, 10, 64); parseErr == nil {
			app.RememberProject(w, id)
		}
	}
	grid, err := taskdomain.LoadGrid(r.Context(), q, pk, sid, currentTime)
	if err != nil {
		app.WriteError(w, r, err)
		return
	}
	if app.IsHTMX(r) {
		url := "/?project=" + pk
		if sid != nil {
			url += "&subproject=" + strconv.FormatInt(*sid, 10)
		}
		w.Header().Set("HX-Push-Url", url)
		app.RenderFragment(w, r, http.StatusOK, views.Grid(grid))
		return
	}
	app.RenderPage(w, r, http.StatusOK, views.GridPage(grid))
}
