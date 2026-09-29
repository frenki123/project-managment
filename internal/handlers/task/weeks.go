package taskhandler

import (
	"net/http"
	"strings"
	"time"

	"cad-development/internal/app"
	"cad-development/internal/db"
	"cad-development/internal/historyaccess"
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
		cell, err := saveWeek(r, q, id, patch, currentTime)
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
			renderWeekRow(w, r, q, id, app.HTTPErrorFrom(err).Message, app.HTTPErrorFrom(err).Status, currentTime, historicalEditingAllowed)
			return
		}
		patch.Unlock = historicalEditingAllowed
		grid, rowIndex, err := weekGrid(r, q, id, currentTime, historicalEditingAllowed)
		if err != nil {
			app.WriteError(w, r, err)
			return
		}
		if rowIndex < 0 {
			app.WriteError(w, r, app.Invalid("task or week is outside the selected view"))
			return
		}
		_, err = saveWeek(r, q, id, patch, currentTime)
		if err != nil {
			grid.Rows[rowIndex].Cells[weekIndex(grid, rowIndex, r.PathValue("weekStart"))].Error = app.HTTPErrorFrom(err).Message
			w.Header().Set("HX-Reswap", "outerHTML")
			app.RenderFragment(w, r, app.HTTPErrorFrom(err).Status, views.WeekRowResponse(grid, rowIndex))
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
		patch.PlannedHours = &planned
	}
	spent, present, err := app.FormFloatValue(r, "spent_hours")
	if err != nil {
		return weekly.Patch{}, err
	}
	if present {
		patch.SpentHours = &spent
	}
	progress, present, err := app.FormFloatValue(r, "progress")
	if err != nil {
		return weekly.Patch{}, err
	}
	if present {
		if strings.TrimSpace(r.FormValue("progress")) == "" {
			patch.ClearProgress = true
		} else {
			patch.Progress = &progress
		}
	}
	return patch, nil
}

func weekIndex(grid taskdomain.Grid, rowIndex int, weekStart string) int {
	for i, cell := range grid.Rows[rowIndex].Cells {
		if string(cell.WeekStart) == weekStart {
			return i
		}
	}
	return -1
}

func weekGrid(r *http.Request, q *db.Queries, taskID int64, now time.Time, historicalEditingAllowed bool) (taskdomain.Grid, int, error) {
	filter, err := taskdomain.ParseFilter(r.FormValue("project"), r.FormValue("subproject"))
	if err != nil {
		return taskdomain.Grid{}, -1, err
	}
	if err := filter.Validate(r.Context(), q); err != nil {
		return taskdomain.Grid{}, -1, err
	}
	grid, err := taskdomain.LoadGrid(r.Context(), q, filter, now, historicalEditingAllowed)
	if err != nil {
		return taskdomain.Grid{}, -1, err
	}
	for i, row := range grid.Rows {
		if row.ID == taskID && weekIndex(grid, i, r.PathValue("weekStart")) >= 0 {
			return grid, i, nil
		}
	}
	return grid, -1, nil
}

func renderWeekRow(w http.ResponseWriter, r *http.Request, q *db.Queries, taskID int64, errMsg string, status int, now time.Time, historicalEditingAllowed bool) {
	grid, rowIndex, err := weekGrid(r, q, taskID, now, historicalEditingAllowed)
	if err != nil {
		app.WriteError(w, r, err)
		return
	}
	if rowIndex >= 0 {
		grid.Rows[rowIndex].Cells[weekIndex(grid, rowIndex, r.PathValue("weekStart"))].Error = errMsg
		w.Header().Set("HX-Reswap", "outerHTML")
		app.RenderFragment(w, r, status, views.WeekRowResponse(grid, rowIndex))
		return
	}
	app.WriteError(w, r, app.Invalid("task or week is outside the selected view"))
}

func saveWeek(r *http.Request, q *db.Queries, taskID int64, patch weekly.Patch, now time.Time) (weekly.Cell, error) {
	return weekly.Save(r.Context(), q, taskID, weekly.WeekStart(r.PathValue("weekStart")), patch, now)
}
