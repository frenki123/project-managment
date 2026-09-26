package handlers_test

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"

	"cad-development/internal/dbtest"
	"cad-development/internal/handlers"
)

func TestJSONTaskAndWeek(t *testing.T) {
	_, q := dbtest.Open(t)
	mux := http.NewServeMux()
	handlers.Register(mux, q, t.TempDir())

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

	rr = httptest.NewRecorder()
	mux.ServeHTTP(rr, httptest.NewRequest(http.MethodGet, "/api/v1/tasks/999", nil))
	if rr.Code != http.StatusNotFound {
		t.Fatalf("unknown task status: got %d", rr.Code)
	}
}
