package web_test

import (
	"context"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	assets "cad-development"
	"cad-development/internal/web"
)

type textComponent string

func (c textComponent) Render(_ context.Context, w io.Writer) error {
	_, err := io.WriteString(w, string(c))
	return err
}

func TestJSONWritesContentTypeAndStatus(t *testing.T) {
	rr := httptest.NewRecorder()
	web.JSON(rr, http.StatusCreated, map[string]string{"status": "created"})
	if rr.Code != http.StatusCreated {
		t.Fatalf("got status %d", rr.Code)
	}
	if got := rr.Header().Get("Content-Type"); got != "application/json; charset=utf-8" {
		t.Fatalf("got content type %q", got)
	}
	if got := rr.Header().Get("X-Content-Type-Options"); got != "nosniff" {
		t.Fatalf("got nosniff header %q", got)
	}
}

func TestWriteErrorUsesAPIAndHTMLResponses(t *testing.T) {
	t.Run("api", func(t *testing.T) {
		rr := httptest.NewRecorder()
		r := httptest.NewRequest(http.MethodGet, "/api/v1/projects", nil)
		web.WriteError(rr, r, web.HTTPError{Status: http.StatusNotFound, Message: "missing"})
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
		web.WriteError(rr, r, web.HTTPError{Status: http.StatusBadRequest, Message: "invalid"})
		if rr.Code != http.StatusBadRequest || !strings.Contains(rr.Body.String(), "invalid") {
			t.Fatalf("got %d %q", rr.Code, rr.Body.String())
		}
	})
	t.Run("htmx", func(t *testing.T) {
		rr := httptest.NewRecorder()
		r := httptest.NewRequest(http.MethodGet, "/tasks/1", nil)
		r.Header.Set("HX-Request", "true")
		r.Header.Set("HX-Target", "#panel")
		web.WriteError(rr, r, web.HTTPError{Status: http.StatusNotFound, Message: "missing"})
		if rr.Code != http.StatusNotFound || rr.Header().Get("HX-Retarget") != "" || rr.Header().Get("Content-Type") != "text/html; charset=utf-8" || !strings.Contains(rr.Body.String(), "missing") {
			t.Fatalf("got %d %q with headers %v", rr.Code, rr.Body.String(), rr.Header())
		}
	})
}

func TestWriteFragmentErrorRendersExplicitFragment(t *testing.T) {
	rr := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/tasks/1", nil)
	r.Header.Set("HX-Request", "true")
	r.Header.Set("HX-Target", "#panel")
	web.WriteFragmentError(rr, r, web.HTTPError{Status: http.StatusNotFound, Message: "<missing>"})
	if rr.Code != http.StatusNotFound || rr.Header().Get("HX-Retarget") != "" || !strings.Contains(rr.Body.String(), "&lt;missing&gt;") {
		t.Fatalf("got %d %q with headers %v", rr.Code, rr.Body.String(), rr.Header())
	}
}

func TestAPIPathsIgnoreHTMXHeaders(t *testing.T) {
	rr := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/api/v1/projects/999", nil)
	r.Header.Set("HX-Request", "true")
	r.Header.Set("HX-Target", "#panel")
	web.WriteError(rr, r, web.Missing("project not found"))
	if rr.Code != http.StatusNotFound || rr.Header().Get("HX-Retarget") != "" || !strings.Contains(rr.Body.String(), `"error":"project not found"`) {
		t.Fatalf("got %d %q with headers %v", rr.Code, rr.Body.String(), rr.Header())
	}
}

func TestErrorHelpersMapToStatusCodes(t *testing.T) {
	tests := []struct {
		name   string
		err    error
		status int
	}{
		{"invalid", web.Invalid("bad"), http.StatusBadRequest},
		{"missing", web.Missing("gone"), http.StatusNotFound},
		{"conflict", web.Conflict("busy"), http.StatusConflict},
		{"locked", web.Locked("no"), http.StatusForbidden},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if httpErr, ok := errors.AsType[web.HTTPError](tt.err); !ok || httpErr.Status != tt.status {
				t.Fatalf("got %#v", tt.err)
			}
		})
	}
}

func TestFormAndPathParsing(t *testing.T) {
	r := httptest.NewRequest(http.MethodPost, "/tasks/12", strings.NewReader("hours=4.5"))
	r.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	id, err := web.FormInt64Checked(r, "missing")
	if err != nil || id != nil {
		t.Fatalf("expected missing ID to be optional, got %v %v", id, err)
	}
	hours, err := web.FormFloatRequired(r, "hours")
	if err != nil || hours != 4.5 {
		t.Fatalf("got hours %v with error %v", hours, err)
	}
	r.SetPathValue("id", "12")
	parsed, err := web.PathID(r, "id")
	if err != nil || parsed != 12 {
		t.Fatalf("got path ID %d with error %v", parsed, err)
	}
}

func TestFormParsingDistinguishesEmptyAndMalformedValues(t *testing.T) {
	empty := httptest.NewRequest(http.MethodPost, "/", strings.NewReader("hours="))
	empty.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	value, present, err := web.FormFloatValue(empty, "hours")
	if err != nil || !present || value != 0 {
		t.Fatalf("empty value: got %v %v %v", value, present, err)
	}

	malformed := httptest.NewRequest(http.MethodPost, "/", strings.NewReader("project_id=%zz"))
	malformed.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	malformedValue, err := web.FormInt64Checked(malformed, "project_id")
	if err == nil {
		t.Fatal("expected malformed form error")
	}
	if malformedValue != nil {
		t.Fatalf("malformed form returned a value: %#v", malformedValue)
	}
}

func TestDecodeJSONRejectsOversizedBody(t *testing.T) {
	body := strings.NewReader(fmt.Sprintf(`{"known":"%s"}`, strings.Repeat("x", 1<<20)))
	r := httptest.NewRequest(http.MethodPost, "/", body)
	var value struct {
		Known string `json:"known"`
	}
	err := web.DecodeJSON(httptest.NewRecorder(), r, &value)
	if httpErr, ok := errors.AsType[web.HTTPError](err); !ok || httpErr.Status != http.StatusRequestEntityTooLarge {
		t.Fatalf("got %v", err)
	}
}

func TestStaticHandlerRejectsDirectories(t *testing.T) {
	root := t.TempDir()
	staticRoot := filepath.Join(root, "static")
	if err := os.Mkdir(staticRoot, 0o750); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(staticRoot, "app.js"), []byte("ok"), 0o600); err != nil {
		t.Fatal(err)
	}
	handler := web.StaticHandler(os.DirFS(staticRoot))
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

func TestStaticHandlerServesEmbeddedAsset(t *testing.T) {
	staticFS, err := fs.Sub(assets.FS, "static")
	if err != nil {
		t.Fatal(err)
	}
	handler := web.StaticHandler(staticFS)
	for _, name := range []string{"/static/js/app.js", "/static/js/chart.umd.min.js"} {
		rr := httptest.NewRecorder()
		handler.ServeHTTP(rr, httptest.NewRequest(http.MethodGet, name, nil))
		if rr.Code != http.StatusOK || rr.Body.Len() == 0 {
			t.Fatalf("embedded asset %s response %d %q", name, rr.Code, rr.Body.String())
		}
	}
}

func TestRenderWritesSuccessfulComponent(t *testing.T) {
	rr := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/", nil)
	web.RenderPage(rr, r, http.StatusCreated, textComponent("rendered"))
	if rr.Code != http.StatusCreated || rr.Header().Get("Content-Type") != "text/html; charset=utf-8" || rr.Body.String() != "rendered" {
		t.Fatalf("got %d %q %q", rr.Code, rr.Header().Get("Content-Type"), rr.Body.String())
	}
}

func TestRecoverReturnsInternalServerError(t *testing.T) {
	handler := web.Recover(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		panic("test panic")
	}))
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, httptest.NewRequest(http.MethodGet, "/", nil))
	if rr.Code != http.StatusInternalServerError || !strings.Contains(rr.Body.String(), "internal server error") {
		t.Fatalf("got %d %q", rr.Code, rr.Body.String())
	}
}
