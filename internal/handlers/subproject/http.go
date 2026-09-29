package subprojecthandler

import (
	"context"
	"database/sql"
	"errors"
	"net/http"
	"strconv"

	"cad-development/internal/app"
	"cad-development/internal/db"
	"cad-development/internal/nullable"
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
		if _, err := q.GetProject(ctx, *pid); errors.Is(err, sql.ErrNoRows) {
			return domain.SubprojectsResponse{}, app.Missing("project not found")
		} else if err != nil {
			return domain.SubprojectsResponse{}, err
		}
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
	vals := submittedFormValues(r)
	in := domain.Input{Name: vals.Name}
	if pid, err := app.FormInt64Checked(r, "project_id"); err != nil {
		return domain.Input{}, err
	} else if pid != nil {
		in.ProjectID = *pid
	}
	in.TotalHours = &hours
	return in, nil
}

func formPatch(r *http.Request) (domain.Patch, error) {
	in, err := formInput(r)
	if err != nil {
		return domain.Patch{}, err
	}
	return domain.Patch{
		ProjectID: *nullable.Set(in.ProjectID), Name: *nullable.Set(in.Name), TotalHours: *nullable.Set(*in.TotalHours),
	}, nil
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
		app.WriteFragmentError(w, r, optsErr)
		return
	}
	data := views.SubprojectFormData{
		Action: action, Title: title, Context: "Project assignment and budget", Subproject: vals, Projects: opts, Error: app.FriendlyFormMessage(httpErr.Message), DeleteAction: deleteAction,
	}
	app.RenderFragment(w, r, httpErr.Status, views.SubprojectForm(data))
}

func newForm(q *db.Queries) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		pid, err := app.FormInt64Checked(r, "project_id")
		if err != nil {
			app.WriteFragmentError(w, r, err)
			return
		}
		selected := int64(0)
		if pid != nil {
			selected = *pid
		}
		opts, err := projectOpts(q, r, selected)
		if err != nil {
			app.WriteFragmentError(w, r, err)
			return
		}
		data := views.SubprojectFormData{
			Action:   "/subprojects",
			Title:    "New subproject",
			Context:  "Project assignment and budget",
			Projects: opts,
		}
		app.RenderFragment(w, r, http.StatusOK, views.SubprojectForm(data))
	}
}

func editForm(q *db.Queries) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, err := app.PathID(r, "id")
		if err != nil {
			app.WriteFragmentError(w, r, err)
			return
		}
		s, err := domain.Get(r.Context(), q, id)
		if err != nil {
			app.WriteFragmentError(w, r, err)
			return
		}
		opts, err := projectOpts(q, r, s.ProjectID)
		if err != nil {
			app.WriteFragmentError(w, r, err)
			return
		}
		data := subFormData(s, opts, "")
		app.RenderFragment(w, r, http.StatusOK, views.SubprojectForm(data))
	}
}

func subFormData(s domain.Subproject, opts []task.Option, errMsg string) views.SubprojectFormData {
	id := strconv.FormatInt(s.ID, 10)
	return views.SubprojectFormData{
		Action:  "/subprojects/" + id,
		Title:   s.Name,
		Context: "Project assignment and budget",
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
		vals := submittedFormValues(r)
		in, err := formInput(r)
		if err != nil {
			renderSubprojectForm(w, r, q, vals, "/subprojects", "New subproject", "", err)
			return
		}
		s, err := domain.Create(r.Context(), q, in)
		if err != nil {
			renderSubprojectForm(w, r, q, vals, "/subprojects", "New subproject", "", err)
			return
		}
		app.Redirect(w, r, "/?project="+strconv.FormatInt(s.ProjectID, 10)+"&subproject="+strconv.FormatInt(s.ID, 10))
	}
}

func updateHTML(q *db.Queries) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, err := app.PathID(r, "id")
		if err != nil {
			app.WriteFragmentError(w, r, err)
			return
		}
		in, err := formPatch(r)
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
			app.WriteFragmentError(w, r, err)
			return
		}
		s, err := domain.Get(r.Context(), q, id)
		if err != nil {
			app.WriteFragmentError(w, r, err)
			return
		}
		if err := domain.Delete(r.Context(), q, id); err != nil {
			idStr := strconv.FormatInt(id, 10)
			httpErr := app.HTTPErrorFrom(err)
			app.SetToast(w, app.FriendlyFormMessage(httpErr.Message))
			renderSubprojectForm(w, r, q, views.SubprojectFormValues{
				Name:       s.Name,
				ProjectID:  strconv.FormatInt(s.ProjectID, 10),
				TotalHours: strconv.FormatFloat(s.TotalHours, 'f', -1, 64),
			}, "/subprojects/"+idStr, "Edit subproject", "/subprojects/"+idStr+"/delete", httpErr)
			return
		}
		app.Redirect(w, r, "/?project="+strconv.FormatInt(s.ProjectID, 10))
	}
}
