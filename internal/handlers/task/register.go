package taskhandler

import (
	"context"
	"net/http"
	"strconv"

	"cad-development/internal/db"
	"cad-development/internal/task"
	"cad-development/internal/web"
)

func Register(mux *http.ServeMux, q *db.Queries) {
	mux.HandleFunc("GET /{$}", gridPage(q))
	mux.HandleFunc("GET /api/v1/tasks", web.JSONList(q, listTasks))
	mux.HandleFunc("POST /api/v1/tasks", web.JSONCreate(q, task.Create))
	mux.HandleFunc("GET /api/v1/tasks/{id}", web.JSONGet(q, task.Get))
	mux.HandleFunc("PUT /api/v1/tasks/{id}", web.JSONUpdate(q, task.Update))
	mux.HandleFunc("DELETE /api/v1/tasks/{id}", web.JSONDelete(q, task.Delete))
	mux.HandleFunc("PUT /api/v1/tasks/{id}/weeks/{weekStart}", weekJSON(q))
	mux.HandleFunc("POST /tasks/{id}/weeks/{weekStart}", weekHTML(q))
	mux.HandleFunc("GET /tasks/new", newForm(q))
	mux.HandleFunc("POST /tasks", createHTML(q))
	mux.HandleFunc("GET /tasks/{id}", panel(q))
	mux.HandleFunc("GET /tasks/{id}/edit", editForm(q))
	mux.HandleFunc("POST /tasks/{id}", updateHTML(q))
	mux.HandleFunc("POST /tasks/{id}/delete", deleteHTML(q))
}

func listTasks(ctx context.Context, q *db.Queries, r *http.Request) (task.TasksResponse, error) {
	projectID, projectErr := web.FormInt64Checked(r, "project_id")
	subprojectID, subprojectErr := web.FormInt64Checked(r, "subproject_id")
	if projectErr != nil {
		return task.TasksResponse{}, projectErr
	}
	if subprojectErr != nil {
		return task.TasksResponse{}, subprojectErr
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
	filter, err := task.ParseFilter(projectKey, subprojectKey)
	if err != nil {
		return task.TasksResponse{}, err
	}
	list, err := task.ListByFilter(ctx, q, filter)
	if err != nil {
		return task.TasksResponse{}, err
	}
	return task.TasksResponse{Tasks: list}, nil
}
