package historyaccesshandler

import (
	"net/http"
	"time"

	"cad-development/internal/app"
	"cad-development/internal/db"
	taskhandler "cad-development/internal/handlers/task"
	"cad-development/internal/historylock"
	taskdomain "cad-development/internal/task"
)

func Register(mux *http.ServeMux, q *db.Queries) {
	mux.HandleFunc("POST /history-access/set", setHistoryAccess(q))
}

func setHistoryAccess(q *db.Queries) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		historyOpenValue := r.FormValue("history_open")
		if historyOpenValue != "true" && historyOpenValue != "false" {
			app.WriteError(w, r, app.Invalid("history_open must be true or false"))
			return
		}
		now := time.Now()
		pk, sid, err := taskdomain.ParseFilter(r.FormValue("project"), r.FormValue("subproject"))
		if err == nil {
			err = taskdomain.ValidateFilter(r.Context(), q, pk, sid)
		}
		if err != nil {
			app.WriteError(w, r, err)
			return
		}
		historyOpen := historyOpenValue == "true"
		if historyOpen {
			historylock.SetHistoricalEditingCookie(w, now)
		} else {
			historylock.ClearHistoricalEditingCookie(w)
		}
		if app.IsHTMX(r) {
			taskhandler.RenderGrid(w, r, q, now, historyOpen)
			return
		}
		app.RedirectWithFormFilter(w, r, "/", "project", "subproject")
	}
}
