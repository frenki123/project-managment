package taskhandler

import (
	"context"
	"database/sql"
	"errors"
	"net/http"

	"cad-development/internal/app"
	"cad-development/internal/db"
	taskdomain "cad-development/internal/task"
)

func Register(mux *http.ServeMux, q *db.Queries) {
	mux.HandleFunc("GET /{$}", gridPage(q))
	mux.HandleFunc("GET /api/v1/tasks", app.JSONList(q, listTasks))
	mux.HandleFunc("POST /api/v1/tasks", app.JSONCreate(q, taskdomain.Create))
	mux.HandleFunc("GET /api/v1/tasks/{id}", app.JSONGet(q, taskdomain.Get))
	mux.HandleFunc("PUT /api/v1/tasks/{id}", app.JSONUpdate(q, taskdomain.Update))
	mux.HandleFunc("DELETE /api/v1/tasks/{id}", app.JSONDelete(q, taskdomain.Delete))
	mux.HandleFunc("PUT /api/v1/tasks/{id}/weeks/{weekStart}", weekJSON(q))
	mux.HandleFunc("POST /tasks/{id}/weeks/{weekStart}", weekHTML(q))
	mux.HandleFunc("GET /tasks/new", newForm(q))
	mux.HandleFunc("POST /tasks", createHTML(q))
	mux.HandleFunc("GET /tasks/{id}", panel(q))
	mux.HandleFunc("GET /tasks/{id}/edit", editForm(q))
	mux.HandleFunc("POST /tasks/{id}", updateHTML(q))
	mux.HandleFunc("POST /tasks/{id}/delete", deleteHTML(q))
}

func listTasks(ctx context.Context, q *db.Queries, r *http.Request) (taskdomain.TasksResponse, error) {
	projectID, projectErr := app.FormInt64Checked(r, "project_id")
	subprojectID, subprojectErr := app.FormInt64Checked(r, "subproject_id")
	if projectErr != nil {
		return taskdomain.TasksResponse{}, projectErr
	}
	if subprojectErr != nil {
		return taskdomain.TasksResponse{}, subprojectErr
	}
	if projectID != nil {
		if _, err := q.GetProject(ctx, *projectID); errors.Is(err, sql.ErrNoRows) {
			return taskdomain.TasksResponse{}, app.Missing("project not found")
		} else if err != nil {
			return taskdomain.TasksResponse{}, err
		}
	}
	if subprojectID != nil {
		sp, err := q.GetSubproject(ctx, *subprojectID)
		if errors.Is(err, sql.ErrNoRows) {
			return taskdomain.TasksResponse{}, app.Missing("subproject not found")
		}
		if err != nil {
			return taskdomain.TasksResponse{}, err
		}
		if projectID != nil && *projectID != sp.ProjectID {
			return taskdomain.TasksResponse{}, app.Invalid("subproject does not belong to project")
		}
	}
	var (
		list []taskdomain.Task
		err  error
	)
	switch {
	case subprojectID != nil:
		list, err = taskdomain.ListBySubproject(ctx, q, *subprojectID)
	case projectID != nil:
		list, err = taskdomain.ListByProject(ctx, q, *projectID)
	case r.URL.Query().Get("ideas") == "true":
		list, err = taskdomain.ListIdeas(ctx, q)
	default:
		list, err = taskdomain.List(ctx, q)
	}
	if err != nil {
		return taskdomain.TasksResponse{}, err
	}
	return taskdomain.TasksResponse{Tasks: list}, nil
}
