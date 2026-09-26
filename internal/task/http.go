package task

import (
	"errors"
	"net/http"
	"strconv"
	"time"

	"cad-development/internal/apperr"
	"cad-development/internal/db"
	"cad-development/internal/httpx"
	"cad-development/internal/monthlock"
	"cad-development/internal/project"
	"cad-development/internal/subproject"
	"cad-development/internal/views"
	"cad-development/internal/weekly"
)

func Register(mux *http.ServeMux, q *db.Queries) {
	mux.HandleFunc("GET /{$}", gridPage(q))
	mux.HandleFunc("GET /api/v1/tasks", listJSON(q))
	mux.HandleFunc("POST /api/v1/tasks", createJSON(q))
	mux.HandleFunc("GET /api/v1/tasks/{id}", getJSON(q))
	mux.HandleFunc("PUT /api/v1/tasks/{id}", updateJSON(q))
	mux.HandleFunc("DELETE /api/v1/tasks/{id}", deleteJSON(q))
	mux.HandleFunc("PUT /api/v1/tasks/{id}/weeks/{weekStart}", weekJSON(q))
	mux.HandleFunc("GET /tasks/new", newForm(q))
	mux.HandleFunc("POST /tasks", createHTML(q))
	mux.HandleFunc("GET /tasks/{id}", panel(q))
	mux.HandleFunc("GET /tasks/{id}/edit", editForm(q))
	mux.HandleFunc("POST /tasks/{id}", updateHTML(q))
	mux.HandleFunc("POST /tasks/{id}/delete", deleteHTML(q))
	mux.HandleFunc("POST /tasks/{id}/weeks/{weekStart}", weekHTML(q))
	mux.HandleFunc("POST /month-locks/last/toggle", toggleLastMonth(q))
	mux.HandleFunc("POST /month-locks/unlock", unlockMonth(q))
}

func gridPage(q *db.Queries) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		renderGrid(w, r, q, "", time.Now())
	}
}

func renderGrid(w http.ResponseWriter, r *http.Request, q *db.Queries, errMsg string, currentTime time.Time) {
	pk, sid, err := ParseFilter(r)
	if err != nil {
		httpx.Error(w, r, err)
		return
	}
	data, err := LoadGrid(r.Context(), q, pk, sid, currentTime)
	if err != nil {
		httpx.Error(w, r, err)
		return
	}
	data.Error = errMsg
	httpx.HTML(w, http.StatusOK)
	if httpx.IsHTMX(r) {
		_ = views.Grid(data).Render(r.Context(), w)
		return
	}
	_ = views.GridPage(data).Render(r.Context(), w)
}

func listJSON(q *db.Queries) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var (
			list []Task
			err  error
		)
		projectID, projectErr := httpx.FormInt64Checked(r, "project_id")
		subprojectID, subprojectErr := httpx.FormInt64Checked(r, "subproject_id")
		if projectErr != nil {
			httpx.Error(w, r, projectErr)
			return
		}
		if subprojectErr != nil {
			httpx.Error(w, r, subprojectErr)
			return
		}
		switch {
		case r.URL.Query().Get("ideas") == "true":
			list, err = ListIdeas(r.Context(), q)
		case subprojectID != nil:
			list, err = ListBySubproject(r.Context(), q, *subprojectID)
		case projectID != nil:
			list, err = ListByProject(r.Context(), q, *projectID)
		default:
			list, err = List(r.Context(), q)
		}
		if err != nil {
			httpx.Error(w, r, err)
			return
		}
		httpx.JSON(w, http.StatusOK, map[string]any{"tasks": list})
	}
}

func getJSON(q *db.Queries) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, err := httpx.PathID(r, "id")
		if err != nil {
			httpx.Error(w, r, err)
			return
		}
		t, err := Get(r.Context(), q, id)
		if err != nil {
			httpx.Error(w, r, err)
			return
		}
		httpx.JSON(w, http.StatusOK, t)
	}
}

func createJSON(q *db.Queries) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var in Input
		if err := httpx.DecodeJSON(r, &in); err != nil {
			httpx.Error(w, r, err)
			return
		}
		t, err := Create(r.Context(), q, in)
		if err != nil {
			httpx.Error(w, r, err)
			return
		}
		httpx.JSON(w, http.StatusCreated, t)
	}
}

func updateJSON(q *db.Queries) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, err := httpx.PathID(r, "id")
		if err != nil {
			httpx.Error(w, r, err)
			return
		}
		var in Input
		if err := httpx.DecodeJSON(r, &in); err != nil {
			httpx.Error(w, r, err)
			return
		}
		t, err := Update(r.Context(), q, id, in)
		if err != nil {
			httpx.Error(w, r, err)
			return
		}
		httpx.JSON(w, http.StatusOK, t)
	}
}

func deleteJSON(q *db.Queries) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, err := httpx.PathID(r, "id")
		if err != nil {
			httpx.Error(w, r, err)
			return
		}
		if err := Delete(r.Context(), q, id); err != nil {
			httpx.Error(w, r, err)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	}
}

func weekJSON(q *db.Queries) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		currentTime := time.Now()
		id, err := httpx.PathID(r, "id")
		if err != nil {
			httpx.Error(w, r, err)
			return
		}
		var patch weekly.Patch
		if err := httpx.DecodeJSON(r, &patch); err != nil {
			httpx.Error(w, r, err)
			return
		}
		unlocked, err := monthlock.UnlockedSet(r.Context(), q)
		if err != nil {
			httpx.Error(w, r, err)
			return
		}
		cell, err := weekly.Save(r.Context(), q, id, r.PathValue("weekStart"), patch, currentTime, unlocked)
		if err != nil {
			httpx.Error(w, r, err)
			return
		}
		httpx.JSON(w, http.StatusOK, cell)
	}
}

func weekHTML(q *db.Queries) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		currentTime := time.Now()
		id, err := httpx.PathID(r, "id")
		if err != nil {
			httpx.Error(w, r, err)
			return
		}
		planned, err := httpx.FormFloat(r, "planned_hours")
		if err != nil {
			httpx.Error(w, r, err)
			return
		}
		spent, err := httpx.FormFloat(r, "spent_hours")
		if err != nil {
			httpx.Error(w, r, err)
			return
		}
		progress, err := httpx.FormFloat(r, "progress")
		if err != nil {
			httpx.Error(w, r, err)
			return
		}
		unlocked, err := monthlock.UnlockedSet(r.Context(), q)
		if err != nil {
			httpx.Error(w, r, err)
			return
		}
		_, err = weekly.Save(r.Context(), q, id, r.PathValue("weekStart"), weekly.Patch{
			PlannedHours: planned,
			SpentHours:   spent,
			Progress:     progress,
		}, currentTime, unlocked)
		if err != nil {
			renderGrid(w, r, q, errMsg(err), currentTime)
			return
		}
		renderGrid(w, r, q, "", currentTime)
	}
}

func formInput(r *http.Request) (Input, error) {
	projectID, err := httpx.FormInt64Checked(r, "project_id")
	if err != nil {
		return Input{}, err
	}
	subprojectID, err := httpx.FormInt64Checked(r, "subproject_id")
	if err != nil {
		return Input{}, err
	}
	return Input{
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
		pid, err := httpx.FormInt64Checked(r, "project_id")
		if err != nil {
			httpx.Error(w, r, err)
			return
		}
		po, so, err := taskSelects(r, q, pid, nil)
		if err != nil {
			httpx.Error(w, r, err)
			return
		}
		vals := views.TaskFormValues{}
		if pid != nil {
			vals.ProjectID = strconv.FormatInt(*pid, 10)
		}
		httpx.HTML(w, http.StatusOK)
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
		id, err := httpx.PathID(r, "id")
		if err != nil {
			httpx.Error(w, r, err)
			return
		}
		t, err := Get(r.Context(), q, id)
		if err != nil {
			httpx.Error(w, r, err)
			return
		}
		httpx.HTML(w, http.StatusOK)
		data, err := taskFormData(r, q, t, "")
		if err != nil {
			httpx.Error(w, r, err)
			return
		}
		_ = views.TaskForm(data).Render(r.Context(), w)
	}
}

func taskFormData(r *http.Request, q *db.Queries, t Task, errMsg string) (views.TaskFormData, error) {
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
			httpx.Error(w, r, err)
			return
		}
		t, err := Create(r.Context(), q, in)
		if err != nil {
			dummy := Task{
				Name: in.Name, Description: in.Description, ImplementationNotes: in.ImplementationNotes,
				Department: in.Department, Developers: in.Developers, Priority: in.Priority,
				ProjectID: in.ProjectID, SubprojectID: in.SubprojectID,
			}
			httpx.HTML(w, http.StatusBadRequest)
			data, formErr := taskFormData(r, q, dummy, err.Error())
			if formErr != nil {
				httpx.Error(w, r, formErr)
				return
			}
			data.Action = "/tasks"
			data.Title = "New task"
			data.DeleteAction = ""
			_ = views.TaskForm(data).Render(r.Context(), w)
			return
		}
		httpx.Redirect(w, r, afterTask(t))
	}
}

func updateHTML(q *db.Queries) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, err := httpx.PathID(r, "id")
		if err != nil {
			httpx.Error(w, r, err)
			return
		}
		in, err := formInput(r)
		if err != nil {
			httpx.Error(w, r, err)
			return
		}
		t, err := Update(r.Context(), q, id, in)
		if err != nil {
			cur := Task{ID: id, Name: in.Name, Description: in.Description, ImplementationNotes: in.ImplementationNotes,
				Department: in.Department, Developers: in.Developers, Priority: in.Priority,
				ProjectID: in.ProjectID, SubprojectID: in.SubprojectID}
			httpx.HTML(w, http.StatusBadRequest)
			data, formErr := taskFormData(r, q, cur, err.Error())
			if formErr != nil {
				httpx.Error(w, r, formErr)
				return
			}
			_ = views.TaskForm(data).Render(r.Context(), w)
			return
		}
		httpx.Redirect(w, r, afterTask(t))
	}
}

func deleteHTML(q *db.Queries) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, err := httpx.PathID(r, "id")
		if err != nil {
			httpx.Error(w, r, err)
			return
		}
		if err := Delete(r.Context(), q, id); err != nil {
			httpx.Error(w, r, err)
			return
		}
		httpx.Redirect(w, r, "/")
	}
}

func afterTask(t Task) string {
	if t.ProjectID == nil {
		return "/?project=ideas"
	}
	u := "/?project=" + strconv.FormatInt(*t.ProjectID, 10)
	if t.SubprojectID != nil {
		u += "&subproject=" + strconv.FormatInt(*t.SubprojectID, 10)
	}
	return u
}

func panel(q *db.Queries) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, err := httpx.PathID(r, "id")
		if err != nil {
			httpx.Error(w, r, err)
			return
		}
		t, err := Get(r.Context(), q, id)
		if err != nil {
			httpx.Error(w, r, err)
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
				httpx.Error(w, r, err)
				return
			}
			row.ProjectName = p.Name
		}
		if t.SubprojectID != nil {
			s, err := subproject.Get(r.Context(), q, *t.SubprojectID)
			if err != nil {
				httpx.Error(w, r, err)
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
		httpx.HTML(w, http.StatusOK)
		if httpx.IsHTMX(r) {
			_ = views.TaskPanel(data).Render(r.Context(), w)
			return
		}
		_ = views.TaskPanelPage(data).Render(r.Context(), w)
	}
}

func toggleLastMonth(q *db.Queries) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		currentTime := time.Now()
		unlocked, err := monthlock.UnlockedSet(r.Context(), q)
		if err != nil {
			httpx.Error(w, r, err)
			return
		}
		last := monthlock.PreviousMonth(currentTime)
		if err := monthlock.Set(r.Context(), q, last, !unlocked[last], currentTime); err != nil {
			renderGrid(w, r, q, errMsg(err), currentTime)
			return
		}
		renderGrid(w, r, q, "", currentTime)
	}
}

func unlockMonth(q *db.Queries) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		currentTime := time.Now()
		if err := monthlock.Set(r.Context(), q, r.FormValue("year_month"), true, currentTime); err != nil {
			renderGrid(w, r, q, errMsg(err), currentTime)
			return
		}
		renderGrid(w, r, q, "", currentTime)
	}
}

func errMsg(err error) string {
	var ae apperr.Error
	if errors.As(err, &ae) {
		return ae.Message
	}
	return err.Error()
}
