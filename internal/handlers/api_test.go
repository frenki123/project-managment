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

func TestJSONTaskAndWeek(t *testing.T) {
	q := testkit.Open(t)
	mux := http.NewServeMux()
	handlers.Register(mux, q)

	projBody := []byte(`{"name":"Alpha","purchase_order_name":"PO-1","total_hours":100,"start_date":"2026-09-07","end_date":"2026-10-05"}`)
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, httptest.NewRequest(http.MethodPost, "/api/v1/projects", bytes.NewReader(projBody)))
	if rr.Code != http.StatusCreated {
		t.Fatalf("create project %d %s", rr.Code, rr.Body.String())
	}
	var proj struct {
		ID int64 `json:"id"`
	}
	if err := json.Unmarshal(rr.Body.Bytes(), &proj); err != nil {
		t.Fatal(err)
	}

	taskBody, err := json.Marshal(map[string]any{"name": "Do work", "project_id": proj.ID, "developers": "Ada", "priority": "high"})
	if err != nil {
		t.Fatal(err)
	}
	rr = httptest.NewRecorder()
	mux.ServeHTTP(rr, httptest.NewRequest(http.MethodPost, "/api/v1/tasks", bytes.NewReader(taskBody)))
	if rr.Code != http.StatusCreated {
		t.Fatalf("create task %d %s", rr.Code, rr.Body.String())
	}
	var tk struct {
		ID         int64  `json:"id"`
		Developers string `json:"developers"`
		Priority   string `json:"priority"`
	}
	if err := json.Unmarshal(rr.Body.Bytes(), &tk); err != nil {
		t.Fatal(err)
	}
	if tk.Developers != "Ada" || tk.Priority != "high" {
		t.Fatalf("got %#v", tk)
	}

	weekBody := []byte(`{"planned_hours":8,"spent_hours":3,"progress":25}`)
	rr = httptest.NewRecorder()
	mux.ServeHTTP(rr, httptest.NewRequest(http.MethodPut, "/api/v1/tasks/"+strconv.FormatInt(tk.ID, 10)+"/weeks/2026-09-21", bytes.NewReader(weekBody)))
	if rr.Code != http.StatusOK {
		t.Fatalf("week %d %s", rr.Code, rr.Body.String())
	}

	rr = httptest.NewRecorder()
	mux.ServeHTTP(rr, httptest.NewRequest(http.MethodGet, "/api/v1/tasks/"+strconv.FormatInt(tk.ID, 10), nil))
	if rr.Code != http.StatusOK {
		t.Fatalf("get task %d %s", rr.Code, rr.Body.String())
	}
	var got struct {
		TotalHours float64 `json:"total_hours"`
		SpentHours float64 `json:"spent_hours"`
		Progress   float64 `json:"progress"`
	}
	if err := json.Unmarshal(rr.Body.Bytes(), &got); err != nil {
		t.Fatal(err)
	}
	if got.TotalHours != 8 || got.SpentHours != 3 || got.Progress != 25 {
		t.Fatalf("got %#v", got)
	}

	updatedBody := []byte(`{"name":"Updated work","project_id":` + strconv.FormatInt(proj.ID, 10) + `}`)
	rr = httptest.NewRecorder()
	mux.ServeHTTP(rr, httptest.NewRequest(http.MethodPut, "/api/v1/tasks/"+strconv.FormatInt(tk.ID, 10), bytes.NewReader(updatedBody)))
	if rr.Code != http.StatusOK {
		t.Fatalf("update task %d %s", rr.Code, rr.Body.String())
	}
	var updated struct {
		Name string `json:"name"`
	}
	if err := json.Unmarshal(rr.Body.Bytes(), &updated); err != nil {
		t.Fatal(err)
	}
	if updated.Name != "Updated work" {
		t.Fatalf("updated task name: %q", updated.Name)
	}

	rr = httptest.NewRecorder()
	mux.ServeHTTP(rr, httptest.NewRequest(http.MethodDelete, "/api/v1/tasks/"+strconv.FormatInt(tk.ID, 10), nil))
	if rr.Code != http.StatusNoContent {
		t.Fatalf("delete task %d %s", rr.Code, rr.Body.String())
	}
	rr = httptest.NewRecorder()
	mux.ServeHTTP(rr, httptest.NewRequest(http.MethodGet, "/api/v1/tasks/"+strconv.FormatInt(tk.ID, 10), nil))
	if rr.Code != http.StatusNotFound {
		t.Fatalf("deleted task status: got %d", rr.Code)
	}

	rr = httptest.NewRecorder()
	mux.ServeHTTP(rr, httptest.NewRequest(http.MethodGet, "/api/v1/tasks/999", nil))
	if rr.Code != http.StatusNotFound {
		t.Fatalf("unknown task status: got %d", rr.Code)
	}

	rr = httptest.NewRecorder()
	mux.ServeHTTP(rr, httptest.NewRequest(http.MethodPost, "/api/v1/projects", bytes.NewReader([]byte(`{"name":""}`))))
	if rr.Code != http.StatusBadRequest {
		t.Fatalf("invalid project status: got %d", rr.Code)
	}
	var apiError struct {
		Error string `json:"error"`
	}
	if err := json.Unmarshal(rr.Body.Bytes(), &apiError); err != nil || apiError.Error != "name is required" {
		t.Fatalf("expected JSON error response, got %q", rr.Body.String())
	}

	rr = httptest.NewRecorder()
	mux.ServeHTTP(rr, httptest.NewRequest(http.MethodPost, "/api/v1/tasks", bytes.NewReader([]byte(`{"name":"orphan","project_id":999}`))))
	if rr.Code != http.StatusNotFound {
		t.Fatalf("missing project status: got %d", rr.Code)
	}
}

func TestSubprojectListReturnsDatabaseError(t *testing.T) {
	database := testkit.OpenDatabase(t)
	q := database.Q
	if err := database.Close(); err != nil {
		t.Fatal(err)
	}
	mux := http.NewServeMux()
	handlers.Register(mux, q)
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, httptest.NewRequest(http.MethodGet, "/api/v1/subprojects", nil))
	if rr.Code != http.StatusInternalServerError {
		t.Fatalf("got status %d body %s", rr.Code, rr.Body.String())
	}
}

func TestMonthLockAPIUsesBooleanUnlocked(t *testing.T) {
	q := testkit.Open(t)
	mux := http.NewServeMux()
	handlers.Register(mux, q)
	yearMonth := time.Now().AddDate(0, -1, 0).Format("2006-01")
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, httptest.NewRequest(http.MethodPost, "/api/v1/month-locks/"+yearMonth+"/unlock", nil))
	if rr.Code != http.StatusOK {
		t.Fatalf("unlock status %d: %s", rr.Code, rr.Body.String())
	}
	var got struct {
		Unlocked bool `json:"unlocked"`
	}
	if err := json.Unmarshal(rr.Body.Bytes(), &got); err != nil {
		t.Fatal(err)
	}
	if !got.Unlocked {
		t.Fatalf("expected unlocked=true, got %s", rr.Body.String())
	}
	rr = httptest.NewRecorder()
	mux.ServeHTTP(rr, httptest.NewRequest(http.MethodGet, "/api/v1/month-locks", nil))
	if rr.Code != http.StatusOK {
		t.Fatalf("list status %d: %s", rr.Code, rr.Body.String())
	}
	var list struct {
		Locks []struct {
			Unlocked bool `json:"unlocked"`
		} `json:"month_locks"`
	}
	if err := json.Unmarshal(rr.Body.Bytes(), &list); err != nil {
		t.Fatal(err)
	}
	if len(list.Locks) != 1 || !list.Locks[0].Unlocked {
		t.Fatalf("unexpected list response %s", rr.Body.String())
	}
}

func TestRegisterServesGridRoot(t *testing.T) {
	q := testkit.Open(t)
	mux := http.NewServeMux()
	handlers.Register(mux, q)
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, httptest.NewRequest(http.MethodGet, "/", nil))
	if rr.Code != http.StatusOK || !bytes.Contains(rr.Body.Bytes(), []byte("CAD Development")) {
		t.Fatalf("root response %d: %s", rr.Code, rr.Body.String())
	}
}
