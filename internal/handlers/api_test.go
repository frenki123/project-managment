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

	"cad-development/internal/db"
	"cad-development/internal/db/testkit"
	"cad-development/internal/handlers"
	"cad-development/internal/historyaccess"
	"cad-development/internal/weekly"
)

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

func createResource(t *testing.T, mux *http.ServeMux, path, kind string, body []byte) int64 {
	t.Helper()
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, httptest.NewRequest(http.MethodPost, path, bytes.NewReader(body)))
	if rr.Code != http.StatusCreated {
		t.Fatalf("create %s: %d %s", kind, rr.Code, rr.Body.String())
	}
	var created struct {
		ID int64 `json:"id"`
	}
	if err := json.Unmarshal(rr.Body.Bytes(), &created); err != nil {
		t.Fatal(err)
	}
	return created.ID
}

func putJSON(t *testing.T, mux *http.ServeMux, path, body string, want int) {
	t.Helper()
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, httptest.NewRequest(http.MethodPut, path, strings.NewReader(body)))
	if rr.Code != want {
		t.Fatalf("week %s: expected %d, got %d %s", body, want, rr.Code, rr.Body.String())
	}
}

func TestJSONDuplicateProjectNameConflict(t *testing.T) {
	q := testkit.Open(t)
	mux := http.NewServeMux()
	handlers.Register(mux, q)
	start := weekly.MondayOnOrBefore(time.Now().AddDate(0, 0, 7))
	createProject(t, mux, "Alpha", start)
	second := createProject(t, mux, "Beta", start)
	body := []byte(`{"name":"alpha","total_hours":100,"start_date":"` + start.Format("2006-01-02") + `","end_date":"` + start.AddDate(0, 0, 28).Format("2006-01-02") + `"}`)
	for _, request := range []struct {
		method string
		path   string
	}{
		{http.MethodPost, "/api/v1/projects"},
		{http.MethodPut, "/api/v1/projects/" + strconv.FormatInt(second, 10)},
	} {
		rr := httptest.NewRecorder()
		mux.ServeHTTP(rr, httptest.NewRequest(request.method, request.path, bytes.NewReader(body)))
		assertAPIError(t, rr, http.StatusConflict, "project name already exists", "project-name-taken")
	}
}

func TestJSONTaskAndWeek(t *testing.T) {
	q := testkit.Open(t)
	mux := http.NewServeMux()
	handlers.Register(mux, q)

	start := weekly.MondayOnOrBefore(time.Now().AddDate(0, 0, 7))
	projID := createProject(t, mux, "Alpha", start)
	personID := createPerson(t, mux, "Ada")

	// Keep this map to pin the request wire shape independently of domain types.
	taskBody, err := json.Marshal(map[string]any{"name": "Do work", "project_id": projID, "developer_ids": []int64{personID}, "priority": "high"})
	if err != nil {
		t.Fatal(err)
	}
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, httptest.NewRequest(http.MethodPost, "/api/v1/tasks", bytes.NewReader(taskBody)))
	if rr.Code != http.StatusCreated {
		t.Fatalf("create task %d %s", rr.Code, rr.Body.String())
	}
	var tk struct {
		ID         int64 `json:"id"`
		Developers []struct {
			ID             int64   `json:"id"`
			Name           string  `json:"name"`
			WeeklyCapacity float64 `json:"weekly_capacity"`
		} `json:"developers"`
		Priority string `json:"priority"`
	}
	if err := json.Unmarshal(rr.Body.Bytes(), &tk); err != nil {
		t.Fatal(err)
	}
	if len(tk.Developers) != 1 || tk.Developers[0].ID != personID || tk.Developers[0].Name != "Ada" || tk.Developers[0].WeeklyCapacity != 40 || tk.Priority != "high" {
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
		Status     string  `json:"status"`
	}
	if err := json.Unmarshal(rr.Body.Bytes(), &got); err != nil {
		t.Fatal(err)
	}
	if got.TotalHours != 8 || got.SpentHours != 3 || got.Progress != 25 || got.Status != "Development" {
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

func TestJSONTaskDevelopersWire(t *testing.T) {
	q := testkit.Open(t)
	mux := http.NewServeMux()
	handlers.Register(mux, q)
	start := weekly.MondayOnOrBefore(time.Now().AddDate(0, 0, 7))
	projID := createProject(t, mux, "Alpha", start)
	ada := createPerson(t, mux, "Ada")

	taskID := createResource(t, mux, "/api/v1/tasks", "task", []byte(`{"name":"T","project_id":`+strconv.FormatInt(projID, 10)+`,"developer_ids":[`+strconv.FormatInt(ada, 10)+`]}`))
	path := "/api/v1/tasks/" + strconv.FormatInt(taskID, 10)

	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, httptest.NewRequest(http.MethodGet, path, nil))
	if rr.Code != http.StatusOK {
		t.Fatalf("get task %d %s", rr.Code, rr.Body.String())
	}
	var detail struct {
		Developers []struct {
			ID             int64   `json:"id"`
			Name           string  `json:"name"`
			WeeklyCapacity float64 `json:"weekly_capacity"`
		} `json:"developers"`
	}
	if err := json.Unmarshal(rr.Body.Bytes(), &detail); err != nil {
		t.Fatal(err)
	}
	if len(detail.Developers) != 1 || detail.Developers[0].ID != ada || detail.Developers[0].Name != "Ada" || detail.Developers[0].WeeklyCapacity != 40 {
		t.Fatalf("detail developers = %#v", detail.Developers)
	}

	rr = httptest.NewRecorder()
	mux.ServeHTTP(rr, httptest.NewRequest(http.MethodGet, "/api/v1/tasks?project_id="+strconv.FormatInt(projID, 10), nil))
	if rr.Code != http.StatusOK {
		t.Fatalf("list tasks: %d %s", rr.Code, rr.Body.String())
	}
	var list struct {
		Tasks []struct {
			ID         int64 `json:"id"`
			Developers []struct {
				ID int64 `json:"id"`
			} `json:"developers"`
		} `json:"tasks"`
	}
	if err := json.Unmarshal(rr.Body.Bytes(), &list); err != nil {
		t.Fatal(err)
	}
	if len(list.Tasks) != 1 || list.Tasks[0].ID != taskID {
		t.Fatalf("list tasks = %#v", list.Tasks)
	}
	if list.Tasks[0].Developers != nil {
		t.Fatalf("task list must omit developers: %#v", list.Tasks[0].Developers)
	}

	rr = httptest.NewRecorder()
	mux.ServeHTTP(rr, httptest.NewRequest(http.MethodPut, path, bytes.NewReader([]byte(`{"developer_ids":[999]}`))))
	assertAPIError(t, rr, http.StatusNotFound, "person not found", "person-not-found")

	rr = httptest.NewRecorder()
	mux.ServeHTTP(rr, httptest.NewRequest(http.MethodPut, path, bytes.NewReader([]byte(`{"developer_ids":[`+strconv.FormatInt(ada, 10)+`,`+strconv.FormatInt(ada, 10)+`]}`))))
	assertAPIError(t, rr, http.StatusBadRequest, "duplicate developer", "")

	rr = httptest.NewRecorder()
	mux.ServeHTTP(rr, httptest.NewRequest(http.MethodGet, path, nil))
	if rr.Code != http.StatusOK {
		t.Fatalf("get task after duplicate %d %s", rr.Code, rr.Body.String())
	}
	var after struct {
		Developers []struct {
			ID int64 `json:"id"`
		} `json:"developers"`
	}
	if err := json.Unmarshal(rr.Body.Bytes(), &after); err != nil {
		t.Fatal(err)
	}
	if len(after.Developers) != 1 || after.Developers[0].ID != ada {
		t.Fatalf("duplicate request changed stored developers: %#v", after.Developers)
	}

	rr = httptest.NewRecorder()
	mux.ServeHTTP(rr, httptest.NewRequest(http.MethodPut, path, bytes.NewReader([]byte(`{"developer_ids":[]}`))))
	if rr.Code != http.StatusOK {
		t.Fatalf("clear developers %d %s", rr.Code, rr.Body.String())
	}
	if !bytes.Contains(rr.Body.Bytes(), []byte(`"developers":[]`)) {
		t.Fatalf("detail with no developers must render developers:[]: %s", rr.Body.String())
	}
}

func TestHTMLWeekEditPersists(t *testing.T) {
	q := testkit.Open(t)
	mux := http.NewServeMux()
	handlers.Register(mux, q)

	start := weekly.MondayOnOrBefore(time.Now().AddDate(0, 0, 7))
	projID := createProject(t, mux, "Alpha", start)
	tk := struct {
		ID int64 `json:"id"`
	}{ID: createResource(t, mux, "/api/v1/tasks", "task", []byte(`{"name":"Grid task","project_id":`+strconv.FormatInt(projID, 10)+`}`))}

	body := "planned_hours=8&spent_hours=3&progress=25&project=" + strconv.FormatInt(projID, 10)
	r := httptest.NewRequest(http.MethodPost, "/tasks/"+strconv.FormatInt(tk.ID, 10)+"/weeks/"+start.Format("2006-01-02"), bytes.NewBufferString(body))
	r.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	r.Header.Set("HX-Request", "true")
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, r)
	if rr.Code != http.StatusOK {
		t.Fatalf("week edit %d %s", rr.Code, rr.Body.String())
	}
	if !bytes.Contains(rr.Body.Bytes(), []byte("task-row-"+strconv.FormatInt(tk.ID, 10))) || !bytes.Contains(rr.Body.Bytes(), []byte("week-total-")) || !bytes.Contains(rr.Body.Bytes(), []byte("hx-swap-oob")) || bytes.Contains(rr.Body.Bytes(), []byte("<!DOCTYPE html>")) {
		t.Fatalf("expected row fragment with OOB totals, got %q", rr.Body.String())
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

func TestHTMLWeekHistoricalEditUsesHistoricalEditingCookie(t *testing.T) {
	q := testkit.Open(t)
	mux := http.NewServeMux()
	handlers.Register(mux, q)

	start := weekly.MondayOnOrBefore(time.Now().AddDate(0, -2, 0))
	projectID := createProject(t, mux, "Historical UI", start)
	taskID := createResource(t, mux, "/api/v1/tasks", "task", []byte(`{"name":"Historical task","project_id":`+strconv.FormatInt(projectID, 10)+`}`))
	path := "/tasks/" + strconv.FormatInt(taskID, 10) + "/weeks/" + start.Format(time.DateOnly)
	post := func(cookie *http.Cookie) *httptest.ResponseRecorder {
		r := httptest.NewRequest(http.MethodPost, path, bytes.NewBufferString("planned_hours=4&project="+strconv.FormatInt(projectID, 10)))
		r.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		r.Header.Set("HX-Request", "true")
		if cookie != nil {
			r.AddCookie(cookie)
		}
		rr := httptest.NewRecorder()
		mux.ServeHTTP(rr, r)
		return rr
	}
	if rr := post(nil); rr.Code != http.StatusForbidden || !strings.Contains(rr.Body.String(), "historical editing is not enabled") {
		t.Fatalf("historical edit without cookie: %d %s", rr.Code, rr.Body.String())
	}

	unlock := httptest.NewRequest(http.MethodPost, "/history-access/set", bytes.NewBufferString("project="+strconv.FormatInt(projectID, 10)+"&historical_editing=true"))
	unlock.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	unlock.Header.Set("HX-Request", "true")
	unlockResponse := httptest.NewRecorder()
	mux.ServeHTTP(unlockResponse, unlock)
	cookies := unlockResponse.Result().Cookies()
	var historicalCookie *http.Cookie
	for _, cookie := range cookies {
		if cookie.Name == historyaccess.HistoricalEditingCookieName {
			historicalCookie = cookie
			break
		}
	}
	if unlockResponse.Code != http.StatusOK || historicalCookie == nil {
		t.Fatalf("unlock response: %d cookies=%#v %s", unlockResponse.Code, cookies, unlockResponse.Body.String())
	}
	if rr := post(historicalCookie); rr.Code != http.StatusOK || strings.Contains(rr.Body.String(), "historical editing is not enabled") {
		t.Fatalf("historical edit with cookie: %d %s", rr.Code, rr.Body.String())
	}
	weeks, err := q.ListTaskWeekSeriesByTask(t.Context(), taskID)
	if err != nil || len(weeks) == 0 || weeks[0].PlannedHours != 4 {
		t.Fatalf("historical edit was not saved: %#v %v", weeks, err)
	}
}

func TestHTMLTaskEditAndProjectDeleteFragments(t *testing.T) {
	q := testkit.Open(t)
	mux := http.NewServeMux()
	handlers.Register(mux, q)

	start := weekly.MondayOnOrBefore(time.Now().AddDate(0, 0, 7))
	projectID := createProject(t, mux, "Alpha", start)
	taskID := createResource(t, mux, "/api/v1/tasks", "task", []byte(`{"name":"Original task","project_id":`+strconv.FormatInt(projectID, 10)+`}`))

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
		"developer_ids":        {},
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
	if rr.Code != http.StatusConflict || strings.Contains(rr.Body.String(), "<!DOCTYPE html>") || !strings.Contains(rr.Body.String(), "EDITING PROJECT") || !strings.Contains(rr.Body.String(), `hx-post="/projects/`) {
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

func TestJSONProjectValidationErrorMessage(t *testing.T) {
	q := testkit.Open(t)
	mux := http.NewServeMux()
	handlers.Register(mux, q)
	start := weekly.MondayOnOrBefore(time.Now().AddDate(0, 0, 7))
	body := []byte(`{"name":"P","start_date":"` + start.Format("2006-01-02") + `","end_date":"` + start.AddDate(0, 0, 28).Format("2006-01-02") + `"}`)
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, httptest.NewRequest(http.MethodPost, "/api/v1/projects", bytes.NewReader(body)))
	if rr.Code != http.StatusBadRequest {
		t.Fatalf("project without total_hours: got %d %s", rr.Code, rr.Body.String())
	}
	var apiError struct {
		Error string `json:"error"`
	}
	if err := json.Unmarshal(rr.Body.Bytes(), &apiError); err != nil || apiError.Error != "total hours is required" {
		t.Fatalf("expected JSON error response, got %q", rr.Body.String())
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

func TestSubprojectListUnknownProjectIsNotFound(t *testing.T) {
	q := testkit.Open(t)
	mux := http.NewServeMux()
	handlers.Register(mux, q)
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, httptest.NewRequest(http.MethodGet, "/api/v1/subprojects?project_id=999", nil))
	if rr.Code != http.StatusNotFound {
		t.Fatalf("got status %d body %s", rr.Code, rr.Body.String())
	}
}

func TestListKnownProjectWithoutEntriesIsEmpty(t *testing.T) {
	for _, tc := range []struct {
		name string
		path string
		key  string
	}{
		{name: "subprojects", path: "/api/v1/subprojects", key: "subprojects"},
		{name: "tasks", path: "/api/v1/tasks", key: "tasks"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			q := testkit.Open(t)
			mux := http.NewServeMux()
			handlers.Register(mux, q)
			start := weekly.MondayOnOrBefore(time.Now().AddDate(0, 0, 7))
			pid := createProject(t, mux, "Empty", start)
			rr := httptest.NewRecorder()
			mux.ServeHTTP(rr, httptest.NewRequest(http.MethodGet, tc.path+"?project_id="+strconv.FormatInt(pid, 10), nil))
			if rr.Code != http.StatusOK {
				t.Fatalf("valid project with no %s should be 200: %d %s", tc.name, rr.Code, rr.Body.String())
			}
			var out map[string][]struct{}
			if err := json.Unmarshal(rr.Body.Bytes(), &out); err != nil {
				t.Fatal(err)
			}
			if len(out[tc.key]) != 0 {
				t.Fatalf("expected empty %s list, got %d", tc.name, len(out[tc.key]))
			}
		})
	}
}

func TestSubprojectFormRejectsMalformedProjectID(t *testing.T) {
	q := testkit.Open(t)
	mux := http.NewServeMux()
	handlers.Register(mux, q)
	rr := postForm(t, mux, "/subprojects", "name=SP&project_id=bad&total_hours=")
	if rr.Code != http.StatusBadRequest || !strings.Contains(rr.Body.String(), "invalid project") || strings.Contains(rr.Body.String(), "total hours is required") {
		t.Fatalf("expected 400 with invalid project message, got %d %s", rr.Code, rr.Body.String())
	}
}

func TestJSONUnlockAppliesToOneWeeklyRequest(t *testing.T) {
	q := testkit.Open(t)
	mux := http.NewServeMux()
	handlers.Register(mux, q)
	first := weekly.MondayOnOrBefore(time.Now().AddDate(0, -5, 0))
	second := weekly.MondayOnOrBefore(time.Now().AddDate(0, -3, 0))
	projectBody := `{"name":"History","total_hours":100,"start_date":"` + first.Format(time.DateOnly) + `","end_date":"` + time.Now().Format(time.DateOnly) + `"}`
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, httptest.NewRequest(http.MethodPost, "/api/v1/projects", strings.NewReader(projectBody)))
	if rr.Code != http.StatusCreated {
		t.Fatalf("create project: %d %s", rr.Code, rr.Body.String())
	}
	var p struct {
		ID int64 `json:"id"`
	}
	if err := json.Unmarshal(rr.Body.Bytes(), &p); err != nil {
		t.Fatal(err)
	}
	id := createResource(t, mux, "/api/v1/tasks", "task", []byte(`{"name":"History task","project_id":`+strconv.FormatInt(p.ID, 10)+`}`))
	path := func(week time.Time) string {
		return "/api/v1/tasks/" + strconv.FormatInt(id, 10) + "/weeks/" + week.Format(time.DateOnly)
	}
	putJSON(t, mux, path(first), `{"planned_hours":6,"progress":40}`, http.StatusForbidden)
	putJSON(t, mux, path(first), `{"planned_hours":6,"progress":40,"unlock":true}`, http.StatusOK)
	putJSON(t, mux, path(second), `{"spent_hours":3}`, http.StatusForbidden)
	putJSON(t, mux, path(second), `{"spent_hours":3,"progress":60,"unlock":true}`, http.StatusOK)
	putJSON(t, mux, path(first), `{"planned_hours":7}`, http.StatusForbidden)
	for _, want := range []struct {
		week                  time.Time
		plan, spent, progress float64
	}{{first, 6, 0, 40}, {second, 0, 3, 60}} {
		row, err := q.GetTaskWeek(t.Context(), db.GetTaskWeekParams{TaskID: id, WeekStart: want.week.Format(time.DateOnly)})
		if err != nil || row.PlannedHours != want.plan || row.SpentHours != want.spent || !row.Progress.Valid || row.Progress.Float64 != want.progress {
			t.Fatalf("history changed after relock: %#v %v", row, err)
		}
	}
}

func TestHistoryAccessHTMLFragmentPreservesFilters(t *testing.T) {
	q := testkit.Open(t)
	mux := http.NewServeMux()
	handlers.Register(mux, q)
	projectID := createProject(t, mux, "Lock filter", weekly.MondayOnOrBefore(time.Now().AddDate(0, 0, 7)))
	project := strconv.FormatInt(projectID, 10)
	r := httptest.NewRequest(http.MethodPost, "/history-access/set", bytes.NewBufferString("project="+project+"&historical_editing=true"))
	r.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	r.Header.Set("HX-Request", "true")
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, r)
	if rr.Code != http.StatusOK || !bytes.Contains(rr.Body.Bytes(), []byte(`<section id="grid"`)) || bytes.Contains(rr.Body.Bytes(), []byte("<!DOCTYPE html>")) {
		t.Fatalf("got status %d body %q", rr.Code, rr.Body.String())
	}
	if !strings.Contains(rr.Body.String(), `value="`+project+`" selected`) {
		t.Fatalf("project filter not preserved: %s", rr.Body.String())
	}
	if !strings.Contains(rr.Body.String(), "Stop historical editing") || strings.Contains(rr.Body.String(), "Allow historical editing") {
		t.Fatalf("grid did not reflect browser unlock: %s", rr.Body.String())
	}
	if !strings.HasPrefix(rr.Header().Get("Set-Cookie"), historyaccess.HistoricalEditingCookieName+"=") {
		t.Fatalf("HTML unlock did not set a cookie: %s", rr.Header().Get("Set-Cookie"))
	}
	rr = httptest.NewRecorder()
	mux.ServeHTTP(rr, httptest.NewRequest(http.MethodGet, "/?project=ideas", nil))
	if rr.Code != http.StatusOK || strings.Contains(rr.Body.String(), "Stop historical editing") || strings.Contains(rr.Body.String(), "Allow historical editing") {
		t.Fatalf("history controls should not render in ideas view: %d %s", rr.Code, rr.Body.String())
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
	if !strings.Contains(rr.Body.String(), `value="all" selected`) || !strings.Contains(rr.Body.String(), "All tasks") || strings.Contains(rr.Body.String(), "Weekly timeline") {
		t.Fatalf("root should render the all-task summary view: %s", rr.Body.String())
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

func TestHTMLHistoryAccessRequiresBooleanState(t *testing.T) {
	q := testkit.Open(t)
	mux := http.NewServeMux()
	handlers.Register(mux, q)

	for _, body := range []string{"project=ideas", "project=ideas&historical_editing=maybe"} {
		rr := postForm(t, mux, "/history-access/set", body)
		if rr.Code != http.StatusBadRequest {
			t.Fatalf("%q: expected 400, got %d %s", body, rr.Code, rr.Body.String())
		}
		if len(rr.Result().Cookies()) != 0 {
			t.Fatalf("%q: invalid state set a cookie: %v", body, rr.Result().Cookies())
		}
	}
}

func TestEditTaskReassignConflictKeepsStoredProject(t *testing.T) {
	q := testkit.Open(t)
	mux := http.NewServeMux()
	handlers.Register(mux, q)

	start := weekly.MondayOnOrBefore(time.Now().AddDate(0, 0, 7))
	a := createProject(t, mux, "A", start)
	b := createProject(t, mux, "B", start)

	tk := struct {
		ID int64 `json:"id"`
	}{ID: createResource(t, mux, "/api/v1/tasks", "task", []byte(`{"name":"Tracked","project_id":`+strconv.FormatInt(a, 10)+`}`))}

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
	start := weekly.MondayOnOrBefore(time.Now().AddDate(0, 0, 7))
	pid := createProject(t, mux, "P", start)

	spBody := []byte(`{"project_id":` + strconv.FormatInt(pid, 10) + `,"name":"SP","total_hours":10}`)
	spID := createResource(t, mux, "/api/v1/subprojects", "subproject", spBody)

	taskID := createResource(t, mux, "/api/v1/tasks", "task", []byte(`{"name":"T","project_id":`+strconv.FormatInt(pid, 10)+`,"subproject_id":`+strconv.FormatInt(spID, 10)+`}`))

	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, httptest.NewRequest(http.MethodPost, "/tasks/"+strconv.FormatInt(taskID, 10)+"/delete", nil))
	if rr.Code != http.StatusSeeOther || rr.Header().Get("Location") != "/?project="+strconv.FormatInt(pid, 10)+"&subproject="+strconv.FormatInt(spID, 10) {
		t.Fatalf("task delete redirect: %d %s", rr.Code, rr.Header().Get("Location"))
	}

	form := "name=SP2&project_id=" + strconv.FormatInt(pid, 10) + "&total_hours=10"
	r := httptest.NewRequest(http.MethodPost, "/subprojects/"+strconv.FormatInt(spID, 10), bytes.NewBufferString(form))
	r.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rr = httptest.NewRecorder()
	mux.ServeHTTP(rr, r)
	if rr.Code != http.StatusSeeOther || rr.Header().Get("Location") != "/?project="+strconv.FormatInt(pid, 10)+"&subproject="+strconv.FormatInt(spID, 10) {
		t.Fatalf("subproject update redirect: %d %s", rr.Code, rr.Header().Get("Location"))
	}

	rr = httptest.NewRecorder()
	mux.ServeHTTP(rr, httptest.NewRequest(http.MethodPost, "/subprojects/"+strconv.FormatInt(spID, 10)+"/delete", nil))
	if rr.Code != http.StatusSeeOther || rr.Header().Get("Location") != "/?project="+strconv.FormatInt(pid, 10) {
		t.Fatalf("subproject delete redirect: %d %s", rr.Code, rr.Header().Get("Location"))
	}

	rr = httptest.NewRecorder()
	mux.ServeHTTP(rr, httptest.NewRequest(http.MethodGet, "/api/v1/tasks/"+strconv.FormatInt(taskID, 10), nil))
	assertAPIError(t, rr, http.StatusNotFound, "task not found", "")
	rr = httptest.NewRecorder()
	mux.ServeHTTP(rr, httptest.NewRequest(http.MethodDelete, "/api/v1/tasks/"+strconv.FormatInt(taskID, 10), nil))
	assertAPIError(t, rr, http.StatusNotFound, "task not found", "")
	rr = httptest.NewRecorder()
	mux.ServeHTTP(rr, httptest.NewRequest(http.MethodGet, "/api/v1/subprojects/"+strconv.FormatInt(spID, 10), nil))
	assertAPIError(t, rr, http.StatusNotFound, "subproject not found", "")
	rr = httptest.NewRecorder()
	mux.ServeHTTP(rr, httptest.NewRequest(http.MethodDelete, "/api/v1/subprojects/"+strconv.FormatInt(spID, 10), nil))
	assertAPIError(t, rr, http.StatusNotFound, "subproject not found", "")
}

func TestJSONProgressNullClearsStoredProgress(t *testing.T) {
	q := testkit.Open(t)
	mux := http.NewServeMux()
	handlers.Register(mux, q)
	start := weekly.MondayOnOrBefore(time.Now().AddDate(0, 0, 7))
	pid := createProject(t, mux, "P", start)
	taskID := createResource(t, mux, "/api/v1/tasks", "task", []byte(`{"name":"T","project_id":`+strconv.FormatInt(pid, 10)+`}`))
	path := "/api/v1/tasks/" + strconv.FormatInt(taskID, 10) + "/weeks/" + start.Format(time.DateOnly)
	putJSON(t, mux, path, `{"progress":30}`, http.StatusOK)
	putJSON(t, mux, path, `{"progress":null}`, http.StatusOK)
	row, err := q.GetTaskWeek(t.Context(), db.GetTaskWeekParams{TaskID: taskID, WeekStart: start.Format(time.DateOnly)})
	if err != nil || row.Progress.Valid {
		t.Fatalf("progress null should clear stored progress: %#v %v", row, err)
	}
}

func TestJSONRejectsClearProgressUnknownField(t *testing.T) {
	q := testkit.Open(t)
	mux := http.NewServeMux()
	handlers.Register(mux, q)
	start := weekly.MondayOnOrBefore(time.Now().AddDate(0, 0, 7))
	pid := createProject(t, mux, "P", start)
	taskID := createResource(t, mux, "/api/v1/tasks", "task", []byte(`{"name":"T","project_id":`+strconv.FormatInt(pid, 10)+`}`))
	path := "/api/v1/tasks/" + strconv.FormatInt(taskID, 10) + "/weeks/" + start.Format(time.DateOnly)
	putJSON(t, mux, path, `{"progress":30}`, http.StatusOK)
	putJSON(t, mux, path, `{"progress":30,"clear_progress":true}`, http.StatusBadRequest)
}

func TestJSONContractConsistency(t *testing.T) {
	q := testkit.Open(t)
	mux := http.NewServeMux()
	handlers.Register(mux, q)
	start := weekly.MondayOnOrBefore(time.Now().AddDate(0, 0, 7))

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
	taskID := createResource(t, mux, "/api/v1/tasks", "task", []byte(`{"name":"T","project_id":`+strconv.FormatInt(pid, 10)+`}`))

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
	var updatedWeek struct {
		WeekStart      string   `json:"week_start"`
		PlannedHours   float64  `json:"planned_hours"`
		SpentHours     float64  `json:"spent_hours"`
		Progress       float64  `json:"progress"`
		StoredProgress *float64 `json:"stored_progress"`
	}
	if err := json.Unmarshal(rr.Body.Bytes(), &updatedWeek); err != nil {
		t.Fatal(err)
	}
	if updatedWeek.WeekStart != start.AddDate(0, 0, 7).Format("2006-01-02") || updatedWeek.PlannedHours != 1 || updatedWeek.SpentHours != 0 || updatedWeek.Progress != 25 || updatedWeek.StoredProgress != nil {
		t.Fatalf("weekly update response = %#v", updatedWeek)
	}

	rr = httptest.NewRecorder()
	mux.ServeHTTP(rr, httptest.NewRequest(http.MethodGet, "/api/v1/tasks/"+strconv.FormatInt(taskID, 10), nil))
	if rr.Code != http.StatusOK {
		t.Fatalf("get task %d %s", rr.Code, rr.Body.String())
	}
	var got struct {
		Weeks []struct {
			WeekStart      string   `json:"week_start"`
			PlannedHours   float64  `json:"planned_hours"`
			SpentHours     float64  `json:"spent_hours"`
			Progress       float64  `json:"progress"`
			StoredProgress *float64 `json:"stored_progress"`
		} `json:"weeks"`
	}
	if err := json.Unmarshal(rr.Body.Bytes(), &got); err != nil {
		t.Fatal(err)
	}
	if len(got.Weeks) != 5 {
		t.Fatalf("expected all project-bounded weeks, got %#v", got.Weeks)
	}
	expectedPlanned := []float64{0, 1, 0, 0, 0}
	for i, week := range got.Weeks {
		expectedWeek := start.AddDate(0, 0, i*7).Format("2006-01-02")
		if week.WeekStart != expectedWeek || week.PlannedHours != expectedPlanned[i] || week.SpentHours != 0 || week.Progress != 25 {
			t.Fatalf("unexpected week %d: %#v", i, week)
		}
	}
	if got.Weeks[0].StoredProgress == nil || *got.Weeks[0].StoredProgress != 25 || got.Weeks[1].StoredProgress != nil {
		t.Fatalf("stored/effective progress distinction lost: %#v", got.Weeks)
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
	if rr.Code != http.StatusOK {
		t.Fatalf("partial task update: got %d %s", rr.Code, rr.Body.String())
	}
	var preserved struct {
		ProjectID *int64 `json:"project_id"`
	}
	if err := json.Unmarshal(rr.Body.Bytes(), &preserved); err != nil {
		t.Fatal(err)
	}
	if preserved.ProjectID == nil || *preserved.ProjectID != pid {
		t.Fatalf("partial update lost project assignment: %#v", preserved.ProjectID)
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

func assertAPIError(t *testing.T, rr *httptest.ResponseRecorder, wantStatus int, wantMessage, wantReason string) {
	t.Helper()
	if rr.Code != wantStatus {
		t.Fatalf("expected %d, got %d %s", wantStatus, rr.Code, rr.Body.String())
	}
	var payload struct {
		Error  string `json:"error"`
		Reason string `json:"reason"`
	}
	if err := json.Unmarshal(rr.Body.Bytes(), &payload); err != nil {
		t.Fatalf("decode API error %q: %v", rr.Body.String(), err)
	}
	if payload.Error != wantMessage || payload.Reason != wantReason {
		t.Fatalf("got message %q reason %q", payload.Error, payload.Reason)
	}
}

func TestJSONConflictReasonCodes(t *testing.T) {
	q := testkit.Open(t)
	mux := http.NewServeMux()
	handlers.Register(mux, q)
	start := weekly.MondayOnOrBefore(time.Now().AddDate(0, 0, 7))
	weekPath := func(id int64, week time.Time) string {
		return "/api/v1/tasks/" + strconv.FormatInt(id, 10) + "/weeks/" + week.Format("2006-01-02")
	}
	request := func(method, path, body string) *httptest.ResponseRecorder {
		rr := httptest.NewRecorder()
		mux.ServeHTTP(rr, httptest.NewRequest(method, path, bytes.NewReader([]byte(body))))
		return rr
	}

	ideaID := createResource(t, mux, "/api/v1/tasks", "task", []byte(`{"name":"Idea"}`))
	assertAPIError(t, request(http.MethodPut, weekPath(ideaID, start), `{"planned_hours":1}`),
		http.StatusBadRequest, "ideas cannot be planned", "idea-task-not-assignable")

	projID := createProject(t, mux, "Alpha", start)
	taskID := createResource(t, mux, "/api/v1/tasks", "task", []byte(`{"name":"T","project_id":`+strconv.FormatInt(projID, 10)+`}`))
	assertAPIError(t, request(http.MethodPut, weekPath(taskID, start.AddDate(0, 0, 35)), `{"planned_hours":1}`),
		http.StatusBadRequest, "week is outside the project date range", "week-outside-project-bounds")

	if rr := request(http.MethodPost, "/api/v1/subprojects", `{"project_id":`+strconv.FormatInt(projID, 10)+`,"name":"SP","total_hours":60}`); rr.Code != http.StatusCreated {
		t.Fatalf("create subproject: %d %s", rr.Code, rr.Body.String())
	}
	assertAPIError(t, request(http.MethodPost, "/api/v1/subprojects", `{"project_id":`+strconv.FormatInt(projID, 10)+`,"name":"SP","total_hours":50}`),
		http.StatusConflict, "subproject hours exceed project hours", "subproject-hours-exceed-project")

	if rr := request(http.MethodPut, weekPath(taskID, start), `{"planned_hours":1}`); rr.Code != http.StatusOK {
		t.Fatalf("save week: %d %s", rr.Code, rr.Body.String())
	}
	otherID := createProject(t, mux, "Beta", start)
	reassign := `{"name":"T","project_id":` + strconv.FormatInt(otherID, 10) + `}`
	assertAPIError(t, request(http.MethodPut, "/api/v1/tasks/"+strconv.FormatInt(taskID, 10), reassign),
		http.StatusConflict, "cannot reassign task with weekly data", "task-has-weekly-data")
}

func TestUIFormMutationRedirects(t *testing.T) {
	q := testkit.Open(t)
	mux := http.NewServeMux()
	handlers.Register(mux, q)
	start := weekly.MondayOnOrBefore(time.Now().AddDate(0, 0, 7))
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

	taskID := createResource(t, mux, "/api/v1/tasks", "task", []byte(`{"name":"T","project_id":`+strconv.FormatInt(pid, 10)+`,"subproject_id":`+strconv.FormatInt(spID, 10)+`}`))
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

	rr = httptest.NewRecorder()
	mux.ServeHTTP(rr, httptest.NewRequest(http.MethodGet, "/api/v1/projects/"+strconv.FormatInt(emptyID, 10), nil))
	assertAPIError(t, rr, http.StatusNotFound, "project not found", "")
	rr = httptest.NewRecorder()
	mux.ServeHTTP(rr, httptest.NewRequest(http.MethodDelete, "/api/v1/projects/"+strconv.FormatInt(emptyID, 10), nil))
	assertAPIError(t, rr, http.StatusNotFound, "project not found", "")
}

func TestHTMLWeekEditErrorsRenderInline(t *testing.T) {
	q := testkit.Open(t)
	mux := http.NewServeMux()
	handlers.Register(mux, q)
	start := weekly.MondayOnOrBefore(time.Now().AddDate(0, 0, 7))
	pid := createProject(t, mux, "P", start)
	taskID := createResource(t, mux, "/api/v1/tasks", "task", []byte(`{"name":"T","project_id":`+strconv.FormatInt(pid, 10)+`}`))

	postWeek := func(body string) *httptest.ResponseRecorder {
		r := httptest.NewRequest(http.MethodPost, "/tasks/"+strconv.FormatInt(taskID, 10)+"/weeks/"+start.Format("2006-01-02"), bytes.NewBufferString(body))
		r.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		r.Header.Set("HX-Request", "true")
		rr := httptest.NewRecorder()
		mux.ServeHTTP(rr, r)
		return rr
	}

	rr := postWeek("planned_hours=-5&project=" + strconv.FormatInt(pid, 10))
	if rr.Code != http.StatusBadRequest || !bytes.Contains(rr.Body.Bytes(), []byte("cell-error")) {
		t.Fatalf("domain error should preserve status + inline cell error: %d %s", rr.Code, rr.Body.String())
	}

	rr = postWeek("planned_hours=abc&project=" + strconv.FormatInt(pid, 10))
	if rr.Code != http.StatusBadRequest || !bytes.Contains(rr.Body.Bytes(), []byte("cell-error")) {
		t.Fatalf("parse error should be inline: %d %s", rr.Code, rr.Body.String())
	}
	rr = postWeek("progress=50&project=" + strconv.FormatInt(pid, 10))
	if rr.Code != http.StatusOK {
		t.Fatalf("set progress: %d %s", rr.Code, rr.Body.String())
	}
	rr = postWeek("progress=&project=" + strconv.FormatInt(pid, 10))
	if rr.Code != http.StatusOK {
		t.Fatalf("clear progress: %d %s", rr.Code, rr.Body.String())
	}
	row, err := q.GetTaskWeek(t.Context(), db.GetTaskWeekParams{TaskID: taskID, WeekStart: start.Format(time.DateOnly)})
	if err != nil || row.Progress.Valid {
		t.Fatalf("clearing input did not clear stored progress: %#v %v", row, err)
	}
}

func TestHTMLWeekEditRejectsInvalidDisplayContextBeforeSaving(t *testing.T) {
	q := testkit.Open(t)
	mux := http.NewServeMux()
	handlers.Register(mux, q)
	start := weekly.MondayOnOrBefore(time.Now().AddDate(0, 0, 7))
	pid := createProject(t, mux, "P", start)
	other := createProject(t, mux, "Other", start)
	taskID := createResource(t, mux, "/api/v1/tasks", "task", []byte(`{"name":"T","project_id":`+strconv.FormatInt(pid, 10)+`}`))
	path := "/tasks/" + strconv.FormatInt(taskID, 10) + "/weeks/" + start.Format(time.DateOnly)
	for _, body := range []string{"planned_hours=5&project=bad", "planned_hours=5&project=" + strconv.FormatInt(other, 10)} {
		r := httptest.NewRequest(http.MethodPost, path, bytes.NewBufferString(body))
		r.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		r.Header.Set("HX-Request", "true")
		rr := httptest.NewRecorder()
		mux.ServeHTTP(rr, r)
		if rr.Code != http.StatusBadRequest {
			t.Fatalf("%q: expected 400, got %d %s", body, rr.Code, rr.Body.String())
		}
		week, err := q.GetTaskWeek(t.Context(), db.GetTaskWeekParams{TaskID: taskID, WeekStart: start.Format(time.DateOnly)})
		if err == nil || week.PlannedHours != 0 {
			t.Fatalf("%q: write despite invalid view: %v %v", body, week, err)
		}
	}
}

func TestHTMLHistoryAccessSetIsIdempotentAndValidatesView(t *testing.T) {
	q := testkit.Open(t)
	mux := http.NewServeMux()
	handlers.Register(mux, q)
	post := func(body string) *httptest.ResponseRecorder {
		r := httptest.NewRequest(http.MethodPost, "/history-access/set", bytes.NewBufferString(body))
		r.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		r.Header.Set("HX-Request", "true")
		rr := httptest.NewRecorder()
		mux.ServeHTTP(rr, r)
		return rr
	}
	for _, body := range []string{"historical_editing=true&project=bad", "historical_editing=true&project=999999"} {
		rr := post(body)
		if rr.Code != http.StatusBadRequest && rr.Code != http.StatusNotFound {
			t.Fatalf("invalid view %q: %d %s", body, rr.Code, rr.Body.String())
		}
		if len(rr.Result().Cookies()) != 0 {
			t.Fatalf("invalid view %q set a cookie: %v", body, rr.Result().Cookies())
		}
	}
	for _, value := range []string{"true", "true", "false", "false"} {
		rr := post("historical_editing=" + value + "&project=ideas")
		if rr.Code != http.StatusOK || !strings.Contains(rr.Body.String(), `<section id="grid"`) || strings.Contains(rr.Body.String(), "<!DOCTYPE html>") {
			t.Fatalf("set %s: %d %s", value, rr.Code, rr.Body.String())
		}
		cookieHeader := rr.Header().Get("Set-Cookie")
		if value == "true" && !strings.HasPrefix(cookieHeader, historyaccess.HistoricalEditingCookieName+"=") {
			t.Fatalf("set %s did not set unlock cookie: %s", value, cookieHeader)
		}
		if value == "false" && (!strings.HasPrefix(cookieHeader, historyaccess.HistoricalEditingCookieName+"=") || !strings.Contains(cookieHeader, "Max-Age=0")) {
			t.Fatalf("set %s did not clear unlock cookie: %s", value, cookieHeader)
		}
	}
}

func TestJSONListTaskFiltersMatchGrid(t *testing.T) {
	q := testkit.Open(t)
	mux := http.NewServeMux()
	handlers.Register(mux, q)
	start := weekly.MondayOnOrBefore(time.Now().AddDate(0, 0, 7))
	createResource(t, mux, "/api/v1/tasks", "task", []byte(`{"name":"Idea"}`))
	pid1 := createProject(t, mux, "P1", start)
	pid2 := createProject(t, mux, "P2", start)
	task1 := createResource(t, mux, "/api/v1/tasks", "task", []byte(`{"name":"In P1","project_id":`+strconv.FormatInt(pid1, 10)+`}`))

	spBody := []byte(`{"project_id":` + strconv.FormatInt(pid2, 10) + `,"name":"SP2","total_hours":10}`)
	spID := createResource(t, mux, "/api/v1/subprojects", "subproject", spBody)

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

	if tasks := getTasks(""); len(tasks) != 2 {
		t.Fatalf("default view should include all tasks: %#v", tasks)
	}
	if tasks := getTasks("?project_id=" + strconv.FormatInt(pid1, 10)); len(tasks) != 1 || tasks[0].ID != task1 {
		t.Fatalf("project filter: %#v", tasks)
	}
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, httptest.NewRequest(http.MethodGet, "/api/v1/tasks?project_id="+strconv.FormatInt(pid1, 10)+"&subproject_id="+strconv.FormatInt(spID, 10), nil))
	assertAPIError(t, rr, http.StatusBadRequest, "subproject does not belong to project", "")
	rr = httptest.NewRecorder()
	mux.ServeHTTP(rr, httptest.NewRequest(http.MethodGet, "/api/v1/tasks?subproject_id=999", nil))
	if rr.Code != http.StatusNotFound {
		t.Fatalf("missing subproject filter should be 404: %d %s", rr.Code, rr.Body.String())
	}
}
