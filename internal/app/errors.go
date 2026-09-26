package app

import (
	"errors"
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
	WriteHTTPError(w, r, HTTPErrorFrom(err))
}

func WriteHTTPError(w http.ResponseWriter, r *http.Request, httpErr HTTPError) {
	httpErr = normalizeHTTPError(httpErr)
	w.Header().Set("X-Content-Type-Options", "nosniff")
	if IsAPI(r) {
		JSON(w, httpErr.Status, map[string]string{"error": httpErr.Message})
		return
	}
	if IsHTMX(r) {
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		w.WriteHeader(httpErr.Status)
		_, _ = w.Write([]byte(httpErr.Message))
		return
	}
	http.Error(w, httpErr.Message, httpErr.Status)
}

func WriteHTMXError(w http.ResponseWriter, r *http.Request, httpErr HTTPError) {
	httpErr = normalizeHTTPError(httpErr)
	w.Header().Set("X-Content-Type-Options", "nosniff")
	if !IsHTMX(r) {
		WriteHTTPError(w, r, httpErr)
		return
	}
	w.Header().Set("HX-Retarget", r.Header.Get("HX-Target"))
	w.Header().Set("HX-Reswap", "innerHTML")
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte(`<p class="error">` + html.EscapeString(httpErr.Message) + `</p>`))
}

func normalizeHTTPError(httpErr HTTPError) HTTPError {
	if httpErr.Status == 0 {
		httpErr.Status = http.StatusInternalServerError
	}
	return httpErr
}

func HTTPErrorFrom(err error) HTTPError {
	if httpErr, ok := errors.AsType[HTTPError](err); ok {
		return normalizeHTTPError(httpErr)
	}
	log.Println(err)
	return HTTPError{Status: http.StatusInternalServerError, Message: "internal error"}
}
