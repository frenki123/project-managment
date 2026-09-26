package handlers

import (
	"net/http"
	"path/filepath"

	"cad-development/internal/db"
	"cad-development/internal/monthlock"
	"cad-development/internal/project"
	"cad-development/internal/subproject"
	"cad-development/internal/task"
)

func Register(mux *http.ServeMux, q *db.Queries, root string) {
	static := http.FileServer(http.Dir(filepath.Join(root, "static")))
	mux.Handle("GET /static/", http.StripPrefix("/static/", static))
	project.Register(mux, q)
	subproject.Register(mux, q)
	task.Register(mux, q)
	monthlock.Register(mux, q)
}
