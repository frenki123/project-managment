package app_test

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"cad-development/internal/app"
)

func TestDecodeJSONRejectsUnknownFields(t *testing.T) {
	r := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(`{"known":"ok","unknown":true}`))
	var value struct {
		Known string `json:"known"`
	}
	if err := app.DecodeJSON(r, &value); err == nil {
		t.Fatal("expected unknown field to be rejected")
	}
}

func TestJSONWritesContentTypeAndStatus(t *testing.T) {
	rr := httptest.NewRecorder()
	app.JSON(rr, http.StatusCreated, map[string]string{"status": "created"})
	if rr.Code != http.StatusCreated {
		t.Fatalf("got status %d", rr.Code)
	}
	if got := rr.Header().Get("Content-Type"); got != "application/json; charset=utf-8" {
		t.Fatalf("got content type %q", got)
	}
}

func TestWriteErrorUsesAPIAndHTMLResponses(t *testing.T) {
	t.Run("api", func(t *testing.T) {
		rr := httptest.NewRecorder()
		r := httptest.NewRequest(http.MethodGet, "/api/v1/projects", nil)
		app.WriteError(rr, r, app.HTTPError{Status: http.StatusNotFound, Message: "missing"})
		if rr.Code != http.StatusNotFound {
			t.Fatalf("got status %d", rr.Code)
		}
		if got := rr.Header().Get("Content-Type"); got != "application/json; charset=utf-8" {
			t.Fatalf("got content type %q", got)
		}
	})

	t.Run("html", func(t *testing.T) {
		rr := httptest.NewRecorder()
		r := httptest.NewRequest(http.MethodGet, "/projects/1", nil)
		app.WriteError(rr, r, app.HTTPError{Status: http.StatusBadRequest, Message: "invalid"})
		if rr.Code != http.StatusBadRequest || !strings.Contains(rr.Body.String(), "invalid") {
			t.Fatalf("got %d %q", rr.Code, rr.Body.String())
		}
	})
}

func TestFormAndPathParsing(t *testing.T) {
	r := httptest.NewRequest(http.MethodPost, "/tasks/12", strings.NewReader("hours=4.5"))
	r.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	id, err := app.FormInt64Checked(r, "missing")
	if err != nil || id != nil {
		t.Fatalf("expected missing ID to be optional, got %v %v", id, err)
	}
	hours, err := app.FormFloatRequired(r, "hours")
	if err != nil || hours != 4.5 {
		t.Fatalf("got hours %v with error %v", hours, err)
	}
	r.SetPathValue("id", "12")
	parsed, err := app.PathID(r, "id")
	if err != nil || parsed != 12 {
		t.Fatalf("got path ID %d with error %v", parsed, err)
	}
}
