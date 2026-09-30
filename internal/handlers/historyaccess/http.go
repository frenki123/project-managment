package historyaccesshandler

import (
	"net/http"
	"time"

	"cad-development/internal/db"
	taskhandler "cad-development/internal/handlers/task"
	"cad-development/internal/historyaccess"
	"cad-development/internal/task"
	"cad-development/internal/web"
)

func Register(mux *http.ServeMux, q *db.Queries) {
	mux.HandleFunc("POST /history-access/set", setHistoryAccess(q))
}

func setHistoryAccess(q *db.Queries) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		historicalEditingValue := r.FormValue("historical_editing")
		if historicalEditingValue != "true" && historicalEditingValue != "false" {
			web.WriteError(w, r, web.Invalid("historical_editing must be true or false"))
			return
		}
		now := time.Now()
		filter, err := task.ParseFilter(r.FormValue("project"), r.FormValue("subproject"))
		if err == nil {
			_, err = task.ResolveFilter(r.Context(), q, filter)
		}
		if err != nil {
			web.WriteError(w, r, err)
			return
		}
		historicalEditingAllowed := historicalEditingValue == "true"
		if historicalEditingAllowed {
			historyaccess.SetCookie(w, now)
		} else {
			historyaccess.ClearCookie(w)
		}
		if web.IsHTMX(r) {
			taskhandler.RenderGrid(w, r, q, now, historicalEditingAllowed)
			return
		}
		web.Redirect(w, r, "/?"+task.FilterKey(filter))
	}
}
