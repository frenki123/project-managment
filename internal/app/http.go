package app

import (
	"bytes"
	"context"
	json "encoding/json/v2"
	"errors"
	"io"
	"net/http"
	"strconv"
	"strings"
)

type Component interface {
	Render(context.Context, io.Writer) error
}

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

func JSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	if v == nil {
		return
	}
	_ = json.MarshalWrite(w, v)
}

func DecodeJSON(w http.ResponseWriter, r *http.Request, v any) error {
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
	http.Redirect(w, r, url, http.StatusSeeOther)
}

func FormInt64Checked(r *http.Request, name string) (*int64, error) {
	s := strings.TrimSpace(r.FormValue(name))
	if s == "" {
		return nil, nil
	}
	n, err := strconv.ParseInt(s, 10, 64)
	if err != nil || n < 1 {
		return nil, HTTPError{Status: http.StatusBadRequest, Message: "invalid " + name}
	}
	return new(n), nil
}

func FormFloat(r *http.Request, name string) (*float64, error) {
	if err := r.ParseForm(); err != nil {
		return nil, HTTPError{Status: http.StatusBadRequest, Message: "invalid form"}
	}
	if !r.Form.Has(name) && !r.PostForm.Has(name) {
		return nil, nil
	}
	s := strings.TrimSpace(r.FormValue(name))
	if s == "" {
		return nil, nil
	}
	f, err := strconv.ParseFloat(s, 64)
	if err != nil {
		return nil, HTTPError{Status: http.StatusBadRequest, Message: "invalid number"}
	}
	return new(f), nil
}

func FormFloatRequired(r *http.Request, name string) (float64, error) {
	value, err := FormFloat(r, name)
	if err != nil {
		return 0, err
	}
	if value == nil {
		return 0, HTTPError{Status: http.StatusBadRequest, Message: name + " is required"}
	}
	return *value, nil
}

func Render(w http.ResponseWriter, r *http.Request, status int, component Component) error {
	var body bytes.Buffer
	if err := component.Render(r.Context(), &body); err != nil {
		return err
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(status)
	_, err := w.Write(body.Bytes())
	return err
}
