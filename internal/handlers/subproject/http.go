package subprojecthandler

import (
	"context"
	"net/http"
	"strconv"

	"cad-development/internal/app"
	"cad-development/internal/db"
	"cad-development/internal/project"
	domain "cad-development/internal/subproject"
	task "cad-development/internal/task"
	"cad-development/internal/views"
)

func Register(mux *http.ServeMux, q *db.Queries) {
	mux.HandleFunc("GET /api/v1/subprojects", app.JSONList(q, listSubprojects))
	mux.HandleFunc("POST /api/v1/subprojects", app.JSONCreate(q, domain.Create))
	mux.HandleFunc("GET /api/v1/subprojects/{id}", app.JSONGet(q, domain.Get))
	mux.HandleFunc("PUT /api/v1/subprojects/{id}", app.JSONUpdate(q, domain.Update))
	mux.HandleFunc("DELETE /api/v1/subprojects/{id}", app.JSONDelete(q, domain.Delete))
	mux.HandleFunc("GET /subprojects/new", newForm(q))
	mux.HandleFunc("POST /subprojects", createHTML(q))
	mux.HandleFunc("GET /subprojects/{id}/edit", editForm(q))
	mux.HandleFunc("POST /subprojects/{id}", updateHTML(q))
	mux.HandleFunc("POST /subprojects/{id}/delete", deleteHTML(q))
}

func listSubprojects(ctx context.Context, q *db.Queries, r *http.Request) (domain.SubprojectsResponse, error) {
	pid, err := app.FormInt64Checked(r, "project_id")
	if err != nil {
		return domain.SubprojectsResponse{}, err
	}
	var list []domain.Subproject
	if pid != nil {
		list, err = domain.ListByProjectWithTotals(ctx, q, *pid)
	} else {
		list, err = domain.ListWithTotals(ctx, q)
	}
	if err != nil {
		return domain.SubprojectsResponse{}, err
	}
	return domain.SubprojectsResponse{Subprojects: list}, nil
}

func formInput(r *http.Request) (domain.Input, error) {
	hours, err := app.FormFloatRequired(r, "total_hours")
	if err != nil {
		return domain.Input{}, err
	}
	in := domain.Input{
		Name: r.FormValue("name"),
	}
	if pid, err := app.FormInt64Checked(r, "project_id"); err != nil {
		return domain.Input{}, err
	} else if pid != nil {
		in.ProjectID = *pid
	}
	in.TotalHours = &hours
	return in, nil
}

func projectOpts(ctxq *db.Queries, r *http.Request, selected int64) ([]task.Option, error) {
	projects, err := project.List(r.Context(), ctxq)
	if err != nil {
		return nil, err
	}
	var out []task.Option
	for _, p := range projects {
		v := strconv.FormatInt(p.ID, 10)
		out = append(out, task.Option{Value: v, Label: p.Name, Selected: p.ID == selected})
	}
	return out, nil
}

func renderSubprojectForm(w http.ResponseWriter, r *http.Request, q *db.Queries, vals views.SubprojectFormValues, action, title, deleteAction string, err error) {
	httpErr := app.HTTPErrorFrom(err)
	selected, _ := strconv.ParseInt(vals.ProjectID, 10, 64)
	opts, optsErr := projectOpts(q, r, selected)
	if optsErr != nil {
		app.WriteError(w, r, optsErr)
		return
	}
	app.RenderPage(w, r, httpErr.Status, views.SubprojectForm(views.SubprojectFormData{
		Action: action, Title: title, Subproject: vals, Projects: opts, Error: httpErr.Message, DeleteAction: deleteAction,
	}))
}

func newForm(q *db.Queries) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		pid, err := app.FormInt64Checked(r, "project_id")
		if err != nil {
			app.WriteError(w, r, err)
			return
		}
		selected := int64(0)
		if pid != nil {
			selected = *pid
		}
		opts, err := projectOpts(q, r, selected)
		if err != nil {
			app.WriteError(w, r, err)
			return
		}
		app.RenderPage(w, r, http.StatusOK, views.SubprojectForm(views.SubprojectFormData{
			Action:   "/subprojects",
			Title:    "New subproject",
			Projects: opts,
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
		s, err := domain.Get(r.Context(), q, id)
		if err != nil {
			app.WriteError(w, r, err)
			return
		}
		opts, err := projectOpts(q, r, s.ProjectID)
		if err != nil {
			app.WriteError(w, r, err)
			return
		}
		app.RenderPage(w, r, http.StatusOK, views.SubprojectForm(subFormData(s, opts, "")))
	}
}

func subFormData(s domain.Subproject, opts []task.Option, errMsg string) views.SubprojectFormData {
	id := strconv.FormatInt(s.ID, 10)
	return views.SubprojectFormData{
		Action: "/subprojects/" + id,
		Title:  "Edit subproject",
		Subproject: views.SubprojectFormValues{
			Name:       s.Name,
			ProjectID:  strconv.FormatInt(s.ProjectID, 10),
			TotalHours: strconv.FormatFloat(s.TotalHours, 'f', -1, 64),
		},
		Projects:     opts,
		Error:        errMsg,
		DeleteAction: "/subprojects/" + id + "/delete",
	}
}

func submittedFormValues(r *http.Request) views.SubprojectFormValues {
	return views.SubprojectFormValues{Name: r.FormValue("name"), ProjectID: r.FormValue("project_id"), TotalHours: r.FormValue("total_hours")}
}

func createHTML(q *db.Queries) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		in, err := formInput(r)
		if err != nil {
			renderSubprojectForm(w, r, q, submittedFormValues(r), "/subprojects", "New subproject", "", err)
			return
		}
		s, err := domain.Create(r.Context(), q, in)
		if err != nil {
			renderSubprojectForm(w, r, q, submittedFormValues(r), "/subprojects", "New subproject", "", err)
			return
		}
		app.Redirect(w, r, "/?project="+strconv.FormatInt(s.ProjectID, 10)+"&subproject="+strconv.FormatInt(s.ID, 10))
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
			renderSubprojectForm(w, r, q, submittedFormValues(r), "/subprojects/"+strconv.FormatInt(id, 10), "Edit subproject", "/subprojects/"+strconv.FormatInt(id, 10)+"/delete", err)
			return
		}
		s, err := domain.Update(r.Context(), q, id, in)
		if err != nil {
			renderSubprojectForm(w, r, q, submittedFormValues(r), "/subprojects/"+strconv.FormatInt(id, 10), "Edit subproject", "/subprojects/"+strconv.FormatInt(id, 10)+"/delete", err)
			return
		}
		app.Redirect(w, r, "/?project="+strconv.FormatInt(s.ProjectID, 10)+"&subproject="+strconv.FormatInt(s.ID, 10))
	}
}

func deleteHTML(q *db.Queries) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, err := app.PathID(r, "id")
		if err != nil {
			app.WriteError(w, r, err)
			return
		}
		s, err := domain.Get(r.Context(), q, id)
		if err != nil {
			app.WriteError(w, r, err)
			return
		}
		if err := domain.Delete(r.Context(), q, id); err != nil {
			app.WriteError(w, r, err)
			return
		}
		app.Redirect(w, r, "/?project="+strconv.FormatInt(s.ProjectID, 10))
	}
}
