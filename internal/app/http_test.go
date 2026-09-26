package app_test

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"cad-development/internal/app"
)

type failingComponent struct{}

func (failingComponent) Render(context.Context, io.Writer) error {
	return errors.New("render failed")
}

func TestDecodeJSONRejectsUnknownFields(t *testing.T) {
	r := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(`{"known":"ok","unknown":true}`))
	var value struct {
		Known string `json:"known"`
	}
	if err := app.DecodeJSON(httptest.NewRecorder(), r, &value); err == nil {
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
	t.Run("htmx", func(t *testing.T) {
		rr := httptest.NewRecorder()
		r := httptest.NewRequest(http.MethodGet, "/tasks/1", nil)
		r.Header.Set("HX-Request", "true")
		r.Header.Set("HX-Target", "#panel")
		app.WriteError(rr, r, app.HTTPError{Status: http.StatusNotFound, Message: "missing"})
		if rr.Code != http.StatusOK || rr.Header().Get("HX-Retarget") != "#panel" || !strings.Contains(rr.Body.String(), "missing") {
			t.Fatalf("got %d %q with headers %v", rr.Code, rr.Body.String(), rr.Header())
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

func TestDecodeJSONRejectsOversizedBody(t *testing.T) {
	body := strings.NewReader(fmt.Sprintf(`{"known":"%s"}`, strings.Repeat("x", 1<<20)))
	r := httptest.NewRequest(http.MethodPost, "/", body)
	var value struct {
		Known string `json:"known"`
	}
	err := app.DecodeJSON(httptest.NewRecorder(), r, &value)
	var httpErr app.HTTPError
	if !errors.As(err, &httpErr) || httpErr.Status != http.StatusRequestEntityTooLarge {
		t.Fatalf("got %v", err)
	}
}

func TestStaticHandlerRejectsDirectories(t *testing.T) {
	root := t.TempDir()
	if err := os.Mkdir(filepath.Join(root, "static"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "static", "app.js"), []byte("ok"), 0o644); err != nil {
		t.Fatal(err)
	}
	handler := app.StaticHandler(root)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, httptest.NewRequest(http.MethodGet, "/static/", nil))
	if rr.Code != http.StatusNotFound {
		t.Fatalf("directory status %d", rr.Code)
	}
	rr = httptest.NewRecorder()
	handler.ServeHTTP(rr, httptest.NewRequest(http.MethodGet, "/static/app.js", nil))
	if rr.Code != http.StatusOK || rr.Body.String() != "ok" {
		t.Fatalf("asset response %d %q", rr.Code, rr.Body.String())
	}
}

func TestRenderDoesNotCommitOnFailure(t *testing.T) {
	rr := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/", nil)
	if err := app.Render(rr, r, http.StatusOK, failingComponent{}); err == nil {
		t.Fatal("expected render error")
	}
	if rr.Header().Get("Content-Type") != "" || rr.Body.Len() != 0 {
		t.Fatalf("response was committed: headers=%v body=%q", rr.Header(), rr.Body.String())
	}
}
