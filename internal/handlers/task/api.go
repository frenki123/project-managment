package taskhandler

import (
	"net/http"

	"cad-development/internal/app"
	"cad-development/internal/db"
	handlererrors "cad-development/internal/handlers/errors"
	taskdomain "cad-development/internal/task"
)

func listJSON(q *db.Queries) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var (
			list []taskdomain.Task
			err  error
		)
		projectID, projectErr := app.FormInt64Checked(r, "project_id")
		subprojectID, subprojectErr := app.FormInt64Checked(r, "subproject_id")
		if projectErr != nil {
			handlererrors.WriteError(w, r, projectErr)
			return
		}
		if subprojectErr != nil {
			handlererrors.WriteError(w, r, subprojectErr)
			return
		}
		switch {
		case r.URL.Query().Get("ideas") == "true":
			list, err = taskdomain.ListIdeas(r.Context(), q)
		case subprojectID != nil:
			list, err = taskdomain.ListBySubproject(r.Context(), q, *subprojectID)
		case projectID != nil:
			list, err = taskdomain.ListByProject(r.Context(), q, *projectID)
		default:
			list, err = taskdomain.List(r.Context(), q)
		}
		if err != nil {
			handlererrors.WriteError(w, r, err)
			return
		}
		app.JSON(w, http.StatusOK, map[string]any{"tasks": list})
	}
}

func getJSON(q *db.Queries) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, err := app.PathID(r, "id")
		if err != nil {
			handlererrors.WriteError(w, r, err)
			return
		}
		t, err := taskdomain.Get(r.Context(), q, id)
		if err != nil {
			handlererrors.WriteError(w, r, err)
			return
		}
		app.JSON(w, http.StatusOK, t)
	}
}

func createJSON(q *db.Queries) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var in taskdomain.Input
		if err := app.DecodeJSON(r, &in); err != nil {
			handlererrors.WriteError(w, r, err)
			return
		}
		t, err := taskdomain.Create(r.Context(), q, in)
		if err != nil {
			handlererrors.WriteError(w, r, err)
			return
		}
		app.JSON(w, http.StatusCreated, t)
	}
}

func updateJSON(q *db.Queries) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, err := app.PathID(r, "id")
		if err != nil {
			handlererrors.WriteError(w, r, err)
			return
		}
		var in taskdomain.Input
		if err := app.DecodeJSON(r, &in); err != nil {
			handlererrors.WriteError(w, r, err)
			return
		}
		t, err := taskdomain.Update(r.Context(), q, id, in)
		if err != nil {
			handlererrors.WriteError(w, r, err)
			return
		}
		app.JSON(w, http.StatusOK, t)
	}
}

func deleteJSON(q *db.Queries) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, err := app.PathID(r, "id")
		if err != nil {
			handlererrors.WriteError(w, r, err)
			return
		}
		if err := taskdomain.Delete(r.Context(), q, id); err != nil {
			handlererrors.WriteError(w, r, err)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	}
}
