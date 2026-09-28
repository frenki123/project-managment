package client

import (
	"bytes"
	"context"
	json "encoding/json/v2"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

const DefaultURL = "http://127.0.0.1:8080"

type Client struct {
	BaseURL *url.URL
	HTTP    *http.Client
}

type APIError struct {
	Status  int
	Message string
}

func (e *APIError) Error() string {
	if e.Message == "" {
		return fmt.Sprintf("server returned HTTP %d", e.Status)
	}
	return e.Message
}

func New(rawURL string) (*Client, error) {
	u, err := url.Parse(rawURL)
	if err != nil || u.Scheme == "" || u.Host == "" {
		return nil, fmt.Errorf("invalid API URL %q", rawURL)
	}
	return &Client{
		BaseURL: u,
		HTTP:    &http.Client{Timeout: 30 * time.Second},
	}, nil
}

func (c *Client) Do(ctx context.Context, method, path string, input, output any) error {
	var body io.Reader
	if input != nil {
		data, err := json.Marshal(input)
		if err != nil {
			return fmt.Errorf("encode request: %w", err)
		}
		body = bytes.NewReader(data)
	}
	rel, err := url.Parse(path)
	if err != nil {
		return fmt.Errorf("create request URL: %w", err)
	}
	u := *c.BaseURL
	u.Path = strings.TrimRight(c.BaseURL.Path, "/") + rel.Path
	u.RawQuery = rel.RawQuery
	req, err := http.NewRequestWithContext(ctx, method, u.String(), body)
	if err != nil {
		return fmt.Errorf("create request: %w", err)
	}
	if input != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	req.Header.Set("Accept", "application/json")
	resp, err := c.HTTP.Do(req)
	if err != nil {
		return fmt.Errorf("request %s %s: %w", method, path, err)
	}
	defer resp.Body.Close()
	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("read response: %w", err)
	}
	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		var payload struct {
			Error string `json:"error"`
		}
		_ = json.Unmarshal(data, &payload)
		return &APIError{Status: resp.StatusCode, Message: payload.Error}
	}
	if output == nil || resp.StatusCode == http.StatusNoContent || len(data) == 0 {
		return nil
	}
	if err := json.Unmarshal(data, output); err != nil {
		return fmt.Errorf("decode response: %w", err)
	}
	return nil
}

type Project struct {
	ID                int64   `json:"id"`
	Name              string  `json:"name"`
	PurchaseOrderName string  `json:"purchase_order_name"`
	TotalHours        float64 `json:"total_hours"`
	StartDate         string  `json:"start_date"`
	EndDate           string  `json:"end_date"`
	PlannedHours      float64 `json:"planned_hours"`
	SpentHours        float64 `json:"spent_hours"`
	ProgressPct       float64 `json:"progress_pct"`
	EarnedHours       float64 `json:"earned_hours"`
}
type ProjectInput struct {
	Name              string   `json:"name"`
	PurchaseOrderName string   `json:"purchase_order_name"`
	TotalHours        *float64 `json:"total_hours"`
	StartDate         string   `json:"start_date"`
	EndDate           string   `json:"end_date"`
}
type ProjectsResponse struct {
	Projects []Project `json:"projects"`
}

type Subproject struct {
	ID           int64   `json:"id"`
	ProjectID    int64   `json:"project_id"`
	Name         string  `json:"name"`
	TotalHours   float64 `json:"total_hours"`
	PlannedHours float64 `json:"planned_hours"`
	SpentHours   float64 `json:"spent_hours"`
}
type SubprojectInput struct {
	ProjectID  int64    `json:"project_id"`
	Name       string   `json:"name"`
	TotalHours *float64 `json:"total_hours"`
}
type SubprojectsResponse struct {
	Subprojects []Subproject `json:"subprojects"`
}

type Task struct {
	ID                  int64      `json:"id"`
	Name                string     `json:"name"`
	Description         string     `json:"description"`
	ImplementationNotes string     `json:"implementation_notes"`
	Department          string     `json:"department"`
	Developers          string     `json:"developers"`
	Priority            string     `json:"priority"`
	ProjectID           *int64     `json:"project_id"`
	SubprojectID        *int64     `json:"subproject_id"`
	TotalHours          float64    `json:"total_hours"`
	SpentHours          float64    `json:"spent_hours"`
	Progress            float64    `json:"progress"`
	Status              string     `json:"status"`
	Weeks               []WeekCell `json:"weeks,omitempty"`
}
type TaskInput struct {
	Name                string `json:"name"`
	Description         string `json:"description"`
	ImplementationNotes string `json:"implementation_notes"`
	Department          string `json:"department"`
	Developers          string `json:"developers"`
	Priority            string `json:"priority"`
	ProjectID           *int64 `json:"project_id"`
	SubprojectID        *int64 `json:"subproject_id"`
}
type TasksResponse struct {
	Tasks []Task `json:"tasks"`
}
type WeekCell struct {
	TaskID       int64    `json:"task_id"`
	WeekStart    string   `json:"week_start"`
	PlannedHours float64  `json:"planned_hours"`
	SpentHours   float64  `json:"spent_hours"`
	Progress     *float64 `json:"progress"`
}
type WeekPatch struct {
	PlannedHours  *float64 `json:"planned_hours"`
	SpentHours    *float64 `json:"spent_hours"`
	Progress      *float64 `json:"progress"`
	ClearProgress bool     `json:"clear_progress,omitempty"`
}
type MonthLockState struct {
	Unlocked bool `json:"unlocked"`
}
type SCurve struct {
	Project Project      `json:"project"`
	Weeks   []SCurveWeek `json:"weeks"`
}
type SCurveWeek struct {
	WeekStart    string  `json:"week_start"`
	PlannedHours float64 `json:"planned_hours"`
	SpentHours   float64 `json:"spent_hours"`
	EarnedHours  float64 `json:"earned_hours"`
}

func (c *Client) Projects(ctx context.Context) (ProjectsResponse, error) {
	var v ProjectsResponse
	return v, c.Do(ctx, http.MethodGet, "/api/v1/projects", nil, &v)
}
func (c *Client) Project(ctx context.Context, id int64) (Project, error) {
	var v Project
	return v, c.Do(ctx, http.MethodGet, fmt.Sprintf("/api/v1/projects/%d", id), nil, &v)
}
func (c *Client) CreateProject(ctx context.Context, in ProjectInput) (Project, error) {
	var v Project
	return v, c.Do(ctx, http.MethodPost, "/api/v1/projects", in, &v)
}
func (c *Client) UpdateProject(ctx context.Context, id int64, in ProjectInput) (Project, error) {
	var v Project
	return v, c.Do(ctx, http.MethodPut, fmt.Sprintf("/api/v1/projects/%d", id), in, &v)
}
func (c *Client) DeleteProject(ctx context.Context, id int64) error {
	return c.Do(ctx, http.MethodDelete, fmt.Sprintf("/api/v1/projects/%d", id), nil, nil)
}

func (c *Client) Subprojects(ctx context.Context, projectID int64) (SubprojectsResponse, error) {
	var v SubprojectsResponse
	path := "/api/v1/subprojects"
	if projectID > 0 {
		path += "?project_id=" + url.QueryEscape(fmt.Sprint(projectID))
	}
	return v, c.Do(ctx, http.MethodGet, path, nil, &v)
}
func (c *Client) Subproject(ctx context.Context, id int64) (Subproject, error) {
	var v Subproject
	return v, c.Do(ctx, http.MethodGet, fmt.Sprintf("/api/v1/subprojects/%d", id), nil, &v)
}
func (c *Client) CreateSubproject(ctx context.Context, in SubprojectInput) (Subproject, error) {
	var v Subproject
	return v, c.Do(ctx, http.MethodPost, "/api/v1/subprojects", in, &v)
}
func (c *Client) UpdateSubproject(ctx context.Context, id int64, in SubprojectInput) (Subproject, error) {
	var v Subproject
	return v, c.Do(ctx, http.MethodPut, fmt.Sprintf("/api/v1/subprojects/%d", id), in, &v)
}
func (c *Client) DeleteSubproject(ctx context.Context, id int64) error {
	return c.Do(ctx, http.MethodDelete, fmt.Sprintf("/api/v1/subprojects/%d", id), nil, nil)
}

func (c *Client) Tasks(ctx context.Context, all, ideas bool, projectID, subprojectID int64) (TasksResponse, error) {
	path := "/api/v1/tasks"
	if all {
		path += "/all"
	}
	values := url.Values{}
	if ideas {
		values.Set("ideas", "true")
	}
	if projectID > 0 {
		values.Set("project_id", fmt.Sprint(projectID))
	}
	if subprojectID > 0 {
		values.Set("subproject_id", fmt.Sprint(subprojectID))
	}
	if len(values) > 0 {
		path += "?" + values.Encode()
	}
	var v TasksResponse
	return v, c.Do(ctx, http.MethodGet, path, nil, &v)
}
func (c *Client) Task(ctx context.Context, id int64) (Task, error) {
	var v Task
	return v, c.Do(ctx, http.MethodGet, fmt.Sprintf("/api/v1/tasks/%d", id), nil, &v)
}
func (c *Client) CreateTask(ctx context.Context, in TaskInput) (Task, error) {
	var v Task
	return v, c.Do(ctx, http.MethodPost, "/api/v1/tasks", in, &v)
}
func (c *Client) UpdateTask(ctx context.Context, id int64, in TaskInput) (Task, error) {
	var v Task
	return v, c.Do(ctx, http.MethodPut, fmt.Sprintf("/api/v1/tasks/%d", id), in, &v)
}
func (c *Client) DeleteTask(ctx context.Context, id int64) error {
	return c.Do(ctx, http.MethodDelete, fmt.Sprintf("/api/v1/tasks/%d", id), nil, nil)
}
func (c *Client) UpdateTaskWeek(ctx context.Context, id int64, week string, in WeekPatch) (WeekCell, error) {
	var v WeekCell
	return v, c.Do(ctx, http.MethodPut, fmt.Sprintf("/api/v1/tasks/%d/weeks/%s", id, url.PathEscape(week)), in, &v)
}
func (c *Client) SetMonthLock(ctx context.Context, unlocked bool) (MonthLockState, error) {
	var v MonthLockState
	action := "lock"
	if unlocked {
		action = "unlock"
	}
	return v, c.Do(ctx, http.MethodPost, "/api/v1/month-locks/"+action, nil, &v)
}
func (c *Client) SCurve(ctx context.Context, id int64) (SCurve, error) {
	var v SCurve
	return v, c.Do(ctx, http.MethodGet, fmt.Sprintf("/api/v1/projects/%d/s-curve", id), nil, &v)
}
