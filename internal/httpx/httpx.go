package httpx

import (
	"encoding/json"
	"errors"
	"io"
	"log"
	"net/http"
	"strconv"
	"strings"

	"cad-development/internal/apperr"
)

func IsAPI(r *http.Request) bool {
	return strings.HasPrefix(r.URL.Path, "/api/")
}

func IsHTMX(r *http.Request) bool {
	return r.Header.Get("HX-Request") != ""
}

func PathID(r *http.Request, key string) (int64, error) {
	id, err := strconv.ParseInt(r.PathValue(key), 10, 64)
	if err != nil || id < 1 {
		return 0, apperr.New(http.StatusBadRequest, "invalid id")
	}
	return id, nil
}

func JSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	if v == nil {
		return
	}
	_ = json.NewEncoder(w).Encode(v)
}

func DecodeJSON(r *http.Request, v any) error {
	defer r.Body.Close()
	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()
	if err := dec.Decode(v); err != nil {
		return apperr.New(http.StatusBadRequest, "invalid json")
	}
	var extra any
	if err := dec.Decode(&extra); err != io.EOF {
		return apperr.New(http.StatusBadRequest, "request must contain one json object")
	}
	return nil
}

func Error(w http.ResponseWriter, r *http.Request, err error) {
	var ae apperr.Error
	if errors.As(err, &ae) {
		if IsAPI(r) {
			JSON(w, ae.Status, map[string]string{"error": ae.Message})
			return
		}
		http.Error(w, ae.Message, ae.Status)
		return
	}
	log.Println(err)
	if IsAPI(r) {
		JSON(w, http.StatusInternalServerError, map[string]string{"error": "internal error"})
		return
	}
	http.Error(w, "internal error", http.StatusInternalServerError)
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
		return nil, apperr.New(http.StatusBadRequest, "invalid "+name)
	}
	return &n, nil
}

func FormFloat(r *http.Request, name string) (*float64, error) {
	if err := r.ParseForm(); err != nil {
		return nil, apperr.New(http.StatusBadRequest, "invalid form")
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
		return nil, apperr.New(http.StatusBadRequest, "invalid number")
	}
	return &f, nil
}

func FormFloatRequired(r *http.Request, name string) (float64, error) {
	value, err := FormFloat(r, name)
	if err != nil {
		return 0, err
	}
	if value == nil {
		return 0, apperr.New(http.StatusBadRequest, name+" is required")
	}
	return *value, nil
}

func HTML(w http.ResponseWriter, status int) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(status)
}
