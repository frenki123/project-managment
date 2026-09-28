package client

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func newTestClient(t *testing.T, handler http.Handler) *Client {
	t.Helper()
	server := httptest.NewServer(handler)
	t.Cleanup(server.Close)
	c, err := New(server.URL)
	if err != nil {
		t.Fatal(err)
	}
	return c
}

func TestTasksEncodesFilters(t *testing.T) {
	c := newTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v1/tasks" {
			http.Error(w, "unexpected path", http.StatusBadRequest)
			return
		}
		if got := r.URL.Query().Get("project_id"); got != "7" {
			http.Error(w, "unexpected project id", http.StatusBadRequest)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"tasks":[]}`))
	}))
	projectID := int64(7)
	if _, err := c.Tasks(context.Background(), false, &projectID, nil); err != nil {
		t.Fatal(err)
	}
}

func TestAPIErrorPreservesStatusAndMessage(t *testing.T) {
	c := newTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusConflict)
		_, _ = w.Write([]byte(`{"error":"history is locked"}`))
	}))
	err := c.Do(context.Background(), http.MethodGet, "/api/v1/tasks/1", nil, nil)
	apiErr, ok := errors.AsType[*APIError](err)
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
			c := newTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusBadGateway)
				_, _ = w.Write([]byte(tc.body))
			}))
			err := c.Do(context.Background(), http.MethodGet, "/api/v1/tasks", nil, nil)
			apiErr, ok := errors.AsType[*APIError](err)
			if !ok || apiErr.Status != http.StatusBadGateway || apiErr.Message != tc.want {
				t.Fatalf("error = %#v", err)
			}
		})
	}
}

func TestAPIErrorBodyIsBounded(t *testing.T) {
	c := newTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadGateway)
		_, _ = w.Write([]byte(strings.Repeat("x", maxAPIErrorBody+1024)))
	}))
	err := c.Do(context.Background(), http.MethodGet, "/api/v1/tasks", nil, nil)
	apiErr, ok := errors.AsType[*APIError](err)
	if !ok || len(apiErr.Message) != maxAPIErrorBody+3 || !strings.HasSuffix(apiErr.Message, "...") {
		t.Fatalf("error = %#v", err)
	}
}

func TestTasksDecodeLargeSuccessfulResponse(t *testing.T) {
	item := `{"id":1,"name":"Task"}`
	body := `{"tasks":[` + strings.Repeat(item+",", 10000) + item + `]}`
	c := newTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(body))
	}))
	var err error
	result, err := c.Tasks(context.Background(), false, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Tasks) != 10001 {
		t.Fatalf("tasks = %d, want 10001", len(result.Tasks))
	}
}

func TestEmptySuccessfulResponseIsAccepted(t *testing.T) {
	c := newTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
	if err := c.Do(context.Background(), http.MethodGet, "/api/v1/tasks", nil, new(TasksResponse)); err != nil {
		t.Fatal(err)
	}
}
