package subprojecthandler

import (
	"net/http"
	"strconv"

	"cad-development/internal/app"
	"cad-development/internal/db"
	handlererrors "cad-development/internal/handlers/errors"
	"cad-development/internal/project"
	domain "cad-development/internal/subproject"
	"cad-development/internal/views"
)

func Register(mux *http.ServeMux, q *db.Queries) {
	mux.HandleFunc("GET /api/v1/subprojects", listJSON(q))
	mux.HandleFunc("POST /api/v1/subprojects", createJSON(q))
	mux.HandleFunc("GET /api/v1/subprojects/{id}", getJSON(q))
	mux.HandleFunc("PUT /api/v1/subprojects/{id}", updateJSON(q))
	mux.HandleFunc("DELETE /api/v1/subprojects/{id}", deleteJSON(q))
	mux.HandleFunc("GET /subprojects/new", newForm(q))
	mux.HandleFunc("POST /subprojects", createHTML(q))
	mux.HandleFunc("GET /subprojects/{id}/edit", editForm(q))
	mux.HandleFunc("POST /subprojects/{id}", updateHTML(q))
	mux.HandleFunc("POST /subprojects/{id}/delete", deleteHTML(q))
}

func listJSON(q *db.Queries) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var (
			list []domain.Subproject
			err  error
		)
		pid, err := app.FormInt64Checked(r, "project_id")
		if err != nil {
			handlererrors.WriteError(w, r, err)
			return
		}
		if pid != nil {
			list, err = domain.ListByProjectWithTotals(r.Context(), q, *pid)
		} else {
			list, err = domain.ListWithTotals(r.Context(), q)
		}
		if err != nil {
			handlererrors.WriteError(w, r, err)
			return
		}
		app.JSON(w, http.StatusOK, domain.SubprojectsResponse{Subprojects: list})
	}
}

func getJSON(q *db.Queries) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, err := app.PathID(r, "id")
		if err != nil {
			handlererrors.WriteError(w, r, err)
			return
		}
		s, err := domain.Get(r.Context(), q, id)
		if err != nil {
			handlererrors.WriteError(w, r, err)
			return
		}
		app.JSON(w, http.StatusOK, s)
	}
}

func createJSON(q *db.Queries) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var in domain.Input
		if err := app.DecodeJSON(w, r, &in); err != nil {
			handlererrors.WriteError(w, r, err)
			return
		}
		s, err := domain.Create(r.Context(), q, in)
		if err != nil {
			handlererrors.WriteError(w, r, err)
			return
		}
		s, err = domain.Get(r.Context(), q, s.ID)
		if err != nil {
			handlererrors.WriteError(w, r, err)
			return
		}
		app.JSON(w, http.StatusCreated, s)
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
		if err := app.DecodeJSON(w, r, &in); err != nil {
			handlererrors.WriteError(w, r, err)
			return
		}
		s, err := domain.Update(r.Context(), q, id, in)
		if err != nil {
			handlererrors.WriteError(w, r, err)
			return
		}
		s, err = domain.Get(r.Context(), q, s.ID)
		if err != nil {
			handlererrors.WriteError(w, r, err)
			return
		}
		app.JSON(w, http.StatusOK, s)
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
		Name: r.FormValue("name"),
	}
	if pid, err := app.FormInt64Checked(r, "project_id"); err != nil {
		return domain.Input{}, err
	} else if pid != nil {
		in.ProjectID = *pid
	}
	in.TotalHours = hours
	return in, nil
}

func projectOpts(ctxq *db.Queries, r *http.Request, selected int64) ([]views.Option, error) {
	projects, err := project.List(r.Context(), ctxq)
	if err != nil {
		return nil, err
	}
	var out []views.Option
	for _, p := range projects {
		v := strconv.FormatInt(p.ID, 10)
		out = append(out, views.Option{Value: v, Label: p.Name, Selected: p.ID == selected})
	}
	return out, nil
}

func newForm(q *db.Queries) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		pid, err := app.FormInt64Checked(r, "project_id")
		if err != nil {
			handlererrors.WriteError(w, r, err)
			return
		}
		selected := int64(0)
		if pid != nil {
			selected = *pid
		}
		opts, err := projectOpts(q, r, selected)
		if err != nil {
			handlererrors.WriteError(w, r, err)
			return
		}
		handlererrors.Render(w, r, http.StatusOK, views.SubprojectForm(views.SubprojectFormData{
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
			handlererrors.WriteError(w, r, err)
			return
		}
		s, err := domain.Get(r.Context(), q, id)
		if err != nil {
			handlererrors.WriteError(w, r, err)
			return
		}
		opts, err := projectOpts(q, r, s.ProjectID)
		if err != nil {
			handlererrors.WriteError(w, r, err)
			return
		}
		handlererrors.Render(w, r, http.StatusOK, views.SubprojectForm(subFormData(s, opts, "")))
	}
}

func subFormData(s domain.Subproject, opts []views.Option, errMsg string) views.SubprojectFormData {
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
			httpErr := handlererrors.ToHTTPError(err)
			opts, optsErr := projectOpts(q, r, 0)
			if optsErr != nil {
				handlererrors.WriteError(w, r, optsErr)
				return
			}
			handlererrors.Render(w, r, httpErr.Status, views.SubprojectForm(views.SubprojectFormData{Action: "/subprojects", Title: "New subproject", Projects: opts, Subproject: submittedFormValues(r), Error: httpErr.Message}))
			return
		}
		s, err := domain.Create(r.Context(), q, in)
		if err != nil {
			opts, optsErr := projectOpts(q, r, in.ProjectID)
			if optsErr != nil {
				handlererrors.WriteError(w, r, optsErr)
				return
			}
			httpErr := handlererrors.ToHTTPError(err)
			handlererrors.Render(w, r, httpErr.Status, views.SubprojectForm(views.SubprojectFormData{
				Action:   "/subprojects",
				Title:    "New subproject",
				Projects: opts,
				Subproject: views.SubprojectFormValues{
					Name:       in.Name,
					ProjectID:  strconv.FormatInt(in.ProjectID, 10),
					TotalHours: r.FormValue("total_hours"),
				},
				Error: httpErr.Message,
			}))
			return
		}
		app.Redirect(w, r, "/?project="+strconv.FormatInt(s.ProjectID, 10)+"&subproject="+strconv.FormatInt(s.ID, 10))
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
			httpErr := handlererrors.ToHTTPError(err)
			opts, optsErr := projectOpts(q, r, 0)
			if optsErr != nil {
				handlererrors.WriteError(w, r, optsErr)
				return
			}
			handlererrors.Render(w, r, httpErr.Status, views.SubprojectForm(views.SubprojectFormData{Action: "/subprojects/" + strconv.FormatInt(id, 10), Title: "Edit subproject", Projects: opts, Subproject: submittedFormValues(r), Error: httpErr.Message, DeleteAction: "/subprojects/" + strconv.FormatInt(id, 10) + "/delete"}))
			return
		}
		s, err := domain.Update(r.Context(), q, id, in)
		if err != nil {
			opts, optsErr := projectOpts(q, r, in.ProjectID)
			if optsErr != nil {
				handlererrors.WriteError(w, r, optsErr)
				return
			}
			cur := domain.Subproject{ID: id, ProjectID: in.ProjectID, Name: in.Name, TotalHours: in.TotalHours}
			httpErr := handlererrors.ToHTTPError(err)
			handlererrors.Render(w, r, httpErr.Status, views.SubprojectForm(subFormData(cur, opts, httpErr.Message)))
			return
		}
		app.Redirect(w, r, "/?project="+strconv.FormatInt(s.ProjectID, 10))
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
