package app_test

import (
	"context"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"math"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	assets "cad-development"
	"cad-development/internal/app"
)

type failingComponent struct{}

func (failingComponent) Render(context.Context, io.Writer) error {
	return errors.New("render failed")
}

type textComponent string

func (c textComponent) Render(_ context.Context, w io.Writer) error {
	_, err := io.WriteString(w, string(c))
	return err
}

type failingWriter struct {
	header http.Header
}

func (w *failingWriter) Header() http.Header { return w.header }
func (w *failingWriter) WriteHeader(int)     {}
func (w *failingWriter) Write([]byte) (int, error) {
	return 0, errors.New("client disconnected")
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

func TestDecodeJSONRejectsTrailingContent(t *testing.T) {
	r := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(`{"known":"ok"}{"known":"second"}`))
	var value struct {
		Known string `json:"known"`
	}
	if err := app.DecodeJSON(httptest.NewRecorder(), r, &value); err == nil {
		t.Fatal("expected trailing JSON to be rejected")
	}
}

func TestDecodeJSONMatchesFieldNamesCaseInsensitively(t *testing.T) {
	r := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(`{"KNOWN":"ok"}`))
	var value struct {
		Known string `json:"known"`
	}
	if err := app.DecodeJSON(httptest.NewRecorder(), r, &value); err != nil {
		t.Fatal(err)
	}
	if value.Known != "ok" {
		t.Fatalf("got %q", value.Known)
	}
}

func TestDecodeJSONRejectsDuplicateNames(t *testing.T) {
	r := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(`{"known":"first","known":"second"}`))
	var value struct {
		Known string `json:"known"`
	}
	if err := app.DecodeJSON(httptest.NewRecorder(), r, &value); err == nil {
		t.Fatal("expected duplicate JSON names to be rejected")
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
	if got := rr.Header().Get("X-Content-Type-Options"); got != "nosniff" {
		t.Fatalf("got nosniff header %q", got)
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
		if rr.Code != http.StatusNotFound || rr.Header().Get("HX-Retarget") != "" || rr.Header().Get("Content-Type") != "text/plain; charset=utf-8" || rr.Body.String() != "missing" {
			t.Fatalf("got %d %q with headers %v", rr.Code, rr.Body.String(), rr.Header())
		}
	})
}

func TestWriteHTMXErrorRendersExplicitFragment(t *testing.T) {
	rr := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/tasks/1", nil)
	r.Header.Set("HX-Request", "true")
	r.Header.Set("HX-Target", "#panel")
	app.WriteHTMXError(rr, r, app.HTTPError{Status: http.StatusNotFound, Message: "<missing>"})
	if rr.Code != http.StatusOK || rr.Header().Get("HX-Retarget") != "#panel" || !strings.Contains(rr.Body.String(), "&lt;missing&gt;") {
		t.Fatalf("got %d %q with headers %v", rr.Code, rr.Body.String(), rr.Header())
	}
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

func TestFormParsingDistinguishesEmptyAndMalformedValues(t *testing.T) {
	empty := httptest.NewRequest(http.MethodPost, "/", strings.NewReader("hours="))
	empty.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	value, present, err := app.FormFloatValue(empty, "hours")
	if err != nil || !present || value != 0 {
		t.Fatalf("empty value: got %v %v %v", value, present, err)
	}

	malformed := httptest.NewRequest(http.MethodPost, "/", strings.NewReader("project_id=%zz"))
	malformed.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	if _, err := app.FormInt64Checked(malformed, "project_id"); err == nil {
		t.Fatal("expected malformed form error")
	}
}

func TestRedirectWithFormFilter(t *testing.T) {
	r := httptest.NewRequest(http.MethodPost, "/", strings.NewReader("project=7&subproject=3"))
	r.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rr := httptest.NewRecorder()
	app.RedirectWithFormFilter(rr, r, "/", "project", "subproject")
	if got := rr.Header().Get("Location"); got != "/?project=7&subproject=3" {
		t.Fatalf("got redirect %q", got)
	}
}

func TestJSONMarshalFailureReturnsInternalError(t *testing.T) {
	rr := httptest.NewRecorder()
	app.JSON(rr, http.StatusOK, math.NaN())
	if rr.Code != http.StatusInternalServerError || rr.Body.String() != `{"error":"internal error"}` {
		t.Fatalf("got %d %q", rr.Code, rr.Body.String())
	}
}

func TestDecodeJSONRejectsOversizedBody(t *testing.T) {
	body := strings.NewReader(fmt.Sprintf(`{"known":"%s"}`, strings.Repeat("x", 1<<20)))
	r := httptest.NewRequest(http.MethodPost, "/", body)
	var value struct {
		Known string `json:"known"`
	}
	err := app.DecodeJSON(httptest.NewRecorder(), r, &value)
	if httpErr, ok := errors.AsType[app.HTTPError](err); !ok || httpErr.Status != http.StatusRequestEntityTooLarge {
		t.Fatalf("got %v", err)
	}
}

func TestStaticHandlerRejectsDirectories(t *testing.T) {
	root := t.TempDir()
	staticRoot := filepath.Join(root, "static")
	if err := os.Mkdir(staticRoot, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(staticRoot, "app.js"), []byte("ok"), 0o644); err != nil {
		t.Fatal(err)
	}
	handler := app.StaticHandler(os.DirFS(staticRoot))
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
	handler := app.StaticHandler(staticFS)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, httptest.NewRequest(http.MethodGet, "/static/js/app.js", nil))
	if rr.Code != http.StatusOK || !strings.Contains(rr.Body.String(), "htmx:responseError") {
		t.Fatalf("embedded asset response %d %q", rr.Code, rr.Body.String())
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

func TestRenderWritesSuccessfulComponent(t *testing.T) {
	rr := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/", nil)
	if err := app.Render(rr, r, http.StatusCreated, textComponent("rendered")); err != nil {
		t.Fatal(err)
	}
	if rr.Code != http.StatusCreated || rr.Header().Get("Content-Type") != "text/html; charset=utf-8" || rr.Body.String() != "rendered" {
		t.Fatalf("got %d %q %q", rr.Code, rr.Header().Get("Content-Type"), rr.Body.String())
	}
}

func TestRenderClassifiesResponseWriteFailure(t *testing.T) {
	r := httptest.NewRequest(http.MethodGet, "/", nil)
	err := app.Render(&failingWriter{header: make(http.Header)}, r, http.StatusOK, textComponent("rendered"))
	if _, ok := errors.AsType[app.ResponseError](err); !ok {
		t.Fatalf("got %T: %v", err, err)
	}
}

func TestRecoverReturnsInternalServerError(t *testing.T) {
	handler := app.Recover(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		panic("test panic")
	}))
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, httptest.NewRequest(http.MethodGet, "/", nil))
	if rr.Code != http.StatusInternalServerError || !strings.Contains(rr.Body.String(), "internal server error") {
		t.Fatalf("got %d %q", rr.Code, rr.Body.String())
	}
}
