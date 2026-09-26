package taskhandler

import (
	"net/http"
	"strconv"

	"cad-development/internal/app"
	"cad-development/internal/db"
	"cad-development/internal/project"
	"cad-development/internal/subproject"
	taskdomain "cad-development/internal/task"
	"cad-development/internal/views"
)

func panel(q *db.Queries) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, err := app.PathID(r, "id")
		if err != nil {
			app.WriteFragmentError(w, r, err)
			return
		}
		t, err := taskdomain.Get(r.Context(), q, id)
		if err != nil {
			app.WriteFragmentError(w, r, err)
			return
		}
		row := taskdomain.GridRow{
			ID:         t.ID,
			Name:       t.Name,
			TotalHours: t.TotalHours,
			SpentHours: t.SpentHours,
			Progress:   t.Progress,
		}
		if t.ProjectID != nil {
			p, err := project.Get(r.Context(), q, *t.ProjectID)
			if err != nil {
				app.WriteFragmentError(w, r, err)
				return
			}
			row.ProjectName = p.Name
		}
		if t.SubprojectID != nil {
			s, err := subproject.Get(r.Context(), q, *t.SubprojectID)
			if err != nil {
				app.WriteFragmentError(w, r, err)
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
			DeletePath: "/tasks/" + strconv.FormatInt(t.ID, 10) + "/delete",
		}
		if app.IsHTMX(r) {
			app.RenderFragment(w, r, http.StatusOK, views.TaskPanel(data))
			return
		}
		app.RenderPage(w, r, http.StatusOK, views.TaskPanelPage(data))
	}
}
