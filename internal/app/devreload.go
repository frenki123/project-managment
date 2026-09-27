package app

import (
	"net/http"
)

// RegisterDevReload exposes a connection that ends when the development
// server is replaced. The browser uses that disconnect to reload the page.
func RegisterDevReload(mux *http.ServeMux) {
	mux.HandleFunc("GET /__dev/reload", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "no-store")
		w.Header().Set("Content-Type", "text/event-stream")
		w.Header().Set("X-Content-Type-Options", "nosniff")
		flusher, ok := w.(http.Flusher)
		if !ok {
			http.Error(w, "streaming unsupported", http.StatusInternalServerError)
			return
		}
		_, _ = w.Write([]byte(": connected\n\n"))
		flusher.Flush()

		<-r.Context().Done()
	})
}
