package handlers

import (
	"log"
	"net/http"
	"path/filepath"

	"cad-development/internal/db"
	"cad-development/internal/views"
)

func Register(mux *http.ServeMux, q *db.Queries, root string) {
	static := http.FileServer(http.Dir(filepath.Join(root, "static")))
	mux.Handle("GET /static/", http.StripPrefix("/static/", static))
	mux.HandleFunc("GET /{$}", home(q))
	mux.HandleFunc("GET /health", health)
	mux.HandleFunc("GET /ping", ping)
}

func home(q *db.Queries) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		name, err := q.GetMeta(r.Context(), "app")
		if err != nil {
			http.Error(w, "database error", http.StatusInternalServerError)
			log.Println(err)
			return
		}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		if err := views.Home(name).Render(r.Context(), w); err != nil {
			log.Println(err)
		}
	}
}

func health(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte("ok"))
}

func ping(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	_, _ = w.Write([]byte("pong"))
}
