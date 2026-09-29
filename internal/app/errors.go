package app

import (
	"errors"
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
	httpErr := HTTPErrorFrom(err)
	if IsAPI(r) {
		writeHTTPError(w, httpErr)
		return
	}
	if IsHTMX(r) {
		WriteFragmentError(w, r, httpErr)
		return
	}
	writeComponentError(w, r, httpErr, ErrorPage(httpErr.Message))
}

// WriteFragmentError renders an error into an HTMX fragment target.
func WriteFragmentError(w http.ResponseWriter, r *http.Request, err error) {
	httpErr := HTTPErrorFrom(err)
	if IsAPI(r) {
		writeHTTPError(w, httpErr)
		return
	}
	if !IsHTMX(r) {
		writeComponentError(w, r, httpErr, ErrorPage(httpErr.Message))
		return
	}
	writeComponentError(w, r, httpErr, ErrorFragment(httpErr.Message))
}

func writeHTTPError(w http.ResponseWriter, httpErr HTTPError) {
	w.Header().Set("X-Content-Type-Options", "nosniff")
	JSON(w, httpErr.Status, map[string]string{"error": httpErr.Message})
}

func writeComponentError(w http.ResponseWriter, r *http.Request, httpErr HTTPError, component Component) {
	w.Header().Set("X-Content-Type-Options", "nosniff")
	if err := Render(w, r, httpErr.Status, component); err != nil {
		log.Printf("render error response: %v", err)
	}
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
