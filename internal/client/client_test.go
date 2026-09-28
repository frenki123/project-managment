package client

import (
	"context"
	"net/http"
	"net/http/httptest"
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
