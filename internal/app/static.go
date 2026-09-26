package app

import (
	"net/http"
	"path/filepath"
)

func StaticHandler(root string) http.Handler {
	static := http.FileServer(http.Dir(filepath.Join(root, "static")))
	return http.StripPrefix("/static/", static)
}
