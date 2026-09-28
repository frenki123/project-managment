package client

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestTasksEncodesFilters(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v1/tasks" {
			t.Fatalf("path = %q", r.URL.Path)
		}
		if got := r.URL.Query().Get("project_id"); got != "7" {
			t.Fatalf("project_id = %q", got)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"tasks":[]}`))
	}))
	defer server.Close()
	c, err := New(server.URL)
	if err != nil {
		t.Fatal(err)
	}
	projectID := int64(7)
	if _, err := c.Tasks(context.Background(), false, &projectID, nil); err != nil {
		t.Fatal(err)
	}
}

func TestAPIErrorPreservesStatusAndMessage(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusConflict)
		_, _ = w.Write([]byte(`{"error":"history is locked"}`))
	}))
	defer server.Close()
	c, err := New(server.URL)
	if err != nil {
		t.Fatal(err)
	}
	err = c.Do(context.Background(), http.MethodGet, "/api/v1/tasks/1", nil, nil)
	apiErr, ok := err.(*APIError)
	if !ok || apiErr.Status != http.StatusConflict || apiErr.Message != "history is locked" {
		t.Fatalf("error = %#v", err)
	}
}

func TestAPIErrorFallsBackToResponseBody(t *testing.T) {
	cases := []struct {
		name string
		body string
		want string
	}{
		{name: "plain text", body: "upstream unavailable", want: "upstream unavailable"},
		{name: "malformed json", body: `{`, want: "{"},
		{name: "empty", want: ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusBadGateway)
				_, _ = w.Write([]byte(tc.body))
			}))
			defer server.Close()
			c, err := New(server.URL)
			if err != nil {
				t.Fatal(err)
			}
			err = c.Do(context.Background(), http.MethodGet, "/api/v1/tasks", nil, nil)
			apiErr, ok := err.(*APIError)
			if !ok || apiErr.Status != http.StatusBadGateway || apiErr.Message != tc.want {
				t.Fatalf("error = %#v", err)
			}
		})
	}
}

func TestAPIErrorBodyIsBounded(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadGateway)
		_, _ = w.Write([]byte(strings.Repeat("x", maxAPIErrorBody+1024)))
	}))
	defer server.Close()
	c, err := New(server.URL)
	if err != nil {
		t.Fatal(err)
	}
	err = c.Do(context.Background(), http.MethodGet, "/api/v1/tasks", nil, nil)
	apiErr, ok := err.(*APIError)
	if !ok || len(apiErr.Message) != maxAPIErrorBody+3 || !strings.HasSuffix(apiErr.Message, "...") {
		t.Fatalf("error = %#v", err)
	}
}
