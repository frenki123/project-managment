package main

import (
	"bytes"
	json "encoding/json/v2"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"cad-development/internal/client"
	"cad-development/internal/project"
	"cad-development/internal/subproject"
	"cad-development/internal/task"
	"cad-development/internal/weekly"
	"github.com/spf13/cobra"
)

func testRoot(t *testing.T, server *httptest.Server) *cobra.Command {
	t.Helper()
	// Keep CLI setup consistent while each test keeps its protocol visible.
	// The helper itself is intentionally small and only owns command wiring.
	// Test-specific request assertions remain in the individual handlers.
	state := &commandState{apiURL: server.URL, out: &strings.Builder{}, errOut: &strings.Builder{}}
	return newRoot(state)
}

func TestTableOutputIsBounded(t *testing.T) {
	values := make([]task.Task, 51)
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

func TestUpdateTaskWeekUnlocksSingleRequest(t *testing.T) {
	var calls []string
	var body string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls = append(calls, r.Method+" "+r.URL.Path)
		w.Header().Set("Content-Type", "application/json")
		if r.URL.Path != "/api/v1/tasks/4/weeks/2026-09-21" {
			http.Error(w, "unexpected request", http.StatusBadRequest)
			return
		}
		data, _ := io.ReadAll(r.Body)
		body = string(data)
		_, _ = w.Write([]byte(`{"task_id":4,"week_start":"2026-09-21","planned_hours":8,"spent_hours":2,"progress":50}`))
	}))
	defer server.Close()
	root := testRoot(t, server)
	root.SetArgs([]string{"update-task-week", "4", "2026-09-21", "--planned-hours", "8", "--unlock"})
	if err := root.Execute(); err != nil {
		t.Fatal(err)
	}
	want := []string{"PUT /api/v1/tasks/4/weeks/2026-09-21"}
	if strings.Join(calls, "\n") != strings.Join(want, "\n") {
		t.Fatalf("calls = %v, want %v", calls, want)
	}
	var patch weekly.Patch
	if err := json.Unmarshal([]byte(body), &patch); err != nil {
		t.Fatal(err)
	}
	if !patch.Unlock {
		t.Fatalf("unlock flag missing from request: %s", body)
	}
}

func TestUpdateTaskWeekReturnsUpdateFailure(t *testing.T) {
	var calls []string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls = append(calls, r.URL.Path)
		w.Header().Set("Content-Type", "application/json")
		if r.URL.Path == "/api/v1/tasks/4/weeks/2026-09-21" {
			w.WriteHeader(http.StatusBadRequest)
			_, _ = w.Write([]byte(`{"error":"invalid hours"}`))
			return
		}
		http.Error(w, "unexpected request", http.StatusBadRequest)
	}))
	defer server.Close()
	root := testRoot(t, server)
	root.SetArgs([]string{"update-task-week", "4", "2026-09-21", "--spent-hours", "2", "--unlock"})
	if err := root.Execute(); err == nil {
		t.Fatal("expected update error")
	}
	if len(calls) != 1 || calls[0] != "/api/v1/tasks/4/weeks/2026-09-21" {
		t.Fatalf("calls = %v", calls)
	}
}

func TestTaskUpdateReturnsUpdateFailure(t *testing.T) {
	var calls []string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls = append(calls, r.Method+" "+r.URL.Path)
		w.Header().Set("Content-Type", "application/json")
		if r.Method == http.MethodPut && r.URL.Path == "/api/v1/tasks/4" {
			w.WriteHeader(http.StatusBadRequest)
			_, _ = w.Write([]byte(`{"error":"invalid task"}`))
			return
		}
		http.Error(w, "unexpected request", http.StatusBadRequest)
	}))
	defer server.Close()
	root := testRoot(t, server)
	root.SetArgs([]string{"tasks", "update", "4", "--name", "X"})
	if err := root.Execute(); err == nil {
		t.Fatal("expected update error")
	}
	if len(calls) != 1 || calls[0] != "PUT /api/v1/tasks/4" {
		t.Fatalf("calls = %v", calls)
	}
}

func TestTaskUpdateFlagsSendPartialPatch(t *testing.T) {
	var putBody string
	var methods []string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		methods = append(methods, r.Method+" "+r.URL.Path)
		if r.Method == http.MethodPut && r.URL.Path == "/api/v1/tasks/4" {
			data, _ := io.ReadAll(r.Body)
			putBody = string(data)
			_, _ = w.Write([]byte(`{"id":4,"name":"","priority":"high"}`))
			return
		}
		http.Error(w, "unexpected request", http.StatusBadRequest)
	}))
	defer server.Close()
	root := testRoot(t, server)
	root.SetArgs([]string{"tasks", "update", "4", "--priority", "high"})
	if err := root.Execute(); err != nil {
		t.Fatal(err)
	}
	if strings.Join(methods, "\n") != "PUT /api/v1/tasks/4" {
		t.Fatalf("requests = %v", methods)
	}
	if !strings.Contains(putBody, `"priority":"high"`) || strings.Contains(putBody, `"name"`) || strings.Contains(putBody, `"project_id"`) {
		t.Fatalf("unexpected partial patch JSON: %s", putBody)
	}
	var input task.Input
	if err := json.Unmarshal([]byte(putBody), &input); err != nil {
		t.Fatal(err)
	}
	if input.Name != "" || input.Description != "" || input.ImplementationNotes != "" || input.Department != "" || input.Developers != "" || input.Priority != "high" {
		t.Fatalf("unexpected task fields: %+v", input)
	}
	if input.ProjectID != nil || input.SubprojectID != nil {
		t.Fatalf("unexpected task assignment: %+v", input)
	}
}

func TestTaskInputAssignmentOverridesIdeas(t *testing.T) {
	var flags taskFlags
	command := &cobra.Command{}
	flags.addFlags(command)
	if err := command.ParseFlags([]string{"--project-id", "5", "--ideas"}); err != nil {
		t.Fatal(err)
	}
	in := taskInput(command, flags)
	if in.ProjectID == nil || *in.ProjectID != 5 || in.SubprojectID != nil {
		t.Fatalf("unexpected assignment: %+v", in)
	}
}

func TestTaskPatchSupportsNullAndIdeas(t *testing.T) {
	var flags taskFlags
	command := &cobra.Command{}
	flags.addFlags(command)
	if err := command.ParseFlags([]string{"--description", "null", "--ideas"}); err != nil {
		t.Fatal(err)
	}
	patch := taskPatch(command, flags)
	if !patch.Description.Present || patch.Description.Value != nil {
		t.Fatalf("description was not cleared: %#v", patch.Description)
	}
	if !patch.ProjectID.Present || patch.ProjectID.Value != nil || !patch.SubprojectID.Present || patch.SubprojectID.Value != nil {
		t.Fatalf("ideas did not clear assignments: project=%#v subproject=%#v", patch.ProjectID, patch.SubprojectID)
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
				http.Error(w, "unexpected query", http.StatusBadRequest)
				return
			}
			_, _ = w.Write([]byte(`{"tasks":[]}`))
		default:
			http.Error(w, "unexpected request", http.StatusBadRequest)
		}
	}))
	defer server.Close()
	root := testRoot(t, server)
	root.SetArgs([]string{"tasks", "list", "--project", "Alpha", "--project-id", "7", "--ideas"})
	if err := root.Execute(); err != nil {
		t.Fatal(err)
	}
}

func TestTaskListDefaultsToAllTasks(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v1/tasks" || r.URL.RawQuery != "" {
			http.Error(w, "unexpected request", http.StatusBadRequest)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"tasks":[]}`))
	}))
	defer server.Close()
	root := testRoot(t, server)
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
				http.Error(w, "unexpected query", http.StatusBadRequest)
				return
			}
			_, _ = w.Write([]byte(`{"subprojects":[{"id":9,"project_id":3,"name":"Backend"}]}`))
		case "/api/v1/tasks":
			if r.URL.Query().Get("subproject_id") != "9" || r.URL.Query().Get("project_id") != "" || r.URL.Query().Get("ideas") != "" {
				http.Error(w, "unexpected query", http.StatusBadRequest)
				return
			}
			_, _ = w.Write([]byte(`{"tasks":[]}`))
		default:
			http.Error(w, "unexpected request", http.StatusBadRequest)
		}
	}))
	defer server.Close()
	root := testRoot(t, server)
	root.SetArgs([]string{"tasks", "list", "--subproject", "Backend", "--subproject-id", "7", "--ideas"})
	if err := root.Execute(); err != nil {
		t.Fatal(err)
	}
}

func TestTaskListLeavesInvalidFilterValidationToServer(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("project_id") != "0" {
			http.Error(w, "unexpected project id", http.StatusBadRequest)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte(`{"error":"project not found"}`))
	}))
	defer server.Close()
	root := testRoot(t, server)
	root.SetArgs([]string{"tasks", "list", "--project-id", "0"})
	err := root.Execute()
	apiErr, ok := errors.AsType[*client.APIError](err)
	if !ok || apiErr.Status != http.StatusBadRequest || apiErr.Message != "project not found" {
		t.Fatalf("error = %#v", err)
	}
}

func TestProjectUpdateSendsFullReplacement(t *testing.T) {
	var body string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPut || r.URL.Path != "/api/v1/projects/7" {
			http.Error(w, "unexpected request", http.StatusBadRequest)
			return
		}
		data, _ := io.ReadAll(r.Body)
		body = string(data)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"id":7,"name":"Alpha","purchase_order_name":"PO-7","total_hours":120,"start_date":"2026-01-05","end_date":"2026-03-30"}`))
	}))
	defer server.Close()
	root := testRoot(t, server)
	root.SetArgs([]string{"projects", "update", "7", "--name", "Alpha", "--purchase-order-name", "PO-7", "--total-hours", "120", "--start-date", "2026-01-05", "--end-date", "2026-03-30"})
	if err := root.Execute(); err != nil {
		t.Fatal(err)
	}
	var input project.Input
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
			http.Error(w, "unexpected request", http.StatusBadRequest)
			return
		}
		data, _ := io.ReadAll(r.Body)
		body = string(data)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"id":9,"project_id":7,"name":"Backend","total_hours":40}`))
	}))
	defer server.Close()
	root := testRoot(t, server)
	root.SetArgs([]string{"subprojects", "update", "9", "--project-id", "7", "--name", "Backend", "--total-hours", "40"})
	if err := root.Execute(); err != nil {
		t.Fatal(err)
	}
	var input subproject.Input
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
			http.Error(w, "unexpected path", http.StatusBadRequest)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"project":{"id":7,"name":"Alpha"},"weeks":[]}`))
	}))
	defer server.Close()
	root := testRoot(t, server)
	root.SetArgs([]string{"project-s-curve", "--project-id", "7"})
	if err := root.Execute(); err != nil {
		t.Fatal(err)
	}
}

func TestTaskListSendsCombinedProjectAndSubprojectIDs(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v1/tasks" || r.URL.Query().Get("project_id") != "5" || r.URL.Query().Get("subproject_id") != "9" || r.URL.Query().Get("ideas") != "" {
			http.Error(w, "unexpected request", http.StatusBadRequest)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"tasks":[]}`))
	}))
	defer server.Close()
	root := testRoot(t, server)
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
	root := testRoot(t, server)
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
			http.Error(w, "unexpected request", http.StatusBadRequest)
		}
	}))
	defer server.Close()
	root := testRoot(t, server)
	root.SetArgs([]string{"project-s-curve", "--project", "Alpha", "--project-id", "7"})
	if err := root.Execute(); err != nil {
		t.Fatal(err)
	}
}
