package app

import (
	"net/http"
	"path/filepath"
	"strings"
)

func StaticHandler(root string) http.Handler {
	static := http.FileServer(http.Dir(filepath.Join(root, "static")))
	assets := http.StripPrefix("/static/", static)
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasSuffix(r.URL.Path, "/") {
			http.NotFound(w, r)
			return
		}
		assets.ServeHTTP(w, r)
	})
}

func RegisterStatic(mux *http.ServeMux, root string) {
	mux.Handle("GET /static/", StaticHandler(root))
}
