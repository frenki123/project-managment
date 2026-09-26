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

func formInput(r *http.Request) (taskdomain.Input, error) {
	projectID, err := app.FormInt64Checked(r, "project_id")
	if err != nil {
		return taskdomain.Input{}, err
	}
	subprojectID, err := app.FormInt64Checked(r, "subproject_id")
	if err != nil {
		return taskdomain.Input{}, err
	}
	return taskdomain.Input{
		Name:                r.FormValue("name"),
		Description:         r.FormValue("description"),
		ImplementationNotes: r.FormValue("implementation_notes"),
		Department:          r.FormValue("department"),
		Developers:          r.FormValue("developers"),
		Priority:            r.FormValue("priority"),
		ProjectID:           projectID,
		SubprojectID:        subprojectID,
	}, nil
}

func formValues(r *http.Request) views.TaskFormValues {
	return views.TaskFormValues{
		Name:                r.FormValue("name"),
		Description:         r.FormValue("description"),
		ImplementationNotes: r.FormValue("implementation_notes"),
		Department:          r.FormValue("department"),
		Developers:          r.FormValue("developers"),
		Priority:            r.FormValue("priority"),
		ProjectID:           r.FormValue("project_id"),
		SubprojectID:        r.FormValue("subproject_id"),
	}
}

func taskFormValues(t taskdomain.Task) views.TaskFormValues {
	vals := views.TaskFormValues{
		Name:                t.Name,
		Description:         t.Description,
		ImplementationNotes: t.ImplementationNotes,
		Department:          t.Department,
		Developers:          t.Developers,
		Priority:            t.Priority,
	}
	if t.ProjectID != nil {
		vals.ProjectID = strconv.FormatInt(*t.ProjectID, 10)
	}
	if t.SubprojectID != nil {
		vals.SubprojectID = strconv.FormatInt(*t.SubprojectID, 10)
	}
	return vals
}

func parseID(s string) *int64 {
	if s == "" {
		return nil
	}
	if id, err := strconv.ParseInt(s, 10, 64); err == nil && id > 0 {
		return new(id)
	}
	return nil
}

func taskSelects(r *http.Request, q *db.Queries, selectedProject, selectedSub *int64) ([]taskdomain.Option, []taskdomain.Option, error) {
	projects, err := project.List(r.Context(), q)
	if err != nil {
		return nil, nil, err
	}
	var subs []subproject.Subproject
	if selectedProject != nil {
		subs, err = subproject.ListByProject(r.Context(), q, *selectedProject)
		if err != nil {
			return nil, nil, err
		}
	}
	var po []taskdomain.Option
	for _, p := range projects {
		v := strconv.FormatInt(p.ID, 10)
		sel := selectedProject != nil && *selectedProject == p.ID
		po = append(po, taskdomain.Option{Value: v, Label: p.Name, Selected: sel})
	}
	var so []taskdomain.Option
	for _, s := range subs {
		v := strconv.FormatInt(s.ID, 10)
		sel := selectedSub != nil && *selectedSub == s.ID
		so = append(so, taskdomain.Option{Value: v, Label: s.Name, Selected: sel})
	}
	return po, so, nil
}

func taskFormData(r *http.Request, q *db.Queries, vals views.TaskFormValues, canReassign bool, errMsg string) (views.TaskFormData, error) {
	po, so, err := taskSelects(r, q, parseID(vals.ProjectID), parseID(vals.SubprojectID))
	if err != nil {
		return views.TaskFormData{}, err
	}
	return views.TaskFormData{
		Task:         vals,
		Projects:     po,
		Subprojects:  so,
		CanReassign:  canReassign,
		ReassignNote: "Project and subproject cannot be changed after weekly data is entered.",
		Error:        errMsg,
	}, nil
}

func renderTaskForm(w http.ResponseWriter, r *http.Request, q *db.Queries, vals views.TaskFormValues, canReassign bool, action, title, deleteAction string, err error) {
	httpErr := app.HTTPErrorFrom(err)
	data, formErr := taskFormData(r, q, vals, canReassign, httpErr.Message)
	if formErr != nil {
		app.WriteError(w, r, formErr)
		return
	}
	data.Action, data.Title, data.DeleteAction = action, title, deleteAction
	app.RenderPage(w, r, httpErr.Status, views.TaskForm(data))
}

func newForm(q *db.Queries) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		pid, err := app.FormInt64Checked(r, "project_id")
		if err != nil {
			app.WriteError(w, r, err)
			return
		}
		po, so, err := taskSelects(r, q, pid, nil)
		if err != nil {
			app.WriteError(w, r, err)
			return
		}
		vals := views.TaskFormValues{}
		if pid != nil {
			vals.ProjectID = strconv.FormatInt(*pid, 10)
		}
		app.RenderPage(w, r, http.StatusOK, views.TaskForm(views.TaskFormData{
			Action:      "/tasks",
			Title:       "New task",
			Task:        vals,
			Projects:    po,
			Subprojects: so,
			CanReassign: true,
		}))
	}
}

func editForm(q *db.Queries) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, err := app.PathID(r, "id")
		if err != nil {
			app.WriteError(w, r, err)
			return
		}
		t, err := taskdomain.Get(r.Context(), q, id)
		if err != nil {
			app.WriteError(w, r, err)
			return
		}
		data, err := taskFormData(r, q, taskFormValues(t), len(t.Weeks) == 0, "")
		if err != nil {
			app.WriteError(w, r, err)
			return
		}
		idStr := strconv.FormatInt(id, 10)
		data.Action, data.Title, data.DeleteAction = "/tasks/"+idStr, "Edit task", "/tasks/"+idStr+"/delete"
		app.RenderPage(w, r, http.StatusOK, views.TaskForm(data))
	}
}

func createHTML(q *db.Queries) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		in, err := formInput(r)
		if err != nil {
			renderTaskForm(w, r, q, formValues(r), true, "/tasks", "New task", "", err)
			return
		}
		t, err := taskdomain.Create(r.Context(), q, in)
		if err != nil {
			renderTaskForm(w, r, q, formValues(r), true, "/tasks", "New task", "", err)
			return
		}
		app.Redirect(w, r, afterTask(t))
	}
}

func updateHTML(q *db.Queries) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, err := app.PathID(r, "id")
		if err != nil {
			app.WriteError(w, r, err)
			return
		}
		in, err := formInput(r)
		if err != nil {
			renderTaskUpdateError(w, r, q, id, err)
			return
		}
		t, err := taskdomain.Update(r.Context(), q, id, in)
		if err != nil {
			renderTaskUpdateError(w, r, q, id, err)
			return
		}
		app.Redirect(w, r, afterTask(t))
	}
}

func renderTaskUpdateError(w http.ResponseWriter, r *http.Request, q *db.Queries, id int64, err error) {
	vals := formValues(r)
	canReassign := true
	if current, getErr := taskdomain.Get(r.Context(), q, id); getErr == nil {
		canReassign = len(current.Weeks) == 0
		if !canReassign {
			vals.ProjectID, vals.SubprojectID = "", ""
			if current.ProjectID != nil {
				vals.ProjectID = strconv.FormatInt(*current.ProjectID, 10)
			}
			if current.SubprojectID != nil {
				vals.SubprojectID = strconv.FormatInt(*current.SubprojectID, 10)
			}
		}
	}
	idStr := strconv.FormatInt(id, 10)
	renderTaskForm(w, r, q, vals, canReassign, "/tasks/"+idStr, "Edit task", "/tasks/"+idStr+"/delete", err)
}

func deleteHTML(q *db.Queries) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, err := app.PathID(r, "id")
		if err != nil {
			app.WriteError(w, r, err)
			return
		}
		t, err := taskdomain.Get(r.Context(), q, id)
		if err != nil {
			app.WriteError(w, r, err)
			return
		}
		if err := taskdomain.Delete(r.Context(), q, id); err != nil {
			idStr := strconv.FormatInt(id, 10)
			renderTaskForm(w, r, q, taskFormValues(t), len(t.Weeks) == 0, "/tasks/"+idStr, "Edit task", "/tasks/"+idStr+"/delete", err)
			return
		}
		app.Redirect(w, r, afterTask(t))
	}
}

func afterTask(t taskdomain.Task) string {
	if t.ProjectID == nil {
		return "/?project=ideas"
	}
	u := "/?project=" + strconv.FormatInt(*t.ProjectID, 10)
	if t.SubprojectID != nil {
		u += "&subproject=" + strconv.FormatInt(*t.SubprojectID, 10)
	}
	return u
}
