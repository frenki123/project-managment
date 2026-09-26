package taskhandler

import (
	"net/http"
	"time"

	"cad-development/internal/app"
	"cad-development/internal/db"
	handlererrors "cad-development/internal/handlers/errors"
	"cad-development/internal/monthlock"
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
		planned, err := app.FormFloat(r, "planned_hours")
		if err != nil {
			handlererrors.WriteError(w, r, err)
			return
		}
		spent, err := app.FormFloat(r, "spent_hours")
		if err != nil {
			handlererrors.WriteError(w, r, err)
			return
		}
		progress, err := app.FormFloat(r, "progress")
		if err != nil {
			handlererrors.WriteError(w, r, err)
			return
		}
		_, err = saveWeek(r, q, id, weekly.Patch{
			PlannedHours: planned,
			SpentHours:   spent,
			Progress:     progress,
		}, currentTime)
		if err != nil {
			httpErr := handlererrors.ToHTTPError(err)
			RenderGrid(w, r, q, httpErr.Message, currentTime)
			return
		}
		RenderGrid(w, r, q, "", currentTime)
	}
}

func saveWeek(r *http.Request, q *db.Queries, taskID int64, patch weekly.Patch, now time.Time) (weekly.Cell, error) {
	unlocked, err := monthlock.UnlockedSet(r.Context(), q)
	if err != nil {
		return weekly.Cell{}, err
	}
	return weekly.Save(r.Context(), q, taskID, r.PathValue("weekStart"), patch, now, unlocked)
}
