package taskhandler

import (
	"context"
	"net/http"
	"strconv"

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
	projectKey := ""
	if projectID != nil {
		projectKey = strconv.FormatInt(*projectID, 10)
	}
	if projectID == nil && r.URL.Query().Get("ideas") == "true" {
		projectKey = "ideas"
	}
	subprojectKey := ""
	if subprojectID != nil {
		subprojectKey = strconv.FormatInt(*subprojectID, 10)
	}
	filter, err := taskdomain.ParseFilter(projectKey, subprojectKey)
	if err != nil {
		return taskdomain.TasksResponse{}, err
	}
	resolved, err := filter.Resolve(ctx, q)
	if err != nil {
		return taskdomain.TasksResponse{}, err
	}
	list, err := resolved.List(ctx, q)
	if err != nil {
		return taskdomain.TasksResponse{}, err
	}
	return taskdomain.TasksResponse{Tasks: list}, nil
}
