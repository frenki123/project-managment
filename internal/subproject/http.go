package subproject

import (
	"net/http"
	"strconv"

	"cad-development/internal/db"
	"cad-development/internal/httpx"
	"cad-development/internal/project"
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
			list []Subproject
			err  error
		)
		if pid, err := httpx.FormInt64Checked(r, "project_id"); err != nil {
			httpx.Error(w, r, err)
			return
		} else if pid != nil {
			list, err = ListByProject(r.Context(), q, *pid)
		} else {
			list, err = List(r.Context(), q)
		}
		if err != nil {
			httpx.Error(w, r, err)
			return
		}
		httpx.JSON(w, http.StatusOK, map[string]any{"subprojects": list})
	}
}

func getJSON(q *db.Queries) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, err := httpx.PathID(r, "id")
		if err != nil {
			httpx.Error(w, r, err)
			return
		}
		s, err := Get(r.Context(), q, id)
		if err != nil {
			httpx.Error(w, r, err)
			return
		}
		httpx.JSON(w, http.StatusOK, s)
	}
}

func createJSON(q *db.Queries) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var in Input
		if err := httpx.DecodeJSON(r, &in); err != nil {
			httpx.Error(w, r, err)
			return
		}
		s, err := Create(r.Context(), q, in)
		if err != nil {
			httpx.Error(w, r, err)
			return
		}
		httpx.JSON(w, http.StatusCreated, s)
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
		s, err := Update(r.Context(), q, id, in)
		if err != nil {
			httpx.Error(w, r, err)
			return
		}
		httpx.JSON(w, http.StatusOK, s)
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

func formInput(r *http.Request) (Input, error) {
	hours, err := httpx.FormFloatRequired(r, "total_hours")
	if err != nil {
		return Input{}, err
	}
	in := Input{
		Name: r.FormValue("name"),
	}
	if pid, err := httpx.FormInt64Checked(r, "project_id"); err != nil {
		return Input{}, err
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
		opts, err := projectOpts(q, r, 0)
		if err != nil {
			httpx.Error(w, r, err)
			return
		}
		httpx.HTML(w, http.StatusOK)
		_ = views.SubprojectForm(views.SubprojectFormData{
			Action:   "/subprojects",
			Title:    "New subproject",
			Projects: opts,
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
		s, err := Get(r.Context(), q, id)
		if err != nil {
			httpx.Error(w, r, err)
			return
		}
		opts, err := projectOpts(q, r, s.ProjectID)
		if err != nil {
			httpx.Error(w, r, err)
			return
		}
		httpx.HTML(w, http.StatusOK)
		_ = views.SubprojectForm(subFormData(s, opts, "")).Render(r.Context(), w)
	}
}

func subFormData(s Subproject, opts []views.Option, errMsg string) views.SubprojectFormData {
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

func createHTML(q *db.Queries) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		in, err := formInput(r)
		if err != nil {
			httpx.Error(w, r, err)
			return
		}
		s, err := Create(r.Context(), q, in)
		if err != nil {
			opts, optsErr := projectOpts(q, r, in.ProjectID)
			if optsErr != nil {
				httpx.Error(w, r, optsErr)
				return
			}
			httpx.HTML(w, http.StatusBadRequest)
			_ = views.SubprojectForm(views.SubprojectFormData{
				Action:   "/subprojects",
				Title:    "New subproject",
				Projects: opts,
				Subproject: views.SubprojectFormValues{
					Name:       in.Name,
					ProjectID:  strconv.FormatInt(in.ProjectID, 10),
					TotalHours: r.FormValue("total_hours"),
				},
				Error: err.Error(),
			}).Render(r.Context(), w)
			return
		}
		httpx.Redirect(w, r, "/?project="+strconv.FormatInt(s.ProjectID, 10)+"&subproject="+strconv.FormatInt(s.ID, 10))
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
		s, err := Update(r.Context(), q, id, in)
		if err != nil {
			opts, optsErr := projectOpts(q, r, in.ProjectID)
			if optsErr != nil {
				httpx.Error(w, r, optsErr)
				return
			}
			cur := Subproject{ID: id, ProjectID: in.ProjectID, Name: in.Name, TotalHours: in.TotalHours}
			httpx.HTML(w, http.StatusBadRequest)
			_ = views.SubprojectForm(subFormData(cur, opts, err.Error())).Render(r.Context(), w)
			return
		}
		httpx.Redirect(w, r, "/?project="+strconv.FormatInt(s.ProjectID, 10))
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
