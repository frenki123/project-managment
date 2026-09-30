package subprojecthandler

import (
	"context"
	"database/sql"
	"errors"
	"net/http"
	"strconv"

	"cad-development/internal/db"
	"cad-development/internal/handlers/shared"
	"cad-development/internal/subproject"
	"cad-development/internal/task"
	"cad-development/internal/views"
	"cad-development/internal/web"
)

func Register(mux *http.ServeMux, q *db.Queries) {
	mux.HandleFunc("GET /api/v1/subprojects", web.JSONList(q, listSubprojects))
	mux.HandleFunc("POST /api/v1/subprojects", web.JSONCreate(q, subproject.Create))
	mux.HandleFunc("GET /api/v1/subprojects/{id}", web.JSONGet(q, subproject.Get))
	mux.HandleFunc("PUT /api/v1/subprojects/{id}", web.JSONUpdate(q, subproject.Update))
	mux.HandleFunc("DELETE /api/v1/subprojects/{id}", web.JSONDelete(q, subproject.Delete))
	mux.HandleFunc("GET /subprojects/new", newForm(q))
	mux.HandleFunc("POST /subprojects", createHTML(q))
	mux.HandleFunc("GET /subprojects/{id}/edit", editForm(q))
	mux.HandleFunc("POST /subprojects/{id}", updateHTML(q))
	mux.HandleFunc("POST /subprojects/{id}/delete", deleteHTML(q))
}

func listSubprojects(ctx context.Context, q *db.Queries, r *http.Request) (subproject.SubprojectsResponse, error) {
	pid, err := web.FormInt64Checked(r, "project_id")
	if err != nil {
		return subproject.SubprojectsResponse{}, err
	}
	var list []subproject.Subproject
	if pid != nil {
		if _, err := q.GetProject(ctx, *pid); errors.Is(err, sql.ErrNoRows) {
			return subproject.SubprojectsResponse{}, web.Missing("project not found")
		} else if err != nil {
			return subproject.SubprojectsResponse{}, err
		}
		list, err = subproject.ListByProjectWithTotals(ctx, q, *pid)
	} else {
		list, err = subproject.ListWithTotals(ctx, q)
	}
	if err != nil {
		return subproject.SubprojectsResponse{}, err
	}
	return subproject.SubprojectsResponse{Subprojects: list}, nil
}

func formInput(r *http.Request) (subproject.Input, error) {
	hours, err := web.FormFloatRequired(r, "total_hours")
	if err != nil {
		return subproject.Input{}, err
	}
	vals := submittedFormValues(r)
	in := subproject.Input{Name: vals.Name}
	if pid, err := web.FormInt64Checked(r, "project_id"); err != nil {
		return subproject.Input{}, err
	} else if pid != nil {
		in.ProjectID = *pid
	}
	in.TotalHours = &hours
	return in, nil
}

func submittedFormValues(r *http.Request) views.SubprojectFormValues {
	return views.SubprojectFormValues{Name: r.FormValue("name"), ProjectID: r.FormValue("project_id"), TotalHours: r.FormValue("total_hours")}
}

func renderSubprojectForm(w http.ResponseWriter, r *http.Request, q *db.Queries, vals views.SubprojectFormValues, action, title, deleteAction string, err error) {
	httpErr := web.HTTPErrorFrom(err)
	selected, parseErr := web.Int64Checked(vals.ProjectID, "project_id")
	if parseErr != nil {
		httpErr = web.HTTPErrorFrom(parseErr)
	}
	opts, optsErr := shared.ProjectOptions(r, q, selected)
	if optsErr != nil {
		web.WriteFragmentError(w, r, optsErr)
		return
	}
	data := views.SubprojectFormData{
		Action: action, Title: title, Context: "Project assignment and budget", Subproject: vals, Projects: opts, Error: httpErr.Message, DeleteAction: deleteAction,
	}
	web.RenderFragment(w, r, httpErr.Status, views.SubprojectForm(data))
}

func newForm(q *db.Queries) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		pid, err := web.FormInt64Checked(r, "project_id")
		if err != nil {
			web.WriteFragmentError(w, r, err)
			return
		}
		opts, err := shared.ProjectOptions(r, q, pid)
		if err != nil {
			web.WriteFragmentError(w, r, err)
			return
		}
		data := views.SubprojectFormData{
			Action:   "/subprojects",
			Title:    "New subproject",
			Context:  "Project assignment and budget",
			Projects: opts,
		}
		web.RenderFragment(w, r, http.StatusOK, views.SubprojectForm(data))
	}
}

func editForm(q *db.Queries) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, ok := shared.PathID(w, r)
		if !ok {
			return
		}
		s, err := subproject.Get(r.Context(), q, id)
		if err != nil {
			web.WriteFragmentError(w, r, err)
			return
		}
		opts, err := shared.ProjectOptions(r, q, &s.ProjectID)
		if err != nil {
			web.WriteFragmentError(w, r, err)
			return
		}
		data := subFormData(s, opts, "")
		web.RenderFragment(w, r, http.StatusOK, views.SubprojectForm(data))
	}
}

func subFormData(s subproject.Subproject, opts []task.Option, errMsg string) views.SubprojectFormData {
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

func createHTML(q *db.Queries) http.HandlerFunc {
	return shared.CreateForm(q, formInput, subproject.Create, func(w http.ResponseWriter, r *http.Request, err error) {
		renderSubprojectForm(w, r, q, submittedFormValues(r), "/subprojects", "New subproject", "", err)
	}, func(s subproject.Subproject) string {
		return "/?" + task.FilterKey(task.Filter{ID: s.ProjectID, Subproject: &s.ID})
	})
}

func updateHTML(q *db.Queries) http.HandlerFunc {
	return shared.UpdateForm(q, formInput, func(ctx context.Context, q *db.Queries, id int64, in subproject.Input) (subproject.Subproject, error) {
		return subproject.Update(ctx, q, id, subproject.PatchFromInput(in))
	}, func(w http.ResponseWriter, r *http.Request, id int64, err error) {
		renderSubprojectForm(w, r, q, submittedFormValues(r), "/subprojects/"+strconv.FormatInt(id, 10), "Edit subproject", "/subprojects/"+strconv.FormatInt(id, 10)+"/delete", err)
	}, func(s subproject.Subproject) string {
		return "/?" + task.FilterKey(task.Filter{ID: s.ProjectID, Subproject: &s.ID})
	})
}

func deleteHTML(q *db.Queries) http.HandlerFunc {
	return shared.DeleteForm(q, subproject.Get, subproject.Delete, func(w http.ResponseWriter, r *http.Request, s subproject.Subproject, err error) {
		httpErr := web.HTTPErrorFrom(err)
		web.SetToast(w, httpErr.Message)
		idStr := strconv.FormatInt(s.ID, 10)
		renderSubprojectForm(w, r, q, views.SubprojectFormValues{
			Name:       s.Name,
			ProjectID:  strconv.FormatInt(s.ProjectID, 10),
			TotalHours: strconv.FormatFloat(s.TotalHours, 'f', -1, 64),
		}, "/subprojects/"+idStr, "Edit subproject", "/subprojects/"+idStr+"/delete", httpErr)
	}, func(s subproject.Subproject) string {
		return "/?" + task.FilterKey(task.Filter{ID: s.ProjectID})
	})
}
