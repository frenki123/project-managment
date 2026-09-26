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
	if taskErr, ok := errors.AsType[task.Error](err); ok {
		message = taskErr.Message
		if taskErr.Kind == task.NotFound {
			status = http.StatusNotFound
		}
	}
	if message == "" {
		if projectErr, ok := errors.AsType[project.Error](err); ok {
			message = projectErr.Message
			switch projectErr.Kind {
			case project.NotFound:
				status = http.StatusNotFound
			case project.Conflict:
				status = http.StatusConflict
			}
		}
	}
	if message == "" {
		if subprojectErr, ok := errors.AsType[subproject.Error](err); ok {
			message = subprojectErr.Message
			switch subprojectErr.Kind {
			case subproject.NotFound:
				status = http.StatusNotFound
			case subproject.Conflict:
				status = http.StatusConflict
			}
		}
	}
	if message == "" {
		if weeklyErr, ok := errors.AsType[weekly.Error](err); ok {
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
	}
	if message == "" {
		if monthlockErr, ok := errors.AsType[monthlock.Error](err); ok {
			message = monthlockErr.Message
		}
	}
	if message == "" {
		return app.HTTPError{}, false
	}
	return app.HTTPError{Status: status, Message: message}, true
}
