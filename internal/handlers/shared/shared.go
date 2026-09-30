package shared

import (
	"context"
	"net/http"
	"strconv"

	"cad-development/internal/db"
	"cad-development/internal/project"
	"cad-development/internal/task"
	"cad-development/internal/web"
)

// PathID parses the {id} path value, writing the error response when invalid.
func PathID(w http.ResponseWriter, r *http.Request) (int64, bool) {
	id, err := web.PathID(r, "id")
	if err != nil {
		web.WriteFragmentError(w, r, err)
		return 0, false
	}
	return id, true
}

// ProjectOptions lists all projects for a form select.
func ProjectOptions(r *http.Request, q *db.Queries, selected *int64) ([]task.Option, error) {
	projects, err := project.List(r.Context(), q)
	if err != nil {
		return nil, err
	}
	out := make([]task.Option, 0, len(projects))
	for _, p := range projects {
		out = append(out, task.Option{Value: strconv.FormatInt(p.ID, 10), Label: p.Name, Selected: selected != nil && *selected == p.ID})
	}
	return out, nil
}

// CreateForm wires the shared create-form handler flow: parse, create, redirect.
func CreateForm[I, E any](q *db.Queries, parse func(*http.Request) (I, error), create func(context.Context, *db.Queries, I) (E, error), renderError func(http.ResponseWriter, *http.Request, error), redirect func(E) string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		in, err := parse(r)
		if err != nil {
			renderError(w, r, err)
			return
		}
		entity, err := create(r.Context(), q, in)
		if err != nil {
			renderError(w, r, err)
			return
		}
		web.Redirect(w, r, redirect(entity))
	}
}

// UpdateForm wires the shared update-form handler flow: parse, update, redirect.
func UpdateForm[I, E any](q *db.Queries, parse func(*http.Request) (I, error), update func(context.Context, *db.Queries, int64, I) (E, error), renderError func(http.ResponseWriter, *http.Request, int64, error), redirect func(E) string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, ok := PathID(w, r)
		if !ok {
			return
		}
		in, err := parse(r)
		if err != nil {
			renderError(w, r, id, err)
			return
		}
		entity, err := update(r.Context(), q, id, in)
		if err != nil {
			renderError(w, r, id, err)
			return
		}
		web.Redirect(w, r, redirect(entity))
	}
}

// DeleteForm wires the shared delete-form handler flow: get, delete, redirect.
func DeleteForm[E any](q *db.Queries, get func(context.Context, *db.Queries, int64) (E, error), del func(context.Context, *db.Queries, int64) error, onError func(http.ResponseWriter, *http.Request, E, error), redirect func(E) string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, ok := PathID(w, r)
		if !ok {
			return
		}
		entity, err := get(r.Context(), q, id)
		if err != nil {
			web.WriteFragmentError(w, r, err)
			return
		}
		if err := del(r.Context(), q, id); err != nil {
			onError(w, r, entity, err)
			return
		}
		web.Redirect(w, r, redirect(entity))
	}
}
