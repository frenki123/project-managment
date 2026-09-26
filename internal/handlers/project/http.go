package projecthandler

import (
	"context"
	"net/http"
	"strconv"

	"cad-development/internal/app"
	"cad-development/internal/db"
	domain "cad-development/internal/project"
	"cad-development/internal/views"
)

func Register(mux *http.ServeMux, q *db.Queries) {
	mux.HandleFunc("GET /api/v1/projects", app.JSONList(q, listProjects))
	mux.HandleFunc("POST /api/v1/projects", app.JSONCreate(q, domain.Create))
	mux.HandleFunc("GET /api/v1/projects/{id}", app.JSONGet(q, domain.Get))
	mux.HandleFunc("PUT /api/v1/projects/{id}", app.JSONUpdate(q, domain.Update))
	mux.HandleFunc("DELETE /api/v1/projects/{id}", app.JSONDelete(q, domain.Delete))
	mux.HandleFunc("GET /projects/new", newForm())
	mux.HandleFunc("POST /projects", createHTML(q))
	mux.HandleFunc("GET /projects/{id}/edit", editForm(q))
	mux.HandleFunc("POST /projects/{id}", updateHTML(q))
	mux.HandleFunc("POST /projects/{id}/delete", deleteHTML(q))
}

func listProjects(ctx context.Context, q *db.Queries, _ *http.Request) (domain.ProjectsResponse, error) {
	list, err := domain.ListWithTotals(ctx, q)
	if err != nil {
		return domain.ProjectsResponse{}, err
	}
	return domain.ProjectsResponse{Projects: list}, nil
}

func formInput(r *http.Request) (domain.Input, error) {
	hours, err := app.FormFloatRequired(r, "total_hours")
	if err != nil {
		return domain.Input{}, err
	}
	in := domain.Input{
		Name:              r.FormValue("name"),
		PurchaseOrderName: r.FormValue("purchase_order_name"),
		StartDate:         r.FormValue("start_date"),
		EndDate:           r.FormValue("end_date"),
	}
	in.TotalHours = &hours
	return in, nil
}

func submittedFormValues(r *http.Request) views.ProjectFormValues {
	return views.ProjectFormValues{Name: r.FormValue("name"), PurchaseOrderName: r.FormValue("purchase_order_name"), TotalHours: r.FormValue("total_hours"), StartDate: r.FormValue("start_date"), EndDate: r.FormValue("end_date")}
}

func renderProjectForm(w http.ResponseWriter, r *http.Request, vals views.ProjectFormValues, action, title, deleteAction string, err error) {
	httpErr := app.HTTPErrorFrom(err)
	app.RenderPage(w, r, httpErr.Status, views.ProjectForm(views.ProjectFormData{
		Action: action, Title: title, Project: vals, Error: httpErr.Message, DeleteAction: deleteAction,
	}))
}

func newForm() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		app.RenderPage(w, r, http.StatusOK, views.ProjectForm(views.ProjectFormData{
			Action: "/projects",
			Title:  "New project",
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
		p, err := domain.Get(r.Context(), q, id)
		if err != nil {
			app.WriteError(w, r, err)
			return
		}
		app.RenderPage(w, r, http.StatusOK, views.ProjectForm(projectFormData(p, "")))
	}
}

func projectFormData(p domain.Project, errMsg string) views.ProjectFormData {
	id := strconv.FormatInt(p.ID, 10)
	return views.ProjectFormData{
		Action: "/projects/" + id,
		Title:  "Edit project",
		Project: views.ProjectFormValues{
			Name:              p.Name,
			PurchaseOrderName: p.PurchaseOrderName,
			TotalHours:        strconv.FormatFloat(p.TotalHours, 'f', -1, 64),
			StartDate:         p.StartDate,
			EndDate:           p.EndDate,
		},
		Error:        errMsg,
		DeleteAction: "/projects/" + id + "/delete",
	}
}

func createHTML(q *db.Queries) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		in, err := formInput(r)
		if err != nil {
			renderProjectForm(w, r, submittedFormValues(r), "/projects", "New project", "", err)
			return
		}
		p, err := domain.Create(r.Context(), q, in)
		if err != nil {
			renderProjectForm(w, r, submittedFormValues(r), "/projects", "New project", "", err)
			return
		}
		app.Redirect(w, r, "/?project="+strconv.FormatInt(p.ID, 10))
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
			renderProjectForm(w, r, submittedFormValues(r), "/projects/"+strconv.FormatInt(id, 10), "Edit project", "/projects/"+strconv.FormatInt(id, 10)+"/delete", err)
			return
		}
		p, err := domain.Update(r.Context(), q, id, in)
		if err != nil {
			renderProjectForm(w, r, submittedFormValues(r), "/projects/"+strconv.FormatInt(id, 10), "Edit project", "/projects/"+strconv.FormatInt(id, 10)+"/delete", err)
			return
		}
		app.Redirect(w, r, "/?project="+strconv.FormatInt(p.ID, 10))
	}
}

func deleteHTML(q *db.Queries) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, err := app.PathID(r, "id")
		if err != nil {
			app.WriteError(w, r, err)
			return
		}
		p, err := domain.Get(r.Context(), q, id)
		if err != nil {
			app.WriteError(w, r, err)
			return
		}
		if err := domain.Delete(r.Context(), q, id); err != nil {
			httpErr := app.HTTPErrorFrom(err)
			app.RenderPage(w, r, httpErr.Status, views.ProjectForm(projectFormData(p, httpErr.Message)))
			return
		}
		app.Redirect(w, r, "/")
	}
}
