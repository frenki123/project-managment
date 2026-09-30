package taskhandler

import (
	"net/http"
	"strings"
	"time"

	"cad-development/internal/app"
	"cad-development/internal/db"
	"cad-development/internal/historyaccess"
	"cad-development/internal/nullable"
	taskdomain "cad-development/internal/task"
	"cad-development/internal/views"
	"cad-development/internal/weekly"
)

func weekJSON(q *db.Queries) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		currentTime := time.Now()
		id, err := app.PathID(r, "id")
		if err != nil {
			app.WriteError(w, r, err)
			return
		}
		var patch weekly.Patch
		if err := app.DecodeJSON(w, r, &patch); err != nil {
			app.WriteError(w, r, err)
			return
		}
		cell, err := weekly.Save(r.Context(), q, id, weekly.WeekStart(r.PathValue("weekStart")), patch, currentTime)
		if err != nil {
			app.WriteError(w, r, err)
			return
		}
		app.JSON(w, http.StatusOK, cell)
	}
}

func weekHTML(q *db.Queries) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		currentTime := time.Now()
		id, err := app.PathID(r, "id")
		if err != nil {
			app.WriteError(w, r, err)
			return
		}
		historicalEditingAllowed := historyaccess.CookieValid(r, currentTime)
		patch, err := formWeekPatch(r)
		if err != nil {
			httpErr := app.HTTPErrorFrom(err)
			renderWeekRow(w, r, q, id, httpErr.Message, httpErr.Status, currentTime, historicalEditingAllowed)
			return
		}
		patch.Unlock = historicalEditingAllowed
		grid, rowIndex, cellIndex, err := weekGrid(r, q, id, currentTime, historicalEditingAllowed)
		if err != nil {
			app.WriteError(w, r, err)
			return
		}
		if rowIndex < 0 {
			app.WriteError(w, r, app.Invalid("task or week is outside the selected view"))
			return
		}
		_, err = weekly.Save(r.Context(), q, id, weekly.WeekStart(r.PathValue("weekStart")), patch, currentTime)
		if err != nil {
			httpErr := app.HTTPErrorFrom(err)
			grid.Rows[rowIndex].Cells[cellIndex].Error = httpErr.Message
			w.Header().Set("HX-Reswap", "outerHTML")
			app.RenderFragment(w, r, httpErr.Status, views.WeekRowResponse(grid, rowIndex))
			return
		}
		renderWeekRow(w, r, q, id, "", http.StatusOK, currentTime, historicalEditingAllowed)
	}
}

func formWeekPatch(r *http.Request) (weekly.Patch, error) {
	var patch weekly.Patch
	planned, present, err := app.FormFloatValue(r, "planned_hours")
	if err != nil {
		return weekly.Patch{}, err
	}
	if present {
		patch.PlannedHours = nullable.Present(planned)
	}
	spent, present, err := app.FormFloatValue(r, "spent_hours")
	if err != nil {
		return weekly.Patch{}, err
	}
	if present {
		patch.SpentHours = nullable.Present(spent)
	}
	progress, present, err := app.FormFloatValue(r, "progress")
	if err != nil {
		return weekly.Patch{}, err
	}
	if present {
		if strings.TrimSpace(r.FormValue("progress")) == "" {
			patch.Progress = nullable.Clear[float64]()
		} else {
			patch.Progress = nullable.Present(progress)
		}
	}
	return patch, nil
}

func weekIndex(grid taskdomain.Grid, rowIndex int, weekStart string) int {
	for i, cell := range grid.Rows[rowIndex].Cells {
		if cell.WeekStart.String() == weekStart {
			return i
		}
	}
	return -1
}

func weekGrid(r *http.Request, q *db.Queries, taskID int64, now time.Time, historicalEditingAllowed bool) (taskdomain.Grid, int, int, error) {
	filter, err := taskdomain.ParseFilter(r.FormValue("project"), r.FormValue("subproject"))
	if err != nil {
		return taskdomain.Grid{}, -1, -1, err
	}
	resolved, err := filter.Resolve(r.Context(), q)
	if err != nil {
		return taskdomain.Grid{}, -1, -1, err
	}
	grid, err := taskdomain.LoadResolvedGrid(r.Context(), q, resolved, now, historicalEditingAllowed)
	if err != nil {
		return taskdomain.Grid{}, -1, -1, err
	}
	for i, row := range grid.Rows {
		if row.ID == taskID {
			if cell := weekIndex(grid, i, r.PathValue("weekStart")); cell >= 0 {
				return grid, i, cell, nil
			}
		}
	}
	return grid, -1, -1, nil
}

func renderWeekRow(w http.ResponseWriter, r *http.Request, q *db.Queries, taskID int64, errMsg string, status int, now time.Time, historicalEditingAllowed bool) {
	grid, rowIndex, cellIndex, err := weekGrid(r, q, taskID, now, historicalEditingAllowed)
	if err != nil {
		app.WriteError(w, r, err)
		return
	}
	if rowIndex >= 0 {
		grid.Rows[rowIndex].Cells[cellIndex].Error = errMsg
		w.Header().Set("HX-Reswap", "outerHTML")
		app.RenderFragment(w, r, status, views.WeekRowResponse(grid, rowIndex))
		return
	}
	app.WriteError(w, r, app.Invalid("task or week is outside the selected view"))
}
