package handlers_test

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"
	"time"

	"cad-development/internal/app/testkit"
	"cad-development/internal/handlers"
)

func TestTaskListsIncludeCalculatedTotals(t *testing.T) {
	q := testkit.Open(t)
	mux := http.NewServeMux()
	handlers.Register(mux, q)
	start := nextMonday(time.Now())
	pid := createProject(t, mux, "Project", start)
	projectID := strconv.FormatInt(pid, 10)
	ideaID := createTask(t, mux, []byte(`{"name":"Idea"}`))
	plainID := createTask(t, mux, []byte(`{"name":"Untracked","project_id":`+projectID+`}`))

	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, httptest.NewRequest(http.MethodPost, "/api/v1/subprojects", bytes.NewBufferString(`{"name":"Part","project_id":`+projectID+`,"total_hours":10}`)))
	if rr.Code != http.StatusCreated {
		t.Fatalf("create subproject: %d %s", rr.Code, rr.Body.String())
	}
	var sp struct {
		ID int64 `json:"id"`
	}
	if err := json.Unmarshal(rr.Body.Bytes(), &sp); err != nil {
		t.Fatal(err)
	}
	trackedID := createTask(t, mux, []byte(`{"name":"Tracked","project_id":`+projectID+`,"subproject_id":`+strconv.FormatInt(sp.ID, 10)+`}`))
	for i, body := range []string{`{"planned_hours":8,"spent_hours":3,"progress":25}`, `{"planned_hours":2,"spent_hours":4}`} {
		week := start.AddDate(0, 0, i*7).Format("2006-01-02")
		rr = httptest.NewRecorder()
		mux.ServeHTTP(rr, httptest.NewRequest(http.MethodPut, "/api/v1/tasks/"+strconv.FormatInt(trackedID, 10)+"/weeks/"+week, bytes.NewBufferString(body)))
		if rr.Code != http.StatusOK {
			t.Fatalf("save week %s: %d %s", week, rr.Code, rr.Body.String())
		}
	}

	type listedTask struct {
		ID         int64   `json:"id"`
		TotalHours float64 `json:"total_hours"`
		SpentHours float64 `json:"spent_hours"`
		Progress   float64 `json:"progress"`
	}
	check := func(path string, wantIDs ...int64) {
		t.Helper()
		rr := httptest.NewRecorder()
		mux.ServeHTTP(rr, httptest.NewRequest(http.MethodGet, path, nil))
		if rr.Code != http.StatusOK {
			t.Fatalf("list %s: %d %s", path, rr.Code, rr.Body.String())
		}
		var result struct {
			Tasks []listedTask `json:"tasks"`
		}
		if err := json.Unmarshal(rr.Body.Bytes(), &result); err != nil {
			t.Fatal(err)
		}
		if len(result.Tasks) != len(wantIDs) {
			t.Fatalf("list %s: got %s", path, rr.Body.String())
		}
		for _, id := range wantIDs {
			found := false
			for _, tk := range result.Tasks {
				if tk.ID != id {
					continue
				}
				found = true
				if id == trackedID && (tk.TotalHours != 10 || tk.SpentHours != 7 || tk.Progress != 25) {
					t.Fatalf("list %s: wrong totals: %+v", path, tk)
				}
				if id != trackedID && (tk.TotalHours != 0 || tk.SpentHours != 0 || tk.Progress != 0) {
					t.Fatalf("list %s: unexpected totals: %+v", path, tk)
				}
			}
			if !found {
				t.Fatalf("list %s: missing task %d: %s", path, id, rr.Body.String())
			}
		}
	}
	check("/api/v1/tasks", ideaID)
	check("/api/v1/tasks?ideas=true&project_id="+projectID, ideaID)
	check("/api/v1/tasks?project_id="+projectID, trackedID, plainID)
	check("/api/v1/tasks?subproject_id="+strconv.FormatInt(sp.ID, 10), trackedID)
	check("/api/v1/tasks/all", ideaID, trackedID, plainID)
}

func TestTaskListMissingProjectIsNotFound(t *testing.T) {
	q := testkit.Open(t)
	mux := http.NewServeMux()
	handlers.Register(mux, q)
	for _, path := range []string{"/api/v1/tasks?project_id=999", "/api/v1/tasks?project_id=999&ideas=true"} {
		rr := httptest.NewRecorder()
		mux.ServeHTTP(rr, httptest.NewRequest(http.MethodGet, path, nil))
		if rr.Code != http.StatusNotFound {
			t.Fatalf("list %s: %d %s", path, rr.Code, rr.Body.String())
		}
	}
}
