package main

import (
	"bytes"
	json "encoding/json/v2"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"cad-development/internal/client"
	"github.com/spf13/cobra"
)

func testRoot(server *httptest.Server) *cobra.Command {
	state := &commandState{apiURL: server.URL, out: &strings.Builder{}, errOut: &strings.Builder{}}
	return newRoot(state)
}

func TestTableOutputIsBounded(t *testing.T) {
	values := make([]client.Task, 51)
	for i := range values {
		values[i].ID = int64(i + 1)
		values[i].Name = fmt.Sprintf("Task %d", i+1)
	}
	var out strings.Builder
	if err := taskTable(&out, values); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), "... 1 more rows; use JSON output") {
		t.Fatalf("missing truncation message: %s", out.String())
	}
	if strings.Contains(out.String(), "Task 51") {
		t.Fatal("table included rows past the limit")
	}
}

func TestWriteErrorIncludesHTTPStatusAsJSON(t *testing.T) {
	var out bytes.Buffer
	writeError(&out, &client.APIError{Method: http.MethodGet, Path: "/api/v1/tasks/1", Status: http.StatusNotFound, Message: "task not found"})
	var got struct {
		Error  string `json:"error"`
		Status int    `json:"status"`
	}
	if err := json.Unmarshal([]byte(out.String()), &got); err != nil {
		t.Fatal(err)
	}
	if got.Error != "HTTP 404: task not found" || got.Status != http.StatusNotFound {
		t.Fatalf("error output = %q", out.String())
	}
}

func TestUpdateTaskWeekUnlocksUpdatesAndLocks(t *testing.T) {
	var calls []string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls = append(calls, r.Method+" "+r.URL.Path)
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/api/v1/month-locks/unlock":
			_, _ = w.Write([]byte(`{"unlocked":true}`))
		case "/api/v1/tasks/4/weeks/2026-09-21":
			_, _ = w.Write([]byte(`{"task_id":4,"week_start":"2026-09-21","planned_hours":8,"spent_hours":2,"progress":50}`))
		case "/api/v1/month-locks/lock":
			_, _ = w.Write([]byte(`{"unlocked":false}`))
		default:
			t.Fatalf("unexpected request: %s %s", r.Method, r.URL.Path)
		}
	}))
	defer server.Close()
	root := testRoot(server)
	root.SetArgs([]string{"update-task-week", "4", "2026-09-21", "--planned-hours", "8", "--unlock"})
	if err := root.Execute(); err != nil {
		t.Fatal(err)
	}
	want := []string{"POST /api/v1/month-locks/unlock", "PUT /api/v1/tasks/4/weeks/2026-09-21", "POST /api/v1/month-locks/lock"}
	if strings.Join(calls, "\n") != strings.Join(want, "\n") {
		t.Fatalf("calls = %v, want %v", calls, want)
	}
}

func TestUpdateTaskWeekLocksAfterUpdateFailure(t *testing.T) {
	var calls []string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls = append(calls, r.URL.Path)
		w.Header().Set("Content-Type", "application/json")
		if r.URL.Path == "/api/v1/tasks/4/weeks/2026-09-21" {
			w.WriteHeader(http.StatusBadRequest)
			_, _ = w.Write([]byte(`{"error":"invalid hours"}`))
			return
		}
		_, _ = w.Write([]byte(`{"unlocked":false}`))
	}))
	defer server.Close()
	root := testRoot(server)
	root.SetArgs([]string{"update-task-week", "4", "2026-09-21", "--spent-hours", "2", "--unlock"})
	if err := root.Execute(); err == nil {
		t.Fatal("expected update error")
	}
	if len(calls) != 3 || calls[2] != "/api/v1/month-locks/lock" {
		t.Fatalf("calls = %v", calls)
	}
}

func TestUpdateTaskWeekRetriesRelock(t *testing.T) {
	lockAttempts := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/api/v1/month-locks/unlock":
			_, _ = w.Write([]byte(`{"unlocked":true}`))
		case "/api/v1/tasks/4/weeks/2026-09-21":
			_, _ = w.Write([]byte(`{"task_id":4,"week_start":"2026-09-21","planned_hours":8,"spent_hours":2,"progress":50}`))
		case "/api/v1/month-locks/lock":
			lockAttempts++
			if lockAttempts == 1 {
				w.WriteHeader(http.StatusInternalServerError)
				_, _ = w.Write([]byte(`{"error":"busy"}`))
				return
			}
			_, _ = w.Write([]byte(`{"unlocked":false}`))
		default:
			t.Fatalf("unexpected request: %s %s", r.Method, r.URL.Path)
		}
	}))
	defer server.Close()
	root := testRoot(server)
	root.SetArgs([]string{"update-task-week", "4", "2026-09-21", "--progress", "50", "--unlock"})
	if err := root.Execute(); err != nil {
		t.Fatal(err)
	}
	if lockAttempts != 2 {
		t.Fatalf("lock attempts = %d, want 2", lockAttempts)
	}
}

func TestTaskUpdateFlagsPreserveOmittedValues(t *testing.T) {
	var putBody string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if r.Method == http.MethodGet && r.URL.Path == "/api/v1/tasks/4" {
			_, _ = w.Write([]byte(`{"id":4,"name":"Existing","description":"Keep me","implementation_notes":"Keep notes","department":"Engineering","developers":"Alice","priority":"low","project_id":5,"subproject_id":1}`))
			return
		}
		if r.Method == http.MethodPut && r.URL.Path == "/api/v1/tasks/4" {
			data, _ := io.ReadAll(r.Body)
			putBody = string(data)
			_, _ = w.Write([]byte(`{"id":4,"name":"Existing","description":"Keep me","implementation_notes":"Keep notes","department":"Engineering","developers":"Alice","priority":"high","project_id":5,"subproject_id":1}`))
			return
		}
		t.Fatalf("unexpected request: %s %s", r.Method, r.URL.Path)
	}))
	defer server.Close()
	root := testRoot(server)
	root.SetArgs([]string{"tasks", "update", "4", "--priority", "high"})
	if err := root.Execute(); err != nil {
		t.Fatal(err)
	}
	var input client.TaskInput
	if err := json.Unmarshal([]byte(putBody), &input); err != nil {
		t.Fatal(err)
	}
	if input.Name != "Existing" || input.Description != "Keep me" || input.ImplementationNotes != "Keep notes" || input.Priority != "high" {
		t.Fatalf("unexpected task fields: %+v", input)
	}
	if input.ProjectID == nil || *input.ProjectID != 5 || input.SubprojectID == nil || *input.SubprojectID != 1 {
		t.Fatalf("unexpected task assignment: %+v", input)
	}
}

func TestTaskListUsesProjectBeforeIdeasAndProjectID(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/api/v1/projects":
			_, _ = w.Write([]byte(`{"projects":[{"id":5,"name":"Alpha"}]}`))
		case "/api/v1/tasks":
			if r.URL.Query().Get("project_id") != "5" || r.URL.Query().Get("ideas") != "" {
				t.Fatalf("query = %s", r.URL.RawQuery)
			}
			_, _ = w.Write([]byte(`{"tasks":[]}`))
		default:
			t.Fatalf("unexpected request: %s %s", r.Method, r.URL.Path)
		}
	}))
	defer server.Close()
	root := testRoot(server)
	root.SetArgs([]string{"tasks", "list", "--project", "Alpha", "--project-id", "7", "--ideas"})
	if err := root.Execute(); err != nil {
		t.Fatal(err)
	}
}

func TestTaskListDefaultsToAllTasks(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v1/tasks" || r.URL.RawQuery != "" {
			t.Fatalf("request = %s?%s", r.URL.Path, r.URL.RawQuery)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"tasks":[]}`))
	}))
	defer server.Close()
	root := testRoot(server)
	root.SetArgs([]string{"tasks", "list"})
	if err := root.Execute(); err != nil {
		t.Fatal(err)
	}
}

func TestTaskListResolvesSubprojectNameWithoutProject(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/api/v1/subprojects":
			if r.URL.RawQuery != "" {
				t.Fatalf("subproject query = %s", r.URL.RawQuery)
			}
			_, _ = w.Write([]byte(`{"subprojects":[{"id":9,"project_id":3,"name":"Backend"}]}`))
		case "/api/v1/tasks":
			if r.URL.Query().Get("subproject_id") != "9" || r.URL.Query().Get("project_id") != "" || r.URL.Query().Get("ideas") != "" {
				t.Fatalf("task query = %s", r.URL.RawQuery)
			}
			_, _ = w.Write([]byte(`{"tasks":[]}`))
		default:
			t.Fatalf("unexpected request: %s", r.URL.Path)
		}
	}))
	defer server.Close()
	root := testRoot(server)
	root.SetArgs([]string{"tasks", "list", "--subproject", "Backend", "--subproject-id", "7", "--ideas"})
	if err := root.Execute(); err != nil {
		t.Fatal(err)
	}
}

func TestTaskListLeavesInvalidFilterValidationToServer(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("project_id") != "0" {
			t.Fatalf("project_id = %q", r.URL.Query().Get("project_id"))
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte(`{"error":"project not found"}`))
	}))
	defer server.Close()
	root := testRoot(server)
	root.SetArgs([]string{"tasks", "list", "--project-id", "0"})
	err := root.Execute()
	apiErr, ok := err.(*client.APIError)
	if !ok || apiErr.Status != http.StatusBadRequest || apiErr.Message != "project not found" {
		t.Fatalf("error = %#v", err)
	}
}

func TestProjectUpdateSendsFullReplacement(t *testing.T) {
	var body string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPut || r.URL.Path != "/api/v1/projects/7" {
			t.Fatalf("request = %s %s", r.Method, r.URL.Path)
		}
		data, _ := io.ReadAll(r.Body)
		body = string(data)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"id":7,"name":"Alpha","purchase_order_name":"PO-7","total_hours":120,"start_date":"2026-01-05","end_date":"2026-03-30"}`))
	}))
	defer server.Close()
	root := testRoot(server)
	root.SetArgs([]string{"projects", "update", "7", "--name", "Alpha", "--purchase-order-name", "PO-7", "--total-hours", "120", "--start-date", "2026-01-05", "--end-date", "2026-03-30"})
	if err := root.Execute(); err != nil {
		t.Fatal(err)
	}
	var input client.ProjectInput
	if err := json.Unmarshal([]byte(body), &input); err != nil {
		t.Fatal(err)
	}
	if input.Name != "Alpha" || input.PurchaseOrderName != "PO-7" || input.StartDate != "2026-01-05" || input.EndDate != "2026-03-30" {
		t.Fatalf("unexpected project fields: %+v", input)
	}
	if input.TotalHours == nil || *input.TotalHours != 120 {
		t.Fatalf("unexpected project total hours: %+v", input.TotalHours)
	}
}

func TestSubprojectUpdateSendsFullReplacement(t *testing.T) {
	var body string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPut || r.URL.Path != "/api/v1/subprojects/9" {
			t.Fatalf("request = %s %s", r.Method, r.URL.Path)
		}
		data, _ := io.ReadAll(r.Body)
		body = string(data)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"id":9,"project_id":7,"name":"Backend","total_hours":40}`))
	}))
	defer server.Close()
	root := testRoot(server)
	root.SetArgs([]string{"subprojects", "update", "9", "--project-id", "7", "--name", "Backend", "--total-hours", "40"})
	if err := root.Execute(); err != nil {
		t.Fatal(err)
	}
	var input client.SubprojectInput
	if err := json.Unmarshal([]byte(body), &input); err != nil {
		t.Fatal(err)
	}
	if input.ProjectID != 7 || input.Name != "Backend" || input.TotalHours == nil || *input.TotalHours != 40 {
		t.Fatalf("unexpected subproject fields: %+v", input)
	}
}

func TestProjectSCurveUsesProjectIDWithoutResolution(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v1/projects/7/s-curve" {
			t.Fatalf("path = %s", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"project":{"id":7,"name":"Alpha"},"weeks":[]}`))
	}))
	defer server.Close()
	root := testRoot(server)
	root.SetArgs([]string{"project-s-curve", "--project-id", "7"})
	if err := root.Execute(); err != nil {
		t.Fatal(err)
	}
}

func TestTaskListSendsCombinedProjectAndSubprojectIDs(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v1/tasks" || r.URL.Query().Get("project_id") != "5" || r.URL.Query().Get("subproject_id") != "9" || r.URL.Query().Get("ideas") != "" {
			t.Fatalf("request = %s?%s", r.URL.Path, r.URL.RawQuery)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"tasks":[]}`))
	}))
	defer server.Close()
	root := testRoot(server)
	root.SetArgs([]string{"tasks", "list", "--project-id", "5", "--subproject-id", "9", "--ideas"})
	if err := root.Execute(); err != nil {
		t.Fatal(err)
	}
}

func TestProjectSCurveRequiresProject(t *testing.T) {
	var requests int
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests++
	}))
	defer server.Close()
	root := testRoot(server)
	root.SetArgs([]string{"project-s-curve"})
	if err := root.Execute(); err == nil || !strings.Contains(err.Error(), "--project or --project-id is required") {
		t.Fatalf("error = %v", err)
	}
	if requests != 0 {
		t.Fatalf("requests = %d, want 0", requests)
	}
}

func TestProjectSCurveNameTakesPrecedenceOverID(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/api/v1/projects":
			_, _ = w.Write([]byte(`{"projects":[{"id":5,"name":"Alpha"}]}`))
		case "/api/v1/projects/5/s-curve":
			_, _ = w.Write([]byte(`{"project":{"id":5,"name":"Alpha"},"weeks":[]}`))
		default:
			t.Fatalf("unexpected request: %s", r.URL.Path)
		}
	}))
	defer server.Close()
	root := testRoot(server)
	root.SetArgs([]string{"project-s-curve", "--project", "Alpha", "--project-id", "7"})
	if err := root.Execute(); err != nil {
		t.Fatal(err)
	}
}
