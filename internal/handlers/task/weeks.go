package taskhandler

import (
	"net/http"
	"time"

	"cad-development/internal/app"
	"cad-development/internal/db"
	handlererrors "cad-development/internal/handlers/errors"
	"cad-development/internal/monthlock"
	taskdomain "cad-development/internal/task"
	"cad-development/internal/views"
	"cad-development/internal/weekly"
)

func weekJSON(q *db.Queries) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		currentTime := time.Now()
		id, err := app.PathID(r, "id")
		if err != nil {
			handlererrors.WriteError(w, r, err)
			return
		}
		var patch weekly.Patch
		if err := app.DecodeJSON(w, r, &patch); err != nil {
			handlererrors.WriteError(w, r, err)
			return
		}
		cell, err := saveWeek(r, q, id, patch, currentTime)
		if err != nil {
			handlererrors.WriteError(w, r, err)
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
			handlererrors.WriteError(w, r, err)
			return
		}
		plannedValue, plannedPresent, err := app.FormFloatValue(r, "planned_hours")
		if err != nil {
			handlererrors.WriteError(w, r, err)
			return
		}
		spentValue, spentPresent, err := app.FormFloatValue(r, "spent_hours")
		if err != nil {
			handlererrors.WriteError(w, r, err)
			return
		}
		progressValue, progressPresent, err := app.FormFloatValue(r, "progress")
		if err != nil {
			handlererrors.WriteError(w, r, err)
			return
		}
		var patch weekly.Patch
		if plannedPresent {
			patch.PlannedHours = &plannedValue
		}
		if spentPresent {
			patch.SpentHours = &spentValue
		}
		if progressPresent {
			patch.Progress = &progressValue
		}
		_, err = saveWeek(r, q, id, patch, currentTime)
		if err != nil {
			httpErr := handlererrors.ToHTTPError(err)
			renderWeekRow(w, r, q, id, httpErr.Message, currentTime)
			return
		}
		renderWeekRow(w, r, q, id, "", currentTime)
	}
}

func renderWeekRow(w http.ResponseWriter, r *http.Request, q *db.Queries, taskID int64, errMsg string, now time.Time) {
	projectKey, subprojectID, err := ParseFilter(r)
	if err != nil {
		handlererrors.WriteError(w, r, err)
		return
	}
	grid, err := taskdomain.LoadGrid(r.Context(), q, projectKey, subprojectID, now)
	if err != nil {
		handlererrors.WriteError(w, r, err)
		return
	}
	data := toViewGrid(grid, "")
	weekStart := r.PathValue("weekStart")
	for i := range data.Rows {
		if data.Rows[i].ID != taskID {
			continue
		}
		for j := range data.Rows[i].Cells {
			if data.Rows[i].Cells[j].WeekStart != weekStart {
				continue
			}
			data.Rows[i].Cells[j].Error = errMsg
			handlererrors.Render(w, r, http.StatusOK, views.WeekRowResponse(views.WeekRowResponseData{Row: data.Rows[i], WeekTotals: data.WeekTotals, Totals: data.Totals}))
			return
		}
	}
	w.WriteHeader(http.StatusNoContent)
}

func saveWeek(r *http.Request, q *db.Queries, taskID int64, patch weekly.Patch, now time.Time) (weekly.Cell, error) {
	unlocked, err := monthlock.UnlockedSet(r.Context(), q)
	if err != nil {
		return weekly.Cell{}, err
	}
	return weekly.Save(r.Context(), q, taskID, weekly.WeekStart(r.PathValue("weekStart")), patch, now, unlocked)
}
