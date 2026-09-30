package web_test

import (
	"errors"
	"net/http"
	"testing"

	"cad-development/internal/web"
)

func TestHTTPErrorFromReason(t *testing.T) {
	cases := []struct {
		status  int
		reason  string
		message string
	}{
		{http.StatusBadRequest, "idea-task-not-assignable", "ideas cannot be planned"},
		{http.StatusConflict, "project-name-taken", "project name already exists"},
		{http.StatusConflict, "task-has-weekly-data", "cannot reassign task with weekly data"},
	}
	for _, tc := range cases {
		httpErr, ok := errors.AsType[web.HTTPError](web.HTTPErrorFromReason(tc.status, tc.reason))
		if !ok || httpErr.Status != tc.status || httpErr.Reason != tc.reason || httpErr.Message != tc.message {
			t.Fatalf("HTTPErrorFromReason(%d, %q) = %#v", tc.status, tc.reason, httpErr)
		}
	}
	if err := web.HTTPErrorFromReason(0, ""); err != nil {
		t.Fatalf("zero status should return nil, got %v", err)
	}
}
