package taskhandler

import (
	"net/http"
	"strconv"
	"strings"

	"cad-development/internal/db"
	"cad-development/internal/handlers/shared"
	"cad-development/internal/person"
	"cad-development/internal/project"
	"cad-development/internal/subproject"
	"cad-development/internal/task"
	"cad-development/internal/views"
	"cad-development/internal/web"
)

func panel(q *db.Queries) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, ok := shared.PathID(w, r)
		if !ok {
			return
		}
		t, err := task.Get(r.Context(), q, id)
		if err != nil {
			web.WriteFragmentError(w, r, err)
			return
		}
		data, err := taskPanelData(r, q, t)
		if err != nil {
			web.WriteFragmentError(w, r, err)
			return
		}
		web.RenderFragment(w, r, http.StatusOK, views.TaskPanel(data))
	}
}

func taskPanelData(r *http.Request, q *db.Queries, t task.Task) (views.TaskPanelData, error) {
	row := task.GridRow{
		ID:         t.ID,
		Name:       t.Name,
		TotalHours: t.TotalHours,
		SpentHours: t.SpentHours,
		Progress:   t.Progress,
		Status:     t.Status,
	}
	if t.ProjectID != nil {
		p, err := project.Get(r.Context(), q, *t.ProjectID)
		if err != nil {
			return views.TaskPanelData{}, err
		}
		row.ProjectName = p.Name
	}
	if t.SubprojectID != nil {
		s, err := subproject.Get(r.Context(), q, *t.SubprojectID)
		if err != nil {
			return views.TaskPanelData{}, err
		}
		row.Subproject = s.Name
	}
	id := strconv.FormatInt(t.ID, 10)
	return views.TaskPanelData{
		Task:       row,
		Department: t.Department,
		Developers: joinedNames(t.Developers),
		Priority:   t.Priority,
		Notes:      t.ImplementationNotes,
		Desc:       t.Description,
		EditPath:   "/tasks/" + id + "/edit",
		DeletePath: "/tasks/" + id + "/delete",
	}, nil
}

func joinedNames(people []person.Ref) string {
	names := make([]string, 0, len(people))
	for _, p := range people {
		names = append(names, p.Name)
	}
	return strings.Join(names, ", ")
}

func taskPanelError(data views.TaskPanelData, message string) views.TaskPanelData {
	data.Error = message
	return data
}
