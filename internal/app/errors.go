package app

import (
	"errors"
	"fmt"
	"html"
	"log"
	"net/http"
)

// HTTPError is the transport-facing error used by HTTP adapters.
type HTTPError struct {
	Status  int
	Message string
}

func (e HTTPError) Error() string {
	return e.Message
}

func WriteError(w http.ResponseWriter, r *http.Request, err error) {
	httpErr := HTTPErrorFrom(err)
	if IsAPI(r) {
		JSON(w, httpErr.Status, map[string]string{"error": httpErr.Message})
		return
	}
	if IsHTMX(r) {
		if target := r.Header.Get("HX-Target"); target != "" {
			w.Header().Set("HX-Retarget", target)
		}
		w.Header().Set("HX-Reswap", "innerHTML")
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.WriteHeader(http.StatusOK)
		_, _ = fmt.Fprintf(w, `<p class="error">%s</p>`, html.EscapeString(httpErr.Message))
		return
	}
	http.Error(w, httpErr.Message, httpErr.Status)
}

func HTTPErrorFrom(err error) HTTPError {
	if httpErr, ok := errors.AsType[HTTPError](err); ok {
		return httpErr
	}
	log.Println(err)
	return HTTPError{Status: http.StatusInternalServerError, Message: "internal error"}
}
