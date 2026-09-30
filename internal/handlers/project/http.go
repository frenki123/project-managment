package projecthandler

import (
	"context"
	"net/http"
	"strconv"

	"cad-development/internal/db"
	"cad-development/internal/handlers/shared"
	"cad-development/internal/project"
	"cad-development/internal/task"
	"cad-development/internal/views"
	"cad-development/internal/web"
)

func Register(mux *http.ServeMux, q *db.Queries) {
	mux.HandleFunc("GET /api/v1/projects", web.JSONList(q, listProjects))
	mux.HandleFunc("POST /api/v1/projects", web.JSONCreate(q, project.Create))
	mux.HandleFunc("GET /api/v1/projects/{id}", web.JSONGet(q, project.Get))
	mux.HandleFunc("PUT /api/v1/projects/{id}", web.JSONUpdate(q, project.Update))
	mux.HandleFunc("DELETE /api/v1/projects/{id}", web.JSONDelete(q, project.Delete))
	mux.HandleFunc("GET /projects/new", newForm())
	mux.HandleFunc("POST /projects", createHTML(q))
	mux.HandleFunc("GET /projects/{id}/edit", editForm(q))
	mux.HandleFunc("POST /projects/{id}", updateHTML(q))
	mux.HandleFunc("POST /projects/{id}/delete", deleteHTML(q))
}

func listProjects(ctx context.Context, q *db.Queries, _ *http.Request) (project.ProjectsResponse, error) {
	list, err := project.ListWithTotals(ctx, q)
	if err != nil {
		return project.ProjectsResponse{}, err
	}
	return project.ProjectsResponse{Projects: list}, nil
}

func formInput(r *http.Request) (project.Input, error) {
	hours, err := web.FormFloatRequired(r, "total_hours")
	if err != nil {
		return project.Input{}, err
	}
	vals := submittedFormValues(r)
	in := project.Input{Name: vals.Name, PurchaseOrderName: vals.PurchaseOrderName, StartDate: vals.StartDate, EndDate: vals.EndDate}
	in.TotalHours = &hours
	return in, nil
}

func submittedFormValues(r *http.Request) views.ProjectFormValues {
	return views.ProjectFormValues{Name: r.FormValue("name"), PurchaseOrderName: r.FormValue("purchase_order_name"), TotalHours: r.FormValue("total_hours"), StartDate: r.FormValue("start_date"), EndDate: r.FormValue("end_date")}
}

func renderProjectForm(w http.ResponseWriter, r *http.Request, vals views.ProjectFormValues, action, title, deleteAction string, err error) {
	httpErr := web.HTTPErrorFrom(err)
	data := views.ProjectFormData{
		Action: action, Title: title, Context: projectContext(vals), Project: vals, Error: httpErr.Message, DeleteAction: deleteAction,
	}
	web.RenderFragment(w, r, httpErr.Status, views.ProjectForm(data))
}

func newForm() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		data := views.ProjectFormData{
			Action:  "/projects",
			Title:   "New project",
			Context: "Define budget and schedule",
		}
		web.RenderFragment(w, r, http.StatusOK, views.ProjectForm(data))
	}
}

func editForm(q *db.Queries) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, ok := shared.PathID(w, r)
		if !ok {
			return
		}
		p, err := project.Get(r.Context(), q, id)
		if err != nil {
			web.WriteFragmentError(w, r, err)
			return
		}
		data := projectFormData(p, "")
		web.RenderFragment(w, r, http.StatusOK, views.ProjectForm(data))
	}
}

func projectFormData(p project.Project, errMsg string) views.ProjectFormData {
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
	return shared.CreateForm(q, formInput, project.Create, func(w http.ResponseWriter, r *http.Request, err error) {
		renderProjectForm(w, r, submittedFormValues(r), "/projects", "New project", "", err)
	}, func(p project.Project) string {
		return "/?" + task.FilterKey(task.Filter{ID: p.ID})
	})
}

func updateHTML(q *db.Queries) http.HandlerFunc {
	return shared.UpdateForm(q, formInput, func(ctx context.Context, q *db.Queries, id int64, in project.Input) (project.Project, error) {
		return project.Update(ctx, q, id, project.PatchFromInput(in))
	}, func(w http.ResponseWriter, r *http.Request, id int64, err error) {
		renderProjectForm(w, r, submittedFormValues(r), "/projects/"+strconv.FormatInt(id, 10), "Edit project", "/projects/"+strconv.FormatInt(id, 10)+"/delete", err)
	}, func(p project.Project) string {
		return "/?" + task.FilterKey(task.Filter{ID: p.ID})
	})
}

func deleteHTML(q *db.Queries) http.HandlerFunc {
	return shared.DeleteForm(q, project.Get, project.Delete, func(w http.ResponseWriter, r *http.Request, p project.Project, err error) {
		httpErr := web.HTTPErrorFrom(err)
		web.SetToast(w, httpErr.Message)
		web.RenderFragment(w, r, httpErr.Status, views.ProjectForm(projectFormData(p, httpErr.Message)))
	}, func(project.Project) string {
		return "/"
	})
}
