package errors_test

import (
	"errors"
	"fmt"
	"net/http"
	"testing"

	handlererrors "cad-development/internal/handlers/errors"
	"cad-development/internal/monthlock"
	"cad-development/internal/project"
	"cad-development/internal/subproject"
	"cad-development/internal/task"
	"cad-development/internal/weekly"
)

func TestToHTTPErrorMapsDomainOutcomes(t *testing.T) {
	tests := []struct {
		name   string
		err    error
		status int
		msg    string
	}{
		{name: "task missing", err: task.Missing("task missing"), status: http.StatusNotFound, msg: "task missing"},
		{name: "project conflict", err: project.ConflictError("project conflict"), status: http.StatusConflict, msg: "project conflict"},
		{name: "subproject missing", err: subproject.Missing("subproject missing"), status: http.StatusNotFound, msg: "subproject missing"},
		{name: "week locked", err: weekly.Locked("week locked"), status: http.StatusForbidden, msg: "week locked"},
		{name: "month invalid", err: monthlock.Invalid("month invalid"), status: http.StatusBadRequest, msg: "month invalid"},
		{name: "wrapped", err: fmt.Errorf("wrapped: %w", project.Missing("project missing")), status: http.StatusNotFound, msg: "project missing"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := handlererrors.ToHTTPError(tt.err)
			if got.Status != tt.status || got.Message != tt.msg {
				t.Fatalf("got %#v", got)
			}
		})
	}
}

func TestToHTTPErrorHidesUnexpectedErrors(t *testing.T) {
	got := handlererrors.ToHTTPError(errors.New("database driver details"))
	if got.Status != http.StatusInternalServerError || got.Message != "internal error" {
		t.Fatalf("got %#v", got)
	}
}
