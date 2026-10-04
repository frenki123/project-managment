package taskhandler

import (
	"context"
	"net/http"
	"slices"
	"strconv"

	"cad-development/internal/db"
	"cad-development/internal/handlers/shared"
	"cad-development/internal/subproject"
	"cad-development/internal/task"
	"cad-development/internal/views"
	"cad-development/internal/web"
)

func formInput(r *http.Request) (task.Input, error) {
	vals := formValues(r)
	projectID, err := web.FormInt64Checked(r, "project_id")
	if err != nil {
		return task.Input{}, err
	}
	subprojectID, err := web.FormInt64Checked(r, "subproject_id")
	if err != nil {
		return task.Input{}, err
	}
	var manualStatus *string
	if vals.ManualStatus != "" {
		manualStatus = &vals.ManualStatus
	}
	return task.Input{
		Name:                vals.Name,
		Description:         vals.Description,
		ImplementationNotes: vals.ImplementationNotes,
		Department:          vals.Department,
		Developers:          vals.Developers,
		Priority:            vals.Priority,
		ProjectID:           projectID,
		SubprojectID:        subprojectID,
		ManualStatus:        manualStatus,
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
		ManualStatus:        r.FormValue("manual_status"),
	}
}

func taskFormValues(t task.Task) views.TaskFormValues {
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
	if t.ManualStatus != nil {
		vals.ManualStatus = *t.ManualStatus
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

func taskSelects(r *http.Request, q *db.Queries, selectedProject, selectedSub *int64) ([]task.Option, []task.Option, error) {
	po, err := shared.ProjectOptions(r, q, selectedProject)
	if err != nil {
		return nil, nil, err
	}
	var so []task.Option
	if selectedProject != nil {
		subs, err := subproject.ListByProject(r.Context(), q, *selectedProject)
		if err != nil {
			return nil, nil, err
		}
		for _, s := range subs {
			v := strconv.FormatInt(s.ID, 10)
			sel := selectedSub != nil && *selectedSub == s.ID
			so = append(so, task.Option{Value: v, Label: s.Name, Selected: sel})
		}
	}
	return po, so, nil
}

func taskFormData(r *http.Request, q *db.Queries, vals views.TaskFormValues, canReassign bool, errMsg string) (views.TaskFormData, error) {
	po, so, err := taskSelects(r, q, parseID(vals.ProjectID), parseID(vals.SubprojectID))
	if err != nil {
		return views.TaskFormData{}, err
	}
	stages, err := q.ListStages(r.Context())
	if err != nil {
		return views.TaskFormData{}, err
	}
	stageOptions := make([]task.Option, 0, len(stages))
	for _, s := range stages {
		stageOptions = append(stageOptions, task.Option{Value: s.Name, Label: s.Name, Selected: s.Name == vals.ManualStatus})
	}
	if vals.ManualStatus != "" && !slices.ContainsFunc(stages, func(s db.Stage) bool { return s.Name == vals.ManualStatus }) {
		stageOptions = append([]task.Option{{Value: vals.ManualStatus, Label: vals.ManualStatus, Selected: true}}, stageOptions...)
	}
	return views.TaskFormData{
		Task:         vals,
		Projects:     po,
		Subprojects:  so,
		Stages:       stageOptions,
		CanReassign:  canReassign,
		ReassignNote: "Project and subproject cannot be changed after weekly data is entered.",
		Error:        errMsg,
	}, nil
}

func renderTaskForm(w http.ResponseWriter, r *http.Request, q *db.Queries, vals views.TaskFormValues, canReassign bool, action, title, deleteAction string, err error, summary *views.TaskPanelData) {
	httpErr := web.HTTPErrorFrom(err)
	data, formErr := taskFormData(r, q, vals, canReassign, httpErr.Message)
	if formErr != nil {
		web.WriteFragmentError(w, r, formErr)
		return
	}
	data.Action, data.Title, data.DeleteAction = action, title, deleteAction
	if summary != nil {
		data.Summary = *summary
		data.DetailPath = "/tasks/" + strconv.FormatInt(summary.Task.ID, 10)
		web.RenderFragment(w, r, httpErr.Status, views.TaskFormPanel(data))
		return
	}
	web.RenderFragment(w, r, httpErr.Status, views.TaskForm(data))
}

func newForm(q *db.Queries) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		pid, err := web.FormInt64Checked(r, "project_id")
		if err != nil {
			web.WriteFragmentError(w, r, err)
			return
		}
		vals := views.TaskFormValues{}
		if pid != nil {
			vals.ProjectID = strconv.FormatInt(*pid, 10)
		}
		data, err := taskFormData(r, q, vals, true, "")
		if err != nil {
			web.WriteFragmentError(w, r, err)
			return
		}
		data.Action, data.Title = "/tasks", "New task"
		web.RenderFragment(w, r, http.StatusOK, views.TaskForm(data))
	}
}

func editForm(q *db.Queries) http.HandlerFunc {
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
		conflict, err := q.TaskReassignConflict(r.Context(), id)
		if err != nil {
			web.WriteFragmentError(w, r, err)
			return
		}
		data, err := taskFormData(r, q, taskFormValues(t), conflict.Status == 0, "")
		if err != nil {
			web.WriteFragmentError(w, r, err)
			return
		}
		idStr := strconv.FormatInt(id, 10)
		data.Action, data.Title, data.DeleteAction = "/tasks/"+idStr, "Edit task", "/tasks/"+idStr+"/delete"
		data.DetailPath = "/tasks/" + idStr
		data.Summary, err = taskPanelData(r, q, t)
		if err != nil {
			web.WriteFragmentError(w, r, err)
			return
		}
		web.RenderFragment(w, r, http.StatusOK, views.TaskFormPanel(data))
	}
}

func createHTML(q *db.Queries) http.HandlerFunc {
	return shared.CreateForm(q, formInput, task.Create, func(w http.ResponseWriter, r *http.Request, err error) {
		renderTaskForm(w, r, q, formValues(r), true, "/tasks", "New task", "", err, nil)
	}, afterTask)
}

func updateHTML(q *db.Queries) http.HandlerFunc {
	return shared.UpdateForm(q, formInput, func(ctx context.Context, q *db.Queries, id int64, in task.Input) (task.Task, error) {
		return task.Update(ctx, q, id, task.PatchFromInput(in))
	}, func(w http.ResponseWriter, r *http.Request, id int64, err error) {
		renderTaskUpdateError(w, r, q, id, err)
	}, afterTask)
}

func renderTaskUpdateError(w http.ResponseWriter, r *http.Request, q *db.Queries, id int64, err error) {
	vals := formValues(r)
	canReassign := true
	var summary *views.TaskPanelData
	if current, getErr := task.Get(r.Context(), q, id); getErr == nil {
		panel, panelErr := taskPanelData(r, q, current)
		if panelErr == nil {
			summary = &panel
		}
		conflict, conflictErr := q.TaskReassignConflict(r.Context(), id)
		canReassign = conflictErr == nil && conflict.Status == 0
		if !canReassign {
			currentVals := taskFormValues(current)
			vals.ProjectID, vals.SubprojectID = currentVals.ProjectID, currentVals.SubprojectID
		}
	}
	idStr := strconv.FormatInt(id, 10)
	renderTaskForm(w, r, q, vals, canReassign, "/tasks/"+idStr, "Edit task", "/tasks/"+idStr+"/delete", err, summary)
}

func deleteHTML(q *db.Queries) http.HandlerFunc {
	return shared.DeleteForm(q, task.Get, task.Delete, func(w http.ResponseWriter, r *http.Request, t task.Task, err error) {
		summary, summaryErr := taskPanelData(r, q, t)
		if summaryErr != nil {
			web.WriteFragmentError(w, r, summaryErr)
			return
		}
		httpErr := web.HTTPErrorFrom(err)
		web.SetToast(w, httpErr.Message)
		web.RenderFragment(w, r, httpErr.Status, views.TaskPanel(taskPanelError(summary, httpErr.Message)))
	}, afterTask)
}

func afterTask(t task.Task) string {
	if t.ProjectID == nil {
		return "/?" + task.FilterKey(task.Filter{Ideas: true})
	}
	return "/?" + task.FilterKey(task.Filter{ID: *t.ProjectID, Subproject: t.SubprojectID})
}
