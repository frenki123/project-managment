package taskhandler

import (
	"net/http"
	"time"

	"cad-development/internal/app"
	"cad-development/internal/db"
	taskdomain "cad-development/internal/task"
	"cad-development/internal/views"
)

func ParseFilter(r *http.Request) (projectKey string, subprojectID *int64, err error) {
	projectKey = r.FormValue("project")
	subprojectID, err = app.FormInt64Checked(r, "subproject")
	return projectKey, subprojectID, err
}

func gridPage(q *db.Queries) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		RenderGrid(w, r, q, time.Now())
	}
}

func RenderGrid(w http.ResponseWriter, r *http.Request, q *db.Queries, currentTime time.Time) {
	pk, sid, err := ParseFilter(r)
	if err != nil {
		app.WriteError(w, r, err)
		return
	}
	grid, err := taskdomain.LoadGrid(r.Context(), q, pk, sid, currentTime)
	if err != nil {
		app.WriteError(w, r, err)
		return
	}
	if app.IsHTMX(r) {
		app.RenderFragment(w, r, http.StatusOK, views.Grid(grid))
		return
	}
	app.RenderPage(w, r, http.StatusOK, views.GridPage(grid))
}