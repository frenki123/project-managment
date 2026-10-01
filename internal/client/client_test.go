package client

import (
	"context"
	json "encoding/json/v2"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"cad-development/internal/nullable"
	"cad-development/internal/task"
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

func TestTaskPatchOmitsAbsentAndWritesExplicitNull(t *testing.T) {
	data, err := json.Marshal(task.Patch{
		Priority:  nullable.Present("high"),
		ProjectID: nullable.Clear[*int64](),
	})
	if err != nil {
		t.Fatal(err)
	}
	got := string(data)
	if !strings.Contains(got, `"priority":"high"`) || !strings.Contains(got, `"project_id":null`) || strings.Contains(got, `"name"`) {
		t.Fatalf("patch JSON = %s", got)
	}
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
		_, _ = w.Write([]byte(`{"error":"historical editing is not enabled","reason":"week-outside-project-bounds"}`))
	}))
	err := c.DoNoBody(context.Background(), http.MethodGet, "/api/v1/tasks/1", (*struct{})(nil))
	apiErr, ok := errors.AsType[*APIError](err)
	if !ok || apiErr.Status != http.StatusConflict || apiErr.Message != "historical editing is not enabled" || apiErr.Reason != "week-outside-project-bounds" {
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
			err := c.DoNoBody(context.Background(), http.MethodGet, "/api/v1/tasks", (*struct{})(nil))
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
	err := c.DoNoBody(context.Background(), http.MethodGet, "/api/v1/tasks", (*struct{})(nil))
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
	if err := c.DoNoBody(context.Background(), http.MethodGet, "/api/v1/tasks", new(task.TasksResponse)); err != nil {
		t.Fatal(err)
	}
}
