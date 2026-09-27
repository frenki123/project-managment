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

	"cad-development/internal/app/testkit"
	"cad-development/internal/handlers"
)

func nextMonday(now time.Time) time.Time {
	day := now.AddDate(0, 0, 1)
	offset := (time.Monday - day.Weekday() + 7) % 7
	return day.AddDate(0, 0, int(offset))
}

func createProject(t *testing.T, mux *http.ServeMux, name string, start time.Time) int64 {
	t.Helper()
	body := []byte(`{"name":"` + name + `","total_hours":100,"start_date":"` + start.Format("2006-01-02") + `","end_date":"` + start.AddDate(0, 0, 28).Format("2006-01-02") + `"}`)
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, httptest.NewRequest(http.MethodPost, "/api/v1/projects", bytes.NewReader(body)))
	if rr.Code != http.StatusCreated {
		t.Fatalf("create project %s: %d %s", name, rr.Code, rr.Body.String())
	}
	var proj struct {
		ID int64 `json:"id"`
	}
	if err := json.Unmarshal(rr.Body.Bytes(), &proj); err != nil {
		t.Fatal(err)
	}
	return proj.ID
}

func createTask(t *testing.T, mux *http.ServeMux, body []byte) int64 {
	t.Helper()
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, httptest.NewRequest(http.MethodPost, "/api/v1/tasks", bytes.NewReader(body)))
	if rr.Code != http.StatusCreated {
		t.Fatalf("create task: %d %s", rr.Code, rr.Body.String())
	}
	var tk struct {
		ID int64 `json:"id"`
	}
	if err := json.Unmarshal(rr.Body.Bytes(), &tk); err != nil {
		t.Fatal(err)
	}
	return tk.ID
}

func TestJSONTaskAndWeek(t *testing.T) {
	q := testkit.Open(t)
	mux := http.NewServeMux()
	handlers.Register(mux, q)

	start := nextMonday(time.Now())
	projID := createProject(t, mux, "Alpha", start)

	// Keep this map to pin the request wire shape independently of domain types.
	taskBody, err := json.Marshal(map[string]any{"name": "Do work", "project_id": projID, "developers": "Ada", "priority": "high"})
	if err != nil {
		t.Fatal(err)
	}
	rr := httptest.NewRecorder()
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
	mux.ServeHTTP(rr, httptest.NewRequest(http.MethodPut, "/api/v1/tasks/"+strconv.FormatInt(tk.ID, 10)+"/weeks/"+start.Format("2006-01-02"), bytes.NewReader(weekBody)))
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

	updatedBody := []byte(`{"name":"Updated work","project_id":` + strconv.FormatInt(projID, 10) + `}`)
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
	if rr.Code != http.StatusConflict {
		t.Fatalf("delete task with history %d %s", rr.Code, rr.Body.String())
	}
	rr = httptest.NewRecorder()
	mux.ServeHTTP(rr, httptest.NewRequest(http.MethodGet, "/api/v1/tasks/"+strconv.FormatInt(tk.ID, 10), nil))
	if rr.Code != http.StatusOK {
		t.Fatalf("protected task status: got %d", rr.Code)
	}

	rr = httptest.NewRecorder()
	mux.ServeHTTP(rr, httptest.NewRequest(http.MethodGet, "/api/v1/tasks/999", nil))
	if rr.Code != http.StatusNotFound {
		t.Fatalf("unknown task status: got %d", rr.Code)
	}

}

func TestHTMLWeekEditPersists(t *testing.T) {
	q := testkit.Open(t)
	mux := http.NewServeMux()
	handlers.Register(mux, q)

	start := nextMonday(time.Now())
	projID := createProject(t, mux, "Alpha", start)
	tk := struct {
		ID int64 `json:"id"`
	}{ID: createTask(t, mux, []byte(`{"name":"Grid task","project_id":`+strconv.FormatInt(projID, 10)+`}`))}

	body := "planned_hours=8&spent_hours=3&progress=25&project=" + strconv.FormatInt(projID, 10)
	r := httptest.NewRequest(http.MethodPost, "/tasks/"+strconv.FormatInt(tk.ID, 10)+"/weeks/"+start.Format("2006-01-02"), bytes.NewBufferString(body))
	r.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	r.Header.Set("HX-Request", "true")
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, r)
	if rr.Code != http.StatusOK {
		t.Fatalf("week edit %d %s", rr.Code, rr.Body.String())
	}
	if !bytes.Contains(rr.Body.Bytes(), []byte("task-row-"+strconv.FormatInt(tk.ID, 10))) || bytes.Contains(rr.Body.Bytes(), []byte("<!DOCTYPE html>")) {
		t.Fatalf("expected row fragment, got %q", rr.Body.String())
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
		t.Fatalf("week edit not persisted: %#v", got)
	}
}

func TestHTMLTaskEditAndProjectDeleteFragments(t *testing.T) {
	q := testkit.Open(t)
	mux := http.NewServeMux()
	handlers.Register(mux, q)

	start := nextMonday(time.Now())
	projectID := createProject(t, mux, "Alpha", start)
	taskID := createTask(t, mux, []byte(`{"name":"Original task","project_id":`+strconv.FormatInt(projectID, 10)+`}`))

	r := httptest.NewRequest(http.MethodGet, "/tasks/"+strconv.FormatInt(taskID, 10)+"/edit", nil)
	r.Header.Set("HX-Request", "true")
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, r)
	if rr.Code != http.StatusOK || !strings.Contains(rr.Body.String(), `name="name"`) || !strings.Contains(rr.Body.String(), `value="Original task"`) {
		t.Fatalf("expected task edit fragment with name, got %d %s", rr.Code, rr.Body.String())
	}

	values := url.Values{
		"name":                 {"Renamed task"},
		"description":          {""},
		"implementation_notes": {""},
		"department":           {""},
		"developers":           {""},
		"priority":             {""},
		"project_id":           {strconv.FormatInt(projectID, 10)},
		"subproject_id":        {""},
	}
	r = httptest.NewRequest(http.MethodPost, "/tasks/"+strconv.FormatInt(taskID, 10), strings.NewReader(values.Encode()))
	r.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rr = httptest.NewRecorder()
	mux.ServeHTTP(rr, r)
	if rr.Code != http.StatusSeeOther {
		t.Fatalf("task update status: got %d %s", rr.Code, rr.Body.String())
	}

	rr = httptest.NewRecorder()
	mux.ServeHTTP(rr, httptest.NewRequest(http.MethodGet, "/api/v1/tasks/"+strconv.FormatInt(taskID, 10), nil))
	if rr.Code != http.StatusOK || !strings.Contains(rr.Body.String(), `"name":"Renamed task"`) {
		t.Fatalf("task name was not preserved: %d %s", rr.Code, rr.Body.String())
	}

	r = httptest.NewRequest(http.MethodPost, "/projects/"+strconv.FormatInt(projectID, 10)+"/delete", nil)
	r.Header.Set("HX-Request", "true")
	rr = httptest.NewRecorder()
	mux.ServeHTTP(rr, r)
	if rr.Code != http.StatusOK || strings.Contains(rr.Body.String(), "<!DOCTYPE html>") || !strings.Contains(rr.Body.String(), "EDITING PROJECT") || !strings.Contains(rr.Body.String(), `hx-post="/projects/`) {
		t.Fatalf("expected project edit fragment after delete conflict, got %d %s", rr.Code, rr.Body.String())
	}
}

func TestJSONTaskErrors(t *testing.T) {
	q := testkit.Open(t)
	mux := http.NewServeMux()
	handlers.Register(mux, q)

	rr := httptest.NewRecorder()
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

func TestJSONRejectsMalformedAndUnknownFields(t *testing.T) {
	q := testkit.Open(t)
	mux := http.NewServeMux()
	handlers.Register(mux, q)

	for _, body := range []string{`{invalid`, `{"name":"ok","unknown_field":1}`} {
		rr := httptest.NewRecorder()
		mux.ServeHTTP(rr, httptest.NewRequest(http.MethodPost, "/api/v1/projects", bytes.NewReader([]byte(body))))
		if rr.Code != http.StatusBadRequest {
			t.Fatalf("body %q: got status %d %s", body, rr.Code, rr.Body.String())
		}
		var apiError struct {
			Error string `json:"error"`
		}
		if err := json.Unmarshal(rr.Body.Bytes(), &apiError); err != nil || apiError.Error == "" {
			t.Fatalf("body %q: expected JSON error response, got %q", body, rr.Body.String())
		}
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
	yearMonth := "2026-08"
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

func TestMonthLockHTMLFragmentPreservesFilters(t *testing.T) {
	q := testkit.Open(t)
	mux := http.NewServeMux()
	handlers.Register(mux, q)
	r := httptest.NewRequest(http.MethodPost, "/month-locks/last/toggle", bytes.NewBufferString("project=ideas"))
	r.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	r.Header.Set("HX-Request", "true")
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, r)
	if rr.Code != http.StatusOK || !bytes.Contains(rr.Body.Bytes(), []byte(`<section id="grid"`)) || bytes.Contains(rr.Body.Bytes(), []byte("<!DOCTYPE html>")) {
		t.Fatalf("got status %d body %q", rr.Code, rr.Body.String())
	}
}

func TestRegisterServesGridRoot(t *testing.T) {
	q := testkit.Open(t)
	mux := http.NewServeMux()
	handlers.Register(mux, q)
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, httptest.NewRequest(http.MethodGet, "/", nil))
	if rr.Code != http.StatusOK || rr.Header().Get("Content-Type") != "text/html; charset=utf-8" || rr.Body.Len() == 0 {
		t.Fatalf("root response %d: %s", rr.Code, rr.Body.String())
	}
}

func TestHTMXGridReturnsFragment(t *testing.T) {
	q := testkit.Open(t)
	mux := http.NewServeMux()
	handlers.Register(mux, q)
	r := httptest.NewRequest(http.MethodGet, "/?project=ideas", nil)
	r.Header.Set("HX-Request", "true")
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, r)
	if rr.Code != http.StatusOK {
		t.Fatalf("grid status %d: %s", rr.Code, rr.Body.String())
	}
	if !bytes.Contains(rr.Body.Bytes(), []byte(`<section id="grid"`)) {
		t.Fatalf("expected grid fragment: %s", rr.Body.String())
	}
	if bytes.Contains(rr.Body.Bytes(), []byte("<!DOCTYPE html>")) || bytes.Contains(rr.Body.Bytes(), []byte("<html")) {
		t.Fatalf("HTMX grid response contains a document: %s", rr.Body.String())
	}
}

func TestUnlockMonthRequiresYearMonth(t *testing.T) {
	q := testkit.Open(t)
	mux := http.NewServeMux()
	handlers.Register(mux, q)

	r := httptest.NewRequest(http.MethodPost, "/month-locks/unlock", bytes.NewBufferString(""))
	r.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, r)
	if rr.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d %s", rr.Code, rr.Body.String())
	}

	rr = httptest.NewRecorder()
	mux.ServeHTTP(rr, httptest.NewRequest(http.MethodGet, "/api/v1/month-locks", nil))
	var list struct {
		Locks []struct{} `json:"month_locks"`
	}
	if err := json.Unmarshal(rr.Body.Bytes(), &list); err != nil {
		t.Fatal(err)
	}
	if len(list.Locks) != 0 {
		t.Fatalf("empty year_month modified lock data: %s", rr.Body.String())
	}
}

func TestEditTaskReassignConflictKeepsStoredProject(t *testing.T) {
	q := testkit.Open(t)
	mux := http.NewServeMux()
	handlers.Register(mux, q)

	start := nextMonday(time.Now())
	a := createProject(t, mux, "A", start)
	b := createProject(t, mux, "B", start)

	tk := struct {
		ID int64 `json:"id"`
	}{ID: createTask(t, mux, []byte(`{"name":"Tracked","project_id":`+strconv.FormatInt(a, 10)+`}`))}

	weekBody := []byte(`{"planned_hours":1}`)
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, httptest.NewRequest(http.MethodPut, "/api/v1/tasks/"+strconv.FormatInt(tk.ID, 10)+"/weeks/"+start.Format("2006-01-02"), bytes.NewReader(weekBody)))
	if rr.Code != http.StatusOK {
		t.Fatalf("save week %d %s", rr.Code, rr.Body.String())
	}

	form := "name=Tracked&project_id=" + strconv.FormatInt(b, 10)
	r := httptest.NewRequest(http.MethodPost, "/tasks/"+strconv.FormatInt(tk.ID, 10), bytes.NewBufferString(form))
	r.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rr = httptest.NewRecorder()
	mux.ServeHTTP(rr, r)
	if rr.Code != http.StatusConflict {
		t.Fatalf("expected conflict, got %d %s", rr.Code, rr.Body.String())
	}
	storedHidden := `name="project_id" value="` + strconv.FormatInt(a, 10) + `"`
	if !bytes.Contains(rr.Body.Bytes(), []byte(storedHidden)) {
		t.Fatalf("form does not keep stored project (%s): %s", storedHidden, rr.Body.String())
	}
	rejectedHidden := `name="project_id" value="` + strconv.FormatInt(b, 10) + `"`
	if bytes.Contains(rr.Body.Bytes(), []byte(rejectedHidden)) {
		t.Fatalf("form carries rejected project (%s): %s", rejectedHidden, rr.Body.String())
	}
}

func TestMutationsRedirectToContext(t *testing.T) {
	q := testkit.Open(t)
	mux := http.NewServeMux()
	handlers.Register(mux, q)
	start := nextMonday(time.Now())
	pid := createProject(t, mux, "P", start)

	spBody := []byte(`{"project_id":` + strconv.FormatInt(pid, 10) + `,"name":"SP","total_hours":10}`)
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, httptest.NewRequest(http.MethodPost, "/api/v1/subprojects", bytes.NewReader(spBody)))
	if rr.Code != http.StatusCreated {
		t.Fatalf("create subproject %d %s", rr.Code, rr.Body.String())
	}
	var sp struct {
		ID int64 `json:"id"`
	}
	if err := json.Unmarshal(rr.Body.Bytes(), &sp); err != nil {
		t.Fatal(err)
	}

	taskID := createTask(t, mux, []byte(`{"name":"T","project_id":`+strconv.FormatInt(pid, 10)+`,"subproject_id":`+strconv.FormatInt(sp.ID, 10)+`}`))

	rr = httptest.NewRecorder()
	mux.ServeHTTP(rr, httptest.NewRequest(http.MethodPost, "/tasks/"+strconv.FormatInt(taskID, 10)+"/delete", nil))
	if rr.Code != http.StatusSeeOther || rr.Header().Get("Location") != "/?project="+strconv.FormatInt(pid, 10)+"&subproject="+strconv.FormatInt(sp.ID, 10) {
		t.Fatalf("task delete redirect: %d %s", rr.Code, rr.Header().Get("Location"))
	}

	form := "name=SP2&project_id=" + strconv.FormatInt(pid, 10) + "&total_hours=10"
	r := httptest.NewRequest(http.MethodPost, "/subprojects/"+strconv.FormatInt(sp.ID, 10), bytes.NewBufferString(form))
	r.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rr = httptest.NewRecorder()
	mux.ServeHTTP(rr, r)
	if rr.Code != http.StatusSeeOther || rr.Header().Get("Location") != "/?project="+strconv.FormatInt(pid, 10)+"&subproject="+strconv.FormatInt(sp.ID, 10) {
		t.Fatalf("subproject update redirect: %d %s", rr.Code, rr.Header().Get("Location"))
	}

	rr = httptest.NewRecorder()
	mux.ServeHTTP(rr, httptest.NewRequest(http.MethodPost, "/subprojects/"+strconv.FormatInt(sp.ID, 10)+"/delete", nil))
	if rr.Code != http.StatusSeeOther || rr.Header().Get("Location") != "/?project="+strconv.FormatInt(pid, 10) {
		t.Fatalf("subproject delete redirect: %d %s", rr.Code, rr.Header().Get("Location"))
	}
}

func TestJSONContractConsistency(t *testing.T) {
	q := testkit.Open(t)
	mux := http.NewServeMux()
	handlers.Register(mux, q)
	start := nextMonday(time.Now())

	noHours := []byte(`{"name":"No hours","start_date":"` + start.Format("2006-01-02") + `","end_date":"` + start.AddDate(0, 0, 28).Format("2006-01-02") + `"}`)
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, httptest.NewRequest(http.MethodPost, "/api/v1/projects", bytes.NewReader(noHours)))
	if rr.Code != http.StatusBadRequest {
		t.Fatalf("project without total_hours: got %d %s", rr.Code, rr.Body.String())
	}

	rr = httptest.NewRecorder()
	mux.ServeHTTP(rr, httptest.NewRequest(http.MethodPost, "/api/v1/tasks", bytes.NewReader([]byte(`{"name":"zero","project_id":0}`))))
	if rr.Code != http.StatusBadRequest {
		t.Fatalf("project_id 0: got %d %s", rr.Code, rr.Body.String())
	}

	pid := createProject(t, mux, "P", start)
	taskID := createTask(t, mux, []byte(`{"name":"T","project_id":`+strconv.FormatInt(pid, 10)+`}`))

	rr = httptest.NewRecorder()
	mux.ServeHTTP(rr, httptest.NewRequest(http.MethodPut, "/api/v1/tasks/"+strconv.FormatInt(taskID, 10)+"/weeks/"+start.Format("2006-01-02"), bytes.NewReader([]byte(`{"progress":25}`))))
	if rr.Code != http.StatusOK {
		t.Fatalf("save progress %d %s", rr.Code, rr.Body.String())
	}
	rr = httptest.NewRecorder()
	mux.ServeHTTP(rr, httptest.NewRequest(http.MethodPut, "/api/v1/tasks/"+strconv.FormatInt(taskID, 10)+"/weeks/"+start.AddDate(0, 0, 7).Format("2006-01-02"), bytes.NewReader([]byte(`{"planned_hours":1}`))))
	if rr.Code != http.StatusOK {
		t.Fatalf("save planned %d %s", rr.Code, rr.Body.String())
	}

	rr = httptest.NewRecorder()
	mux.ServeHTTP(rr, httptest.NewRequest(http.MethodGet, "/api/v1/tasks/"+strconv.FormatInt(taskID, 10), nil))
	if rr.Code != http.StatusOK {
		t.Fatalf("get task %d %s", rr.Code, rr.Body.String())
	}
	var got struct {
		Weeks []struct {
			Progress float64 `json:"progress"`
		} `json:"weeks"`
	}
	if err := json.Unmarshal(rr.Body.Bytes(), &got); err != nil {
		t.Fatal(err)
	}
	if len(got.Weeks) != 2 || got.Weeks[0].Progress != 25 || got.Weeks[1].Progress != 25 {
		t.Fatalf("expected carried progress in weeks: %#v", got.Weeks)
	}

	putBody := []byte(`{"name":"Renamed","project_id":` + strconv.FormatInt(pid, 10) + `}`)
	rr = httptest.NewRecorder()
	mux.ServeHTTP(rr, httptest.NewRequest(http.MethodPut, "/api/v1/tasks/"+strconv.FormatInt(taskID, 10), bytes.NewReader(putBody)))
	if rr.Code != http.StatusOK {
		t.Fatalf("full-replace update %d %s", rr.Code, rr.Body.String())
	}
	var updated struct {
		Name string `json:"name"`
	}
	if err := json.Unmarshal(rr.Body.Bytes(), &updated); err != nil {
		t.Fatal(err)
	}
	if updated.Name != "Renamed" {
		t.Fatalf("renamed task: %q", updated.Name)
	}

	rr = httptest.NewRecorder()
	mux.ServeHTTP(rr, httptest.NewRequest(http.MethodPut, "/api/v1/tasks/"+strconv.FormatInt(taskID, 10), bytes.NewReader([]byte(`{"name":"Tracked"}`))))
	if rr.Code != http.StatusConflict {
		t.Fatalf("omitted project_id on tracked task: got %d %s", rr.Code, rr.Body.String())
	}
}

func postForm(t *testing.T, mux *http.ServeMux, path, form string) *httptest.ResponseRecorder {
	t.Helper()
	r := httptest.NewRequest(http.MethodPost, path, bytes.NewBufferString(form))
	r.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, r)
	return rr
}

func TestUIFormMutationRedirects(t *testing.T) {
	q := testkit.Open(t)
	mux := http.NewServeMux()
	handlers.Register(mux, q)
	start := nextMonday(time.Now())
	span := "&start_date=" + start.Format("2006-01-02") + "&end_date=" + start.AddDate(0, 0, 28).Format("2006-01-02")

	rr := postForm(t, mux, "/projects", "name=P&total_hours=50"+span)
	loc := rr.Header().Get("Location")
	if rr.Code != http.StatusSeeOther || !strings.HasPrefix(loc, "/?project=") {
		t.Fatalf("project create: %d %s", rr.Code, loc)
	}
	pid, err := strconv.ParseInt(strings.TrimPrefix(loc, "/?project="), 10, 64)
	if err != nil || pid < 1 {
		t.Fatalf("project create redirect id: %s", loc)
	}

	rr = postForm(t, mux, "/projects/"+strconv.FormatInt(pid, 10), "name=P2&total_hours=50"+span)
	if rr.Code != http.StatusSeeOther || rr.Header().Get("Location") != "/?project="+strconv.FormatInt(pid, 10) {
		t.Fatalf("project update: %d %s", rr.Code, rr.Header().Get("Location"))
	}

	rr = postForm(t, mux, "/subprojects", "name=SP&project_id="+strconv.FormatInt(pid, 10)+"&total_hours=10")
	loc = rr.Header().Get("Location")
	spPrefix := "/?project=" + strconv.FormatInt(pid, 10) + "&subproject="
	if rr.Code != http.StatusSeeOther || !strings.HasPrefix(loc, spPrefix) {
		t.Fatalf("subproject create: %d %s", rr.Code, loc)
	}
	spID, err := strconv.ParseInt(strings.TrimPrefix(loc, spPrefix), 10, 64)
	if err != nil || spID < 1 {
		t.Fatalf("subproject create redirect id: %s", loc)
	}

	taskID := createTask(t, mux, []byte(`{"name":"T","project_id":`+strconv.FormatInt(pid, 10)+`,"subproject_id":`+strconv.FormatInt(spID, 10)+`}`))
	rr = postForm(t, mux, "/tasks/"+strconv.FormatInt(taskID, 10), "name=T2&project_id="+strconv.FormatInt(pid, 10)+"&subproject_id="+strconv.FormatInt(spID, 10))
	if rr.Code != http.StatusSeeOther || rr.Header().Get("Location") != spPrefix+strconv.FormatInt(spID, 10) {
		t.Fatalf("task update: %d %s", rr.Code, rr.Header().Get("Location"))
	}

	rr = postForm(t, mux, "/tasks", "name=Idea")
	if rr.Code != http.StatusSeeOther || rr.Header().Get("Location") != "/?project=ideas" {
		t.Fatalf("idea task create: %d %s", rr.Code, rr.Header().Get("Location"))
	}

	empty := postForm(t, mux, "/projects", "name=Empty&total_hours=10"+span)
	emptyID, err := strconv.ParseInt(strings.TrimPrefix(empty.Header().Get("Location"), "/?project="), 10, 64)
	if err != nil || emptyID < 1 {
		t.Fatalf("empty project create: %s", empty.Header().Get("Location"))
	}
	rr = postForm(t, mux, "/projects/"+strconv.FormatInt(emptyID, 10)+"/delete", "")
	if rr.Code != http.StatusSeeOther || rr.Header().Get("Location") != "/" {
		t.Fatalf("project delete: %d %s", rr.Code, rr.Header().Get("Location"))
	}
}

func TestHTMLWeekEditErrorsRenderInline(t *testing.T) {
	q := testkit.Open(t)
	mux := http.NewServeMux()
	handlers.Register(mux, q)
	start := nextMonday(time.Now())
	pid := createProject(t, mux, "P", start)
	taskID := createTask(t, mux, []byte(`{"name":"T","project_id":`+strconv.FormatInt(pid, 10)+`}`))

	postWeek := func(body string) *httptest.ResponseRecorder {
		r := httptest.NewRequest(http.MethodPost, "/tasks/"+strconv.FormatInt(taskID, 10)+"/weeks/"+start.Format("2006-01-02"), bytes.NewBufferString(body))
		r.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		r.Header.Set("HX-Request", "true")
		rr := httptest.NewRecorder()
		mux.ServeHTTP(rr, r)
		return rr
	}

	rr := postWeek("planned_hours=-5&project=" + strconv.FormatInt(pid, 10))
	if rr.Code != http.StatusOK || !bytes.Contains(rr.Body.Bytes(), []byte("cell-error")) {
		t.Fatalf("domain error should be 200 + inline cell error: %d %s", rr.Code, rr.Body.String())
	}

	rr = postWeek("planned_hours=abc&project=" + strconv.FormatInt(pid, 10))
	if rr.Code != http.StatusBadRequest {
		t.Fatalf("parse error should be 400: %d %s", rr.Code, rr.Body.String())
	}
}

func TestJSONListTaskFiltersMatchGrid(t *testing.T) {
	q := testkit.Open(t)
	mux := http.NewServeMux()
	handlers.Register(mux, q)
	start := nextMonday(time.Now())
	ideaID := createTask(t, mux, []byte(`{"name":"Idea"}`))
	pid1 := createProject(t, mux, "P1", start)
	pid2 := createProject(t, mux, "P2", start)
	task1 := createTask(t, mux, []byte(`{"name":"In P1","project_id":`+strconv.FormatInt(pid1, 10)+`}`))

	spBody := []byte(`{"project_id":` + strconv.FormatInt(pid2, 10) + `,"name":"SP2","total_hours":10}`)
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, httptest.NewRequest(http.MethodPost, "/api/v1/subprojects", bytes.NewReader(spBody)))
	if rr.Code != http.StatusCreated {
		t.Fatalf("create subproject %d %s", rr.Code, rr.Body.String())
	}
	var sp2 struct {
		ID int64 `json:"id"`
	}
	if err := json.Unmarshal(rr.Body.Bytes(), &sp2); err != nil {
		t.Fatal(err)
	}

	getTasks := func(query string) []struct {
		ID int64 `json:"id"`
	} {
		rr := httptest.NewRecorder()
		mux.ServeHTTP(rr, httptest.NewRequest(http.MethodGet, "/api/v1/tasks"+query, nil))
		if rr.Code != http.StatusOK {
			t.Fatalf("list %s: %d %s", query, rr.Code, rr.Body.String())
		}
		var out struct {
			Tasks []struct {
				ID int64 `json:"id"`
			} `json:"tasks"`
		}
		if err := json.Unmarshal(rr.Body.Bytes(), &out); err != nil {
			t.Fatal(err)
		}
		return out.Tasks
	}

	if tasks := getTasks(""); len(tasks) != 1 || tasks[0].ID != ideaID {
		t.Fatalf("default view should be ideas only: %#v", tasks)
	}
	if tasks := getTasks("?project_id=" + strconv.FormatInt(pid1, 10)); len(tasks) != 1 || tasks[0].ID != task1 {
		t.Fatalf("project filter: %#v", tasks)
	}
	if tasks := getTasks("?project_id=" + strconv.FormatInt(pid1, 10) + "&subproject_id=" + strconv.FormatInt(sp2.ID, 10)); len(tasks) != 1 || tasks[0].ID != task1 {
		t.Fatalf("mismatched subproject should reset to project scope: %#v", tasks)
	}
	if tasks := getTasks("/all"); len(tasks) != 2 {
		t.Fatalf("all-tasks route should list every task: %#v", tasks)
	}

	rr = httptest.NewRecorder()
	mux.ServeHTTP(rr, httptest.NewRequest(http.MethodGet, "/api/v1/tasks?subproject_id=999", nil))
	if rr.Code != http.StatusNotFound {
		t.Fatalf("missing subproject filter should be 404: %d %s", rr.Code, rr.Body.String())
	}
}
