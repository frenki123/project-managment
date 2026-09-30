package web

import (
	"context"
	"net/http"

	"cad-development/internal/db"
)

func JSONList[E any](q *db.Queries, list func(context.Context, *db.Queries, *http.Request) (E, error)) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		v, err := list(r.Context(), q, r)
		if err != nil {
			WriteError(w, r, err)
			return
		}
		JSON(w, http.StatusOK, v)
	}
}

func JSONGet[E any](q *db.Queries, get func(context.Context, *db.Queries, int64) (E, error)) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, err := PathID(r, "id")
		if err != nil {
			WriteError(w, r, err)
			return
		}
		v, err := get(r.Context(), q, id)
		if err != nil {
			WriteError(w, r, err)
			return
		}
		JSON(w, http.StatusOK, v)
	}
}

func JSONCreate[E, I any](q *db.Queries, create func(context.Context, *db.Queries, I) (E, error)) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var in I
		if err := DecodeJSON(w, r, &in); err != nil {
			WriteError(w, r, err)
			return
		}
		v, err := create(r.Context(), q, in)
		if err != nil {
			WriteError(w, r, err)
			return
		}
		JSON(w, http.StatusCreated, v)
	}
}

func JSONUpdate[E, I any](q *db.Queries, update func(context.Context, *db.Queries, int64, I) (E, error)) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, err := PathID(r, "id")
		if err != nil {
			WriteError(w, r, err)
			return
		}
		var in I
		if err := DecodeJSON(w, r, &in); err != nil {
			WriteError(w, r, err)
			return
		}
		v, err := update(r.Context(), q, id, in)
		if err != nil {
			WriteError(w, r, err)
			return
		}
		JSON(w, http.StatusOK, v)
	}
}

func JSONDelete(q *db.Queries, del func(context.Context, *db.Queries, int64) error) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, err := PathID(r, "id")
		if err != nil {
			WriteError(w, r, err)
			return
		}
		if err := del(r.Context(), q, id); err != nil {
			WriteError(w, r, err)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	}
}
