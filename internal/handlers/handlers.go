package handlers

import (
	"net/http"

	"cad-development/internal/db"
	charthdl "cad-development/internal/handlers/chart"
	historyaccesshdl "cad-development/internal/handlers/historyaccess"
	personhdl "cad-development/internal/handlers/person"
	projecthdl "cad-development/internal/handlers/project"
	subprojecthdl "cad-development/internal/handlers/subproject"
	taskhdl "cad-development/internal/handlers/task"
)

func Register(mux *http.ServeMux, q *db.Queries) {
	projecthdl.Register(mux, q)
	subprojecthdl.Register(mux, q)
	taskhdl.Register(mux, q)
	personhdl.Register(mux, q)
	historyaccesshdl.Register(mux, q)
	charthdl.Register(mux, q)
}
