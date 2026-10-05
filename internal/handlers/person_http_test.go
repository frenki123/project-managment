package handlers_test

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"

	"cad-development/internal/db/testkit"
	"cad-development/internal/handlers"
)

func TestJSONPeopleCreate(t *testing.T) {
	q := testkit.Open(t)
	mux := http.NewServeMux()
	handlers.Register(mux, q)

	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, httptest.NewRequest(http.MethodPost, "/api/v1/people", bytes.NewReader([]byte(`{"name":"Ada"}`))))
	if rr.Code != http.StatusCreated {
		t.Fatalf("create default: %d %s", rr.Code, rr.Body.String())
	}
	var created struct {
		ID             int64   `json:"id"`
		WeeklyCapacity float64 `json:"weekly_capacity"`
	}
	if err := json.Unmarshal(rr.Body.Bytes(), &created); err != nil {
		t.Fatal(err)
	}
	if created.WeeklyCapacity != 40 {
		t.Fatalf("default capacity = %v, want 40", created.WeeklyCapacity)
	}

	rr = httptest.NewRecorder()
	mux.ServeHTTP(rr, httptest.NewRequest(http.MethodPost, "/api/v1/people", bytes.NewReader([]byte(`{"name":"Grace","weekly_capacity":32}`))))
	if rr.Code != http.StatusCreated {
		t.Fatalf("create custom: %d %s", rr.Code, rr.Body.String())
	}
	if err := json.Unmarshal(rr.Body.Bytes(), &created); err != nil {
		t.Fatal(err)
	}
	if created.WeeklyCapacity != 32 {
		t.Fatalf("custom capacity = %v, want 32", created.WeeklyCapacity)
	}
}

func TestJSONDuplicatePersonNameConflict(t *testing.T) {
	q := testkit.Open(t)
	mux := http.NewServeMux()
	handlers.Register(mux, q)
	createPerson(t, mux, "Alpha")
	second := createPerson(t, mux, "Beta")
	for _, request := range []struct {
		method string
		path   string
	}{
		{http.MethodPost, "/api/v1/people"},
		{http.MethodPut, "/api/v1/people/" + strconv.FormatInt(second, 10)},
	} {
		rr := httptest.NewRecorder()
		mux.ServeHTTP(rr, httptest.NewRequest(request.method, request.path, bytes.NewReader([]byte(`{"name":"alpha"}`))))
		assertAPIError(t, rr, http.StatusConflict, "person name already exists", "person-name-taken")
	}
}

func TestPeopleHTMLFlow(t *testing.T) {
	q := testkit.Open(t)
	mux := http.NewServeMux()
	handlers.Register(mux, q)

	for _, path := range []string{"/people", "/people/new"} {
		rr := httptest.NewRecorder()
		mux.ServeHTTP(rr, httptest.NewRequest(http.MethodGet, path, nil))
		if rr.Code != http.StatusOK {
			t.Fatalf("GET %s: %d %s", path, rr.Code, rr.Body.String())
		}
	}

	rr := postForm(t, mux, "/people", "name=Ada&weekly_capacity=32")
	if rr.Code != http.StatusSeeOther || rr.Header().Get("Location") != "/people" {
		t.Fatalf("create person form: %d %s", rr.Code, rr.Header().Get("Location"))
	}
	id := createPerson(t, mux, "Grace")

	rr = postForm(t, mux, "/people/"+strconv.FormatInt(id, 10), "name=Grace&weekly_capacity=20")
	if rr.Code != http.StatusSeeOther || rr.Header().Get("Location") != "/people" {
		t.Fatalf("update person form: %d %s", rr.Code, rr.Header().Get("Location"))
	}

	req := httptest.NewRequest(http.MethodGet, "/people/"+strconv.FormatInt(id, 10)+"/edit", nil)
	req.Header.Set("HX-Request", "true")
	rr = httptest.NewRecorder()
	mux.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK || !bytes.Contains(rr.Body.Bytes(), []byte("Grace")) {
		t.Fatalf("edit modal: %d %s", rr.Code, rr.Body.String())
	}
}

func TestPeopleUpdateErrorKeepsSubmittedInput(t *testing.T) {
	q := testkit.Open(t)
	mux := http.NewServeMux()
	handlers.Register(mux, q)
	createPerson(t, mux, "Alpha")
	beta := createPerson(t, mux, "Beta")

	r := httptest.NewRequest(http.MethodPost, "/people/"+strconv.FormatInt(beta, 10), bytes.NewBufferString("name=alpha&weekly_capacity=99"))
	r.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	r.Header.Set("HX-Request", "true")
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, r)
	if rr.Code != http.StatusConflict {
		t.Fatalf("expected 409, got %d %s", rr.Code, rr.Body.String())
	}
	if !bytes.Contains(rr.Body.Bytes(), []byte(`value="alpha"`)) || !bytes.Contains(rr.Body.Bytes(), []byte(`value="99"`)) {
		t.Fatalf("error form did not preserve submitted input: %s", rr.Body.String())
	}
}

func createPerson(t *testing.T, mux *http.ServeMux, name string) int64 {
	t.Helper()
	return createResource(t, mux, "/api/v1/people", "person", []byte(`{"name":"`+name+`"}`))
}