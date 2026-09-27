package app

import (
	"errors"
	"html"
	"log"
	"net/http"
	"strings"
)

// HTTPError is the single error type used across the app and its HTTP adapters.
type HTTPError struct {
	Status  int
	Message string
}

func (e HTTPError) Error() string {
	return e.Message
}

func Invalid(message string) error  { return HTTPError{Status: http.StatusBadRequest, Message: message} }
func Missing(message string) error  { return HTTPError{Status: http.StatusNotFound, Message: message} }
func Conflict(message string) error { return HTTPError{Status: http.StatusConflict, Message: message} }
func Locked(message string) error   { return HTTPError{Status: http.StatusForbidden, Message: message} }

func WriteError(w http.ResponseWriter, r *http.Request, err error) {
	WriteHTTPError(w, r, HTTPErrorFrom(err))
}

// WriteFragmentError renders an error into an HTMX fragment target.
func WriteFragmentError(w http.ResponseWriter, r *http.Request, err error) {
	WriteHTMXError(w, r, HTTPErrorFrom(err))
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

func FriendlyFormMessage(message string) string {
	message = strings.ReplaceAll(message, "end_date", "End date")
	message = strings.ReplaceAll(message, "start_date", "start date")
	message = strings.ReplaceAll(message, "total_hours", "total hours")
	message = strings.ReplaceAll(message, "subproject_id", "subproject")
	message = strings.ReplaceAll(message, "project_id", "project")
	return message
}
