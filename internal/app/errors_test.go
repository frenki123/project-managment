package app_test

import (
	"errors"
	"net/http"
	"testing"

	"cad-development/internal/app"
)

func TestFromStatus(t *testing.T) {
	for _, test := range []struct {
		status int
		want   int
	}{
		{0, 0},
		{http.StatusBadRequest, http.StatusBadRequest},
		{http.StatusForbidden, http.StatusForbidden},
		{http.StatusNotFound, http.StatusNotFound},
		{http.StatusConflict, http.StatusConflict},
	} {
		err := app.FromStatus(test.status, "message")
		if test.status == 0 {
			if err != nil {
				t.Fatalf("zero status returned %v", err)
			}
			continue
		}
		var httpErr app.HTTPError
		if !errors.As(err, &httpErr) || httpErr.Status != test.want || httpErr.Message != "message" {
			t.Fatalf("FromStatus(%d) = %v", test.status, err)
		}
	}
}
