package taskhandler

import (
	"net/http"
	"strconv"

	"cad-development/internal/app"
	"cad-development/internal/db"
	handlererrors "cad-development/internal/handlers/errors"
	"cad-development/internal/project"
	"cad-development/internal/subproject"
	taskdomain "cad-development/internal/task"
	"cad-development/internal/views"
)

func panel(q *db.Queries) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, err := app.PathID(r, "id")
		if err != nil {
			handlererrors.WriteError(w, r, err)
			return
		}
		t, err := taskdomain.Get(r.Context(), q, id)
		if err != nil {
			handlererrors.WriteError(w, r, err)
			return
		}
		row := views.TaskRow{
			ID:         t.ID,
			Name:       t.Name,
			TotalHours: t.TotalHours,
			SpentHours: t.SpentHours,
			Progress:   t.Progress,
			DetailPath: "/tasks/" + strconv.FormatInt(t.ID, 10),
		}
		if t.ProjectID != nil {
			p, err := project.Get(r.Context(), q, *t.ProjectID)
			if err != nil {
				handlererrors.WriteError(w, r, err)
				return
			}
			row.ProjectName = p.Name
		}
		if t.SubprojectID != nil {
			s, err := subproject.Get(r.Context(), q, *t.SubprojectID)
			if err != nil {
				handlererrors.WriteError(w, r, err)
				return
			}
			row.Subproject = s.Name
		}
		data := views.TaskPanelData{
			Task:       row,
			Department: t.Department,
			Developers: t.Developers,
			Priority:   t.Priority,
			Notes:      t.ImplementationNotes,
			Desc:       t.Description,
			EditPath:   "/tasks/" + strconv.FormatInt(t.ID, 10) + "/edit",
		}
		app.HTML(w, http.StatusOK)
		if app.IsHTMX(r) {
			_ = views.TaskPanel(data).Render(r.Context(), w)
			return
		}
		_ = views.TaskPanelPage(data).Render(r.Context(), w)
	}
}
