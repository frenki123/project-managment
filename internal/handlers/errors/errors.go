package errors

import (
	"errors"
	"net/http"

	"cad-development/internal/app"
	"cad-development/internal/monthlock"
	"cad-development/internal/project"
	"cad-development/internal/subproject"
	"cad-development/internal/task"
	"cad-development/internal/weekly"
)

func WriteError(w http.ResponseWriter, r *http.Request, err error) {
	app.WriteError(w, r, ToHTTPError(err))
}

func ToHTTPError(err error) app.HTTPError {
	if mapped, ok := mapDomainError(err); ok {
		return mapped
	}
	return app.HTTPErrorFrom(err)
}

func Render(w http.ResponseWriter, r *http.Request, status int, component app.Component) {
	if err := app.Render(w, r, status, component); err != nil {
		WriteError(w, r, err)
	}
}

func mapDomainError(err error) (app.HTTPError, bool) {
	status := http.StatusBadRequest
	message := ""
	var taskErr task.Error
	if errors.As(err, &taskErr) {
		message = taskErr.Message
		if taskErr.Kind == task.NotFound {
			status = http.StatusNotFound
		}
	}
	var projectErr project.Error
	if message == "" && errors.As(err, &projectErr) {
		message = projectErr.Message
		switch projectErr.Kind {
		case project.NotFound:
			status = http.StatusNotFound
		case project.Conflict:
			status = http.StatusConflict
		}
	}
	var subprojectErr subproject.Error
	if message == "" && errors.As(err, &subprojectErr) {
		message = subprojectErr.Message
		switch subprojectErr.Kind {
		case subproject.NotFound:
			status = http.StatusNotFound
		case subproject.Conflict:
			status = http.StatusConflict
		}
	}
	var weeklyErr weekly.Error
	if message == "" && errors.As(err, &weeklyErr) {
		message = weeklyErr.Message
		switch weeklyErr.Kind {
		case weekly.NotFound:
			status = http.StatusNotFound
		case weekly.Forbidden:
			status = http.StatusForbidden
		case weekly.Conflict:
			status = http.StatusConflict
		}
	}
	var monthlockErr monthlock.Error
	if message == "" && errors.As(err, &monthlockErr) {
		message = monthlockErr.Message
	}
	if message == "" {
		return app.HTTPError{}, false
	}
	return app.HTTPError{Status: status, Message: message}, true
}
