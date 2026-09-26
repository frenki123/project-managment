package errors

import (
	"errors"
	"log"
	"net/http"

	"cad-development/internal/app"
	"cad-development/internal/monthlock"
	"cad-development/internal/project"
	"cad-development/internal/subproject"
	"cad-development/internal/task"
	"cad-development/internal/weekly"
)

func WriteError(w http.ResponseWriter, r *http.Request, err error) {
	app.WriteHTTPError(w, r, ToHTTPError(err))
}

func WriteHTMXError(w http.ResponseWriter, r *http.Request, err error) {
	app.WriteHTMXError(w, r, ToHTTPError(err))
}

func ToHTTPError(err error) app.HTTPError {
	if mapped, ok := mapDomainError(err); ok {
		return mapped
	}
	return app.HTTPErrorFrom(err)
}

func Render(w http.ResponseWriter, r *http.Request, status int, component app.Component) {
	if err := app.Render(w, r, status, component); err != nil {
		if _, ok := errors.AsType[app.ResponseError](err); ok {
			log.Printf("write response: %v", err)
			return
		}
		WriteError(w, r, err)
	}
}

func RenderHTMX(w http.ResponseWriter, r *http.Request, status int, component app.Component) {
	if err := app.Render(w, r, status, component); err != nil {
		if _, ok := errors.AsType[app.ResponseError](err); ok {
			log.Printf("write response: %v", err)
			return
		}
		WriteHTMXError(w, r, err)
	}
}

func mapDomainError(err error) (app.HTTPError, bool) {
	if taskErr, ok := errors.AsType[task.Error](err); ok {
		status := http.StatusBadRequest
		if taskErr.Kind == task.NotFound {
			status = http.StatusNotFound
		}
		return app.HTTPError{Status: status, Message: taskErr.Message}, true
	}
	if projectErr, ok := errors.AsType[project.Error](err); ok {
		status := http.StatusBadRequest
		switch projectErr.Kind {
		case project.NotFound:
			status = http.StatusNotFound
		case project.Conflict:
			status = http.StatusConflict
		}
		return app.HTTPError{Status: status, Message: projectErr.Message}, true
	}
	if subprojectErr, ok := errors.AsType[subproject.Error](err); ok {
		status := http.StatusBadRequest
		switch subprojectErr.Kind {
		case subproject.NotFound:
			status = http.StatusNotFound
		case subproject.Conflict:
			status = http.StatusConflict
		}
		return app.HTTPError{Status: status, Message: subprojectErr.Message}, true
	}
	if weeklyErr, ok := errors.AsType[weekly.Error](err); ok {
		status := http.StatusBadRequest
		switch weeklyErr.Kind {
		case weekly.NotFound:
			status = http.StatusNotFound
		case weekly.Forbidden:
			status = http.StatusForbidden
		case weekly.Conflict:
			status = http.StatusConflict
		}
		return app.HTTPError{Status: status, Message: weeklyErr.Message}, true
	}
	if monthlockErr, ok := errors.AsType[monthlock.Error](err); ok {
		return app.HTTPError{Status: http.StatusBadRequest, Message: monthlockErr.Message}, true
	}
	return app.HTTPError{}, false
}
