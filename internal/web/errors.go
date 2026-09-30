package web

import (
	"errors"
	"log"
	"net/http"

	"cad-development/internal/db"
)

// HTTPError is the single error type used across the app and its HTTP adapters.
type HTTPError struct {
	Status  int
	Reason  string
	Message string
}

func (e HTTPError) Error() string {
	return e.Message
}

func Invalid(message string) error  { return HTTPError{Status: http.StatusBadRequest, Message: message} }
func Missing(message string) error  { return HTTPError{Status: http.StatusNotFound, Message: message} }
func Conflict(message string) error { return HTTPError{Status: http.StatusConflict, Message: message} }
func Locked(message string) error   { return HTTPError{Status: http.StatusForbidden, Message: message} }

// reasonErrors maps the stable machine reason codes from the SQL conflict queries to their human-readable messages.
var reasonErrors = map[string]string{
	"idea-task-not-assignable":          "ideas cannot be planned",
	"week-outside-project-bounds":       "week is outside the project date range",
	"subproject-not-found":              "subproject not found",
	"project-not-found":                 "project not found",
	"subproject-project-mismatch":       "subproject does not belong to project",
	"project-hours-below-subprojects":   "project hours cannot be less than subproject hours",
	"project-dates-exclude-weekly-data": "project dates cannot exclude existing weekly data",
	"subproject-hours-exceed-project":   "subproject hours exceed project hours",
	"project-name-taken":                "project name already exists",
	"task-has-weekly-data":              "cannot reassign task with weekly data",
}

// HTTPErrorFromReason converts a conflict row's status and machine reason code into an HTTPError.
func HTTPErrorFromReason(status int, reason string) error {
	if status == 0 {
		return nil
	}
	message, ok := reasonErrors[reason]
	if !ok {
		message = reason
	}
	return HTTPError{Status: status, Reason: reason, Message: message}
}

func ReferencedConflict(err error) error {
	if db.ForeignKeyViolation(err) {
		return Conflict("record is still used by other data")
	}
	return err
}

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
	payload := map[string]string{"error": httpErr.Message}
	if httpErr.Reason != "" {
		payload["reason"] = httpErr.Reason
	}
	JSON(w, httpErr.Status, payload)
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
