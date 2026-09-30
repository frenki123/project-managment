package app

import (
	"bytes"
	"context"
	json "encoding/json/v2"
	"errors"
	"io"
	"log"
	"net/http"
	"strconv"
	"strings"
)

type Component interface {
	Render(context.Context, io.Writer) error
}

type ResponseError struct{ Err error }

func (e ResponseError) Error() string { return e.Err.Error() }
func (e ResponseError) Unwrap() error { return e.Err }

func IsAPI(r *http.Request) bool {
	return strings.HasPrefix(r.URL.Path, "/api/")
}

func IsHTMX(r *http.Request) bool {
	return r.Header.Get("HX-Request") != ""
}

func PathID(r *http.Request, key string) (int64, error) {
	id, err := strconv.ParseInt(r.PathValue(key), 10, 64)
	if err != nil || id < 1 {
		return 0, HTTPError{Status: http.StatusBadRequest, Message: "invalid id"}
	}
	return id, nil
}

func JSON[T any](w http.ResponseWriter, status int, v T) {
	var body bytes.Buffer
	if err := json.MarshalWrite(&body, v); err != nil {
		log.Printf("marshal JSON response: %v", err)
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte(`{"error":"internal error"}`))
		return
	}
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.WriteHeader(status)
	_, _ = w.Write(body.Bytes())
}

func DecodeJSON[T any](w http.ResponseWriter, r *http.Request, v *T) error {
	r.Body = http.MaxBytesReader(w, r.Body, 1<<20)
	defer r.Body.Close()
	if err := json.UnmarshalRead(r.Body, v,
		json.RejectUnknownMembers(true),
		json.MatchCaseInsensitiveNames(true),
	); err != nil {
		if _, ok := errors.AsType[*http.MaxBytesError](err); ok {
			return HTTPError{Status: http.StatusRequestEntityTooLarge, Message: "request body too large"}
		}
		return HTTPError{Status: http.StatusBadRequest, Message: "invalid json"}
	}
	return nil
}

func Redirect(w http.ResponseWriter, r *http.Request, url string) {
	if IsHTMX(r) {
		w.Header().Set("HX-Redirect", url)
		w.WriteHeader(http.StatusNoContent)
		return
	}
	http.Redirect(w, r, url, http.StatusSeeOther)
}

func SetToast(w http.ResponseWriter, message string) {
	payload := map[string]map[string]string{"app:toast": {"message": message}}
	body, err := json.Marshal(payload)
	if err == nil {
		w.Header().Set("HX-Trigger", string(body))
	}
}

func Int64Checked(s string, name string) (*int64, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return nil, nil
	}
	n, err := strconv.ParseInt(s, 10, 64)
	if err != nil || n < 1 {
		return nil, HTTPError{Status: http.StatusBadRequest, Message: "invalid " + fieldName(name)}
	}
	return new(n), nil
}

func FormInt64Checked(r *http.Request, name string) (*int64, error) {
	if err := r.ParseForm(); err != nil {
		return nil, HTTPError{Status: http.StatusBadRequest, Message: "invalid form"}
	}
	return Int64Checked(r.FormValue(name), name)
}

func FormFloatValue(r *http.Request, name string) (float64, bool, error) {
	if err := r.ParseForm(); err != nil {
		return 0, false, HTTPError{Status: http.StatusBadRequest, Message: "invalid form"}
	}
	if !r.Form.Has(name) && !r.PostForm.Has(name) {
		return 0, false, nil
	}
	s := strings.TrimSpace(r.FormValue(name))
	if s == "" {
		return 0, true, nil
	}
	f, err := strconv.ParseFloat(s, 64)
	if err != nil {
		return 0, true, HTTPError{Status: http.StatusBadRequest, Message: "invalid number"}
	}
	return f, true, nil
}

func FormFloatRequired(r *http.Request, name string) (float64, error) {
	value, present, err := FormFloatValue(r, name)
	if err != nil {
		return 0, err
	}
	if !present || strings.TrimSpace(r.FormValue(name)) == "" {
		return 0, HTTPError{Status: http.StatusBadRequest, Message: fieldName(name) + " is required"}
	}
	return value, nil
}

// fieldName converts a form field name into the display text used in error messages.
func fieldName(name string) string {
	switch name {
	case "project_id":
		return "project"
	case "subproject_id":
		return "subproject"
	case "total_hours":
		return "total hours"
	}
	return name
}

func Render(w http.ResponseWriter, r *http.Request, status int, component Component) error {
	var body bytes.Buffer
	if err := component.Render(r.Context(), &body); err != nil {
		return err
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.WriteHeader(status)
	_, err := w.Write(body.Bytes())
	if err != nil {
		return ResponseError{Err: err}
	}
	return nil
}

func writeRenderError(w http.ResponseWriter, r *http.Request, err error, fragment bool) {
	if _, ok := errors.AsType[ResponseError](err); ok {
		log.Printf("write response: %v", err)
		return
	}
	if fragment {
		WriteFragmentError(w, r, err)
		return
	}
	WriteError(w, r, err)
}

// RenderPage renders a full page, turning render failures into a response error.
func RenderPage(w http.ResponseWriter, r *http.Request, status int, component Component) {
	if err := Render(w, r, status, component); err != nil {
		writeRenderError(w, r, err, false)
	}
}

// RenderFragment renders an HTMX fragment, turning render failures into an error fragment.
func RenderFragment(w http.ResponseWriter, r *http.Request, status int, component Component) {
	if err := Render(w, r, status, component); err != nil {
		writeRenderError(w, r, err, true)
	}
}
