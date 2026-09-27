package handlers

import (
	"net/http"

	"cad-development/internal/db"
	charthandler "cad-development/internal/handlers/chart"
	monthlockhandler "cad-development/internal/handlers/monthlock"
	projecthandler "cad-development/internal/handlers/project"
	subprojecthandler "cad-development/internal/handlers/subproject"
	taskhandler "cad-development/internal/handlers/task"
)

func Register(mux *http.ServeMux, q *db.Queries) {
	projecthandler.Register(mux, q)
	subprojecthandler.Register(mux, q)
	taskhandler.Register(mux, q)
	monthlockhandler.Register(mux, q)
	charthandler.Register(mux, q)
}
