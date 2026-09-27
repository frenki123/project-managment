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
	vals := submittedFormValues(r)
	in := domain.Input{Name: vals.Name, PurchaseOrderName: vals.PurchaseOrderName, StartDate: vals.StartDate, EndDate: vals.EndDate}
	in.TotalHours = &hours
	return in, nil
}

func submittedFormValues(r *http.Request) views.ProjectFormValues {
	return views.ProjectFormValues{Name: r.FormValue("name"), PurchaseOrderName: r.FormValue("purchase_order_name"), TotalHours: r.FormValue("total_hours"), StartDate: r.FormValue("start_date"), EndDate: r.FormValue("end_date")}
}

func renderProjectForm(w http.ResponseWriter, r *http.Request, vals views.ProjectFormValues, action, title, deleteAction string, err error) {
	httpErr := app.HTTPErrorFrom(err)
	data := views.ProjectFormData{
		Action: action, Title: title, Context: projectContext(vals), Project: vals, Error: app.FriendlyFormMessage(httpErr.Message), DeleteAction: deleteAction,
	}
	app.RenderFragment(w, r, httpErr.Status, views.ProjectForm(data))
}

func newForm() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		data := views.ProjectFormData{
			Action:  "/projects",
			Title:   "New project",
			Context: "Define budget and schedule",
		}
		app.RenderFragment(w, r, http.StatusOK, views.ProjectForm(data))
	}
}

func editForm(q *db.Queries) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, err := app.PathID(r, "id")
		if err != nil {
			app.WriteFragmentError(w, r, err)
			return
		}
		p, err := domain.Get(r.Context(), q, id)
		if err != nil {
			app.WriteFragmentError(w, r, err)
			return
		}
		data := projectFormData(p, "")
		app.RenderFragment(w, r, http.StatusOK, views.ProjectForm(data))
	}
}

func projectFormData(p domain.Project, errMsg string) views.ProjectFormData {
	id := strconv.FormatInt(p.ID, 10)
	return views.ProjectFormData{
		Action:  "/projects/" + id,
		Title:   p.Name,
		Context: "PO " + p.PurchaseOrderName + " · " + p.StartDate + " - " + p.EndDate,
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

func projectContext(vals views.ProjectFormValues) string {
	if vals.PurchaseOrderName == "" && vals.StartDate == "" && vals.EndDate == "" {
		return "Define budget and schedule"
	}
	return "PO " + vals.PurchaseOrderName + " · " + vals.StartDate + " - " + vals.EndDate
}

func createHTML(q *db.Queries) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		vals := submittedFormValues(r)
		in, err := formInput(r)
		if err != nil {
			renderProjectForm(w, r, vals, "/projects", "New project", "", err)
			return
		}
		p, err := domain.Create(r.Context(), q, in)
		if err != nil {
			renderProjectForm(w, r, vals, "/projects", "New project", "", err)
			return
		}
		app.Redirect(w, r, "/?project="+strconv.FormatInt(p.ID, 10))
	}
}

func updateHTML(q *db.Queries) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, err := app.PathID(r, "id")
		if err != nil {
			app.WriteFragmentError(w, r, err)
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
			app.WriteFragmentError(w, r, err)
			return
		}
		p, err := domain.Get(r.Context(), q, id)
		if err != nil {
			app.WriteFragmentError(w, r, err)
			return
		}
		if err := domain.Delete(r.Context(), q, id); err != nil {
			httpErr := app.HTTPErrorFrom(err)
			app.SetToast(w, app.FriendlyFormMessage(httpErr.Message))
			app.RenderFragment(w, r, httpErr.Status, views.ProjectForm(projectFormData(p, app.FriendlyFormMessage(httpErr.Message))))
			return
		}
		app.Redirect(w, r, "/")
	}
}
