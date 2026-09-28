package main

import (
	"bytes"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"cad-development/internal/client"
)

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
	if got := out.String(); got != `{"error":"HTTP 404: task not found","status":404}`+"\n" {
		t.Fatalf("error output = %q", got)
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
	var out strings.Builder
	state := &commandState{apiURL: server.URL, out: &out, errOut: &out}
	root := newRoot(state)
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
	state := &commandState{apiURL: server.URL, out: &strings.Builder{}, errOut: &strings.Builder{}}
	root := newRoot(state)
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
	state := &commandState{apiURL: server.URL, out: &strings.Builder{}, errOut: &strings.Builder{}}
	root := newRoot(state)
	root.SetArgs([]string{"update-task-week", "4", "2026-09-21", "--progress", "50", "--unlock"})
	if err := root.Execute(); err != nil {
		t.Fatal(err)
	}
	if lockAttempts != 2 {
		t.Fatalf("lock attempts = %d, want 2", lockAttempts)
	}
}
