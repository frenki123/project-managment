package main

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

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
