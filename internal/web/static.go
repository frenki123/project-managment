package web

import (
	"io/fs"
	"net/http"
	"strings"
)

func StaticHandler(staticFS fs.FS) http.Handler {
	static := http.FileServer(http.FS(staticFS))
	assets := http.StripPrefix("/static/", static)
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasSuffix(r.URL.Path, "/") {
			http.NotFound(w, r)
			return
		}
		assets.ServeHTTP(w, r)
	})
}

func RegisterStatic(mux *http.ServeMux, staticFS fs.FS) {
	mux.Handle("GET /static/", StaticHandler(staticFS))
}
