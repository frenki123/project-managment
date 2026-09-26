package taskhandler

import (
	"net/http"

	"cad-development/internal/db"
)

func Register(mux *http.ServeMux, q *db.Queries) {
	mux.HandleFunc("GET /api/v1/tasks", listJSON(q))
	mux.HandleFunc("POST /api/v1/tasks", createJSON(q))
	mux.HandleFunc("GET /api/v1/tasks/{id}", getJSON(q))
	mux.HandleFunc("PUT /api/v1/tasks/{id}", updateJSON(q))
	mux.HandleFunc("DELETE /api/v1/tasks/{id}", deleteJSON(q))
	mux.HandleFunc("PUT /api/v1/tasks/{id}/weeks/{weekStart}", weekJSON(q))
	mux.HandleFunc("GET /tasks/new", newForm(q))
	mux.HandleFunc("POST /tasks", createHTML(q))
	mux.HandleFunc("GET /tasks/{id}", panel(q))
	mux.HandleFunc("GET /tasks/{id}/edit", editForm(q))
	mux.HandleFunc("POST /tasks/{id}", updateHTML(q))
	mux.HandleFunc("POST /tasks/{id}/delete", deleteHTML(q))
	mux.HandleFunc("POST /tasks/{id}/weeks/{weekStart}", weekHTML(q))
}

func GridPage(q *db.Queries) http.HandlerFunc {
	return gridPage(q)
}
