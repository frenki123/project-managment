package handlers_test

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strconv"
	"strings"
	"testing"
	"time"

	"cad-development/internal/db/testkit"
	"cad-development/internal/handlers"
	"cad-development/internal/weekly"
)

func TestTaskListsIncludeCalculatedTotals(t *testing.T) {
	q := testkit.Open(t)
	mux := http.NewServeMux()
	handlers.Register(mux, q)
	start := weekly.MondayOnOrBefore(time.Now().AddDate(0, 0, 7))
	pid := createProject(t, mux, "Project", start)
	projectID := strconv.FormatInt(pid, 10)
	ideaID := createResource(t, mux, "/api/v1/tasks", "task", []byte(`{"name":"Idea"}`))
	plainID := createResource(t, mux, "/api/v1/tasks", "task", []byte(`{"name":"Untracked","project_id":`+projectID+`}`))

	spBody := []byte(`{"name":"Part","project_id":` + projectID + `,"total_hours":10}`)
	spID := createResource(t, mux, "/api/v1/subprojects", "subproject", spBody)
	trackedID := createResource(t, mux, "/api/v1/tasks", "task", []byte(`{"name":"Tracked","project_id":`+projectID+`,"subproject_id":`+strconv.FormatInt(spID, 10)+`}`))
	for i, body := range []string{`{"planned_hours":8,"spent_hours":3,"progress":25}`, `{"planned_hours":2,"spent_hours":4}`} {
		week := start.AddDate(0, 0, i*7).Format("2006-01-02")
		rr := httptest.NewRecorder()
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
	check("/api/v1/tasks", ideaID, trackedID, plainID)
	check("/api/v1/tasks?ideas=true", ideaID)
	check("/api/v1/tasks?project_id="+projectID, trackedID, plainID)
	check("/api/v1/tasks?subproject_id="+strconv.FormatInt(spID, 10), trackedID)
	check("/api/v1/tasks?ideas=true&project_id="+projectID, trackedID, plainID)
	check("/api/v1/tasks?ideas=true&project_id="+projectID+"&subproject_id="+strconv.FormatInt(spID, 10), trackedID)
}

func TestTaskListMissingProjectIsNotFound(t *testing.T) {
	q := testkit.Open(t)
	mux := http.NewServeMux()
	handlers.Register(mux, q)
	for _, path := range []string{"/api/v1/tasks?project_id=999"} {
		rr := httptest.NewRecorder()
		mux.ServeHTTP(rr, httptest.NewRequest(http.MethodGet, path, nil))
		if rr.Code != http.StatusNotFound {
			t.Fatalf("list %s: %d %s", path, rr.Code, rr.Body.String())
		}
	}
}

func TestTaskListRejectsIdeasWithSubproject(t *testing.T) {
	q := testkit.Open(t)
	mux := http.NewServeMux()
	handlers.Register(mux, q)
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, httptest.NewRequest(http.MethodGet, "/api/v1/tasks?ideas=true&subproject_id=5", nil))
	if rr.Code != http.StatusBadRequest {
		t.Fatalf("ideas with subproject should be 400: %d %s", rr.Code, rr.Body.String())
	}
}

func TestTaskFormManualStatusRoundTrip(t *testing.T) {
	q := testkit.Open(t)
	mux := http.NewServeMux()
	handlers.Register(mux, q)

	start := weekly.MondayOnOrBefore(time.Now().AddDate(0, 0, 7))
	projID := createProject(t, mux, "Alpha", start)
	projectID := strconv.FormatInt(projID, 10)
	taskID := createResource(t, mux, "/api/v1/tasks", "task", []byte(`{"name":"Staged","project_id":`+projectID+`,"manual_status":"In review"}`))
	idStr := strconv.FormatInt(taskID, 10)

	r := httptest.NewRequest(http.MethodGet, "/tasks/"+idStr+"/edit", nil)
	r.Header.Set("HX-Request", "true")
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, r)
	if rr.Code != http.StatusOK || !strings.Contains(rr.Body.String(), `name="manual_status"`) || !strings.Contains(rr.Body.String(), `value="In review" selected`) {
		t.Fatalf("stage select must preselect the manual stage: %d %s", rr.Code, rr.Body.String())
	}

	values := url.Values{
		"name":                 {"Staged"},
		"description":          {""},
		"implementation_notes": {""},
		"department":           {""},
		"developers":           {""},
		"priority":             {""},
		"project_id":           {projectID},
		"subproject_id":        {""},
		"manual_status":        {"In review"},
	}
	r = httptest.NewRequest(http.MethodPost, "/tasks/"+idStr, strings.NewReader(values.Encode()))
	r.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rr = httptest.NewRecorder()
	mux.ServeHTTP(rr, r)
	if rr.Code != http.StatusSeeOther {
		t.Fatalf("task update status: %d %s", rr.Code, rr.Body.String())
	}

	rr = httptest.NewRecorder()
	mux.ServeHTTP(rr, httptest.NewRequest(http.MethodGet, "/api/v1/tasks/"+idStr, nil))
	if rr.Code != http.StatusOK || !strings.Contains(rr.Body.String(), `"manual_status":"In review"`) || !strings.Contains(rr.Body.String(), `"status":"In review"`) {
		t.Fatalf("form POST did not persist the manual pin: %d %s", rr.Code, rr.Body.String())
	}
}

func TestTaskFormKeepsStageMissingFromStagesTable(t *testing.T) {
	database := testkit.OpenDatabase(t)
	mux := http.NewServeMux()
	handlers.Register(mux, database.Q)

	if _, err := database.Conn.Exec("UPDATE stages SET name = 'Under review' WHERE name = 'In review'"); err != nil {
		t.Fatal(err)
	}
	start := weekly.MondayOnOrBefore(time.Now().AddDate(0, 0, 7))
	projID := createProject(t, mux, "Alpha", start)
	taskID := createResource(t, mux, "/api/v1/tasks", "task", []byte(`{"name":"Staged","project_id":`+strconv.FormatInt(projID, 10)+`,"manual_status":"In review"}`))

	r := httptest.NewRequest(http.MethodGet, "/tasks/"+strconv.FormatInt(taskID, 10)+"/edit", nil)
	r.Header.Set("HX-Request", "true")
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, r)
	if rr.Code != http.StatusOK {
		t.Fatalf("edit fragment: %d %s", rr.Code, rr.Body.String())
	}
	if !strings.Contains(rr.Body.String(), `value="In review" selected`) || !strings.Contains(rr.Body.String(), `value="Under review"`) {
		t.Fatalf("stage select must preserve a manual value that is no longer a stage: %s", rr.Body.String())
	}
}
