package historyaccesshandler

import (
	"net/http"
	"time"

	"cad-development/internal/app"
	"cad-development/internal/db"
	taskhandler "cad-development/internal/handlers/task"
	"cad-development/internal/historyaccess"
	taskdomain "cad-development/internal/task"
)

func Register(mux *http.ServeMux, q *db.Queries) {
	mux.HandleFunc("POST /history-access/set", setHistoryAccess(q))
}

func setHistoryAccess(q *db.Queries) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		historicalEditingValue := r.FormValue("historical_editing")
		if historicalEditingValue != "true" && historicalEditingValue != "false" {
			app.WriteError(w, r, app.Invalid("historical_editing must be true or false"))
			return
		}
		now := time.Now()
		filter, err := taskdomain.ParseFilter(r.FormValue("project"), r.FormValue("subproject"))
		if err == nil {
			_, err = filter.Resolve(r.Context(), q)
		}
		if err != nil {
			app.WriteError(w, r, err)
			return
		}
		historicalEditingAllowed := historicalEditingValue == "true"
		if historicalEditingAllowed {
			historyaccess.SetCookie(w, now)
		} else {
			historyaccess.ClearCookie(w)
		}
		if app.IsHTMX(r) {
			taskhandler.RenderGrid(w, r, q, now, historicalEditingAllowed)
			return
		}
		app.Redirect(w, r, "/?"+filter.Key())
	}
}
