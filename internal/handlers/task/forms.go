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

func taskSelects(r *http.Request, q *db.Queries, selectedProject, selectedSub *int64) ([]views.Option, []views.Option, error) {
	projects, err := project.List(r.Context(), q)
	if err != nil {
		return nil, nil, err
	}
	subs, err := subproject.List(r.Context(), q)
	if err != nil {
		return nil, nil, err
	}
	projNames := map[int64]string{}
	var po []views.Option
	for _, p := range projects {
		projNames[p.ID] = p.Name
		v := strconv.FormatInt(p.ID, 10)
		sel := selectedProject != nil && *selectedProject == p.ID
		po = append(po, views.Option{Value: v, Label: p.Name, Selected: sel})
	}
	var so []views.Option
	for _, s := range subs {
		v := strconv.FormatInt(s.ID, 10)
		sel := selectedSub != nil && *selectedSub == s.ID
		so = append(so, views.Option{Value: v, Label: projNames[s.ProjectID] + " / " + s.Name, Selected: sel})
	}
	return po, so, nil
}

func newForm(q *db.Queries) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		pid, err := app.FormInt64Checked(r, "project_id")
		if err != nil {
			handlererrors.WriteError(w, r, err)
			return
		}
		po, so, err := taskSelects(r, q, pid, nil)
		if err != nil {
			handlererrors.WriteError(w, r, err)
			return
		}
		vals := views.TaskFormValues{}
		if pid != nil {
			vals.ProjectID = strconv.FormatInt(*pid, 10)
		}
		app.HTML(w, http.StatusOK)
		_ = views.TaskForm(views.TaskFormData{
			Action:      "/tasks",
			Title:       "New task",
			Task:        vals,
			Projects:    po,
			Subprojects: so,
		}).Render(r.Context(), w)
	}
}

func editForm(q *db.Queries) http.HandlerFunc {
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
		app.HTML(w, http.StatusOK)
		data, err := taskFormData(r, q, t, "")
		if err != nil {
			handlererrors.WriteError(w, r, err)
			return
		}
		_ = views.TaskForm(data).Render(r.Context(), w)
	}
}

func taskFormData(r *http.Request, q *db.Queries, t taskdomain.Task, errMsg string) (views.TaskFormData, error) {
	po, so, err := taskSelects(r, q, t.ProjectID, t.SubprojectID)
	if err != nil {
		return views.TaskFormData{}, err
	}
	id := strconv.FormatInt(t.ID, 10)
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
	return views.TaskFormData{
		Action:       "/tasks/" + id,
		Title:        "Edit task",
		Task:         vals,
		Projects:     po,
		Subprojects:  so,
		Error:        errMsg,
		DeleteAction: "/tasks/" + id + "/delete",
	}, nil
}

func createHTML(q *db.Queries) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		in, err := formInput(r)
		if err != nil {
			handlererrors.WriteError(w, r, err)
			return
		}
		t, err := taskdomain.Create(r.Context(), q, in)
		if err != nil {
			dummy := taskdomain.Task{
				Name: in.Name, Description: in.Description, ImplementationNotes: in.ImplementationNotes,
				Department: in.Department, Developers: in.Developers, Priority: in.Priority,
				ProjectID: in.ProjectID, SubprojectID: in.SubprojectID,
			}
			app.HTML(w, http.StatusBadRequest)
			data, formErr := taskFormData(r, q, dummy, err.Error())
			if formErr != nil {
				handlererrors.WriteError(w, r, formErr)
				return
			}
			data.Action = "/tasks"
			data.Title = "New task"
			data.DeleteAction = ""
			_ = views.TaskForm(data).Render(r.Context(), w)
			return
		}
		app.Redirect(w, r, afterTask(t))
	}
}

func updateHTML(q *db.Queries) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, err := app.PathID(r, "id")
		if err != nil {
			handlererrors.WriteError(w, r, err)
			return
		}
		in, err := formInput(r)
		if err != nil {
			handlererrors.WriteError(w, r, err)
			return
		}
		t, err := taskdomain.Update(r.Context(), q, id, in)
		if err != nil {
			cur := taskdomain.Task{ID: id, Name: in.Name, Description: in.Description, ImplementationNotes: in.ImplementationNotes,
				Department: in.Department, Developers: in.Developers, Priority: in.Priority,
				ProjectID: in.ProjectID, SubprojectID: in.SubprojectID}
			app.HTML(w, http.StatusBadRequest)
			data, formErr := taskFormData(r, q, cur, err.Error())
			if formErr != nil {
				handlererrors.WriteError(w, r, formErr)
				return
			}
			_ = views.TaskForm(data).Render(r.Context(), w)
			return
		}
		app.Redirect(w, r, afterTask(t))
	}
}

func deleteHTML(q *db.Queries) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, err := app.PathID(r, "id")
		if err != nil {
			handlererrors.WriteError(w, r, err)
			return
		}
		if err := taskdomain.Delete(r.Context(), q, id); err != nil {
			handlererrors.WriteError(w, r, err)
			return
		}
		app.Redirect(w, r, "/")
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
