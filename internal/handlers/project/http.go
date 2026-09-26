package projecthandler

import (
	"net/http"
	"strconv"

	"cad-development/internal/app"
	"cad-development/internal/db"
	handlererrors "cad-development/internal/handlers/errors"
	domain "cad-development/internal/project"
	"cad-development/internal/views"
)

func Register(mux *http.ServeMux, q *db.Queries) {
	mux.HandleFunc("GET /api/v1/projects", listJSON(q))
	mux.HandleFunc("POST /api/v1/projects", createJSON(q))
	mux.HandleFunc("GET /api/v1/projects/{id}", getJSON(q))
	mux.HandleFunc("PUT /api/v1/projects/{id}", updateJSON(q))
	mux.HandleFunc("DELETE /api/v1/projects/{id}", deleteJSON(q))
	mux.HandleFunc("GET /projects/new", newForm())
	mux.HandleFunc("POST /projects", createHTML(q))
	mux.HandleFunc("GET /projects/{id}/edit", editForm(q))
	mux.HandleFunc("POST /projects/{id}", updateHTML(q))
	mux.HandleFunc("POST /projects/{id}/delete", deleteHTML(q))
}

func listJSON(q *db.Queries) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		list, err := domain.List(r.Context(), q)
		if err != nil {
			handlererrors.WriteError(w, r, err)
			return
		}
		app.JSON(w, http.StatusOK, map[string]any{"projects": list})
	}
}

func getJSON(q *db.Queries) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, err := app.PathID(r, "id")
		if err != nil {
			handlererrors.WriteError(w, r, err)
			return
		}
		p, err := domain.Get(r.Context(), q, id)
		if err != nil {
			handlererrors.WriteError(w, r, err)
			return
		}
		app.JSON(w, http.StatusOK, p)
	}
}

func createJSON(q *db.Queries) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var in domain.Input
		if err := app.DecodeJSON(r, &in); err != nil {
			handlererrors.WriteError(w, r, err)
			return
		}
		p, err := domain.Create(r.Context(), q, in)
		if err != nil {
			handlererrors.WriteError(w, r, err)
			return
		}
		app.JSON(w, http.StatusCreated, p)
	}
}

func updateJSON(q *db.Queries) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, err := app.PathID(r, "id")
		if err != nil {
			handlererrors.WriteError(w, r, err)
			return
		}
		var in domain.Input
		if err := app.DecodeJSON(r, &in); err != nil {
			handlererrors.WriteError(w, r, err)
			return
		}
		p, err := domain.Update(r.Context(), q, id, in)
		if err != nil {
			handlererrors.WriteError(w, r, err)
			return
		}
		app.JSON(w, http.StatusOK, p)
	}
}

func deleteJSON(q *db.Queries) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, err := app.PathID(r, "id")
		if err != nil {
			handlererrors.WriteError(w, r, err)
			return
		}
		if err := domain.Delete(r.Context(), q, id); err != nil {
			handlererrors.WriteError(w, r, err)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	}
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
	in.TotalHours = hours
	return in, nil
}

func newForm() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		app.HTML(w, http.StatusOK)
		_ = views.ProjectForm(views.ProjectFormData{
			Action: "/projects",
			Title:  "New project",
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
		p, err := domain.Get(r.Context(), q, id)
		if err != nil {
			handlererrors.WriteError(w, r, err)
			return
		}
		app.HTML(w, http.StatusOK)
		_ = views.ProjectForm(projectFormData(p, "")).Render(r.Context(), w)
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
			handlererrors.WriteError(w, r, err)
			return
		}
		p, err := domain.Create(r.Context(), q, in)
		if err != nil {
			app.HTML(w, http.StatusBadRequest)
			_ = views.ProjectForm(views.ProjectFormData{
				Action: "/projects",
				Title:  "New project",
				Project: views.ProjectFormValues{
					Name:              in.Name,
					PurchaseOrderName: in.PurchaseOrderName,
					TotalHours:        r.FormValue("total_hours"),
					StartDate:         in.StartDate,
					EndDate:           in.EndDate,
				},
				Error: err.Error(),
			}).Render(r.Context(), w)
			return
		}
		app.Redirect(w, r, "/?project="+strconv.FormatInt(p.ID, 10))
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
		p, err := domain.Update(r.Context(), q, id, in)
		if err != nil {
			app.HTML(w, http.StatusBadRequest)
			data := views.ProjectFormData{
				Action: "/projects/" + strconv.FormatInt(id, 10),
				Title:  "Edit project",
				Project: views.ProjectFormValues{
					Name: in.Name, PurchaseOrderName: in.PurchaseOrderName,
					TotalHours: strconv.FormatFloat(in.TotalHours, 'f', -1, 64),
					StartDate:  in.StartDate, EndDate: in.EndDate,
				},
				Error: err.Error(), DeleteAction: "/projects/" + strconv.FormatInt(id, 10) + "/delete",
			}
			_ = views.ProjectForm(data).Render(r.Context(), w)
			return
		}
		app.Redirect(w, r, "/?project="+strconv.FormatInt(p.ID, 10))
	}
}

func deleteHTML(q *db.Queries) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, err := app.PathID(r, "id")
		if err != nil {
			handlererrors.WriteError(w, r, err)
			return
		}
		if err := domain.Delete(r.Context(), q, id); err != nil {
			handlererrors.WriteError(w, r, err)
			return
		}
		app.Redirect(w, r, "/")
	}
}
