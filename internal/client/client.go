package client

import (
	"bufio"
	"bytes"
	"context"
	json "encoding/json/v2"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"cad-development/internal/project"
	"cad-development/internal/subproject"
	"cad-development/internal/task"
	"cad-development/internal/weekly"
)

const DefaultURL = "http://127.0.0.1:8080"

const maxAPIErrorBody = 64 << 10

type Client struct {
	BaseURL *url.URL
	HTTP    *http.Client
}

type APIError struct {
	Method  string
	Path    string
	Status  int
	Message string
	Reason  string
}

func (e *APIError) Error() string {
	if e.Message == "" {
		return fmt.Sprintf("HTTP %d from %s %s", e.Status, e.Method, e.Path)
	}
	return fmt.Sprintf("HTTP %d: %s", e.Status, e.Message)
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

func (c *Client) Do[In, Out any](ctx context.Context, method, path string, input In, output *Out) error {
	data, err := json.Marshal(input)
	if err != nil {
		return fmt.Errorf("encode request: %w", err)
	}
	return c.do(ctx, method, path, bytes.NewReader(data), output)
}

func (c *Client) DoNoBody[Out any](ctx context.Context, method, path string, output *Out) error {
	return c.do(ctx, method, path, nil, output)
}

func (c *Client) do[Out any](ctx context.Context, method, path string, body io.Reader, output *Out) error {
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
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	req.Header.Set("Accept", "application/json")
	resp, err := c.HTTP.Do(req)
	if err != nil {
		return fmt.Errorf("request %s %s: %w", method, path, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		data, err := io.ReadAll(io.LimitReader(resp.Body, maxAPIErrorBody+1))
		if err != nil {
			return fmt.Errorf("read response: %w", err)
		}
		var payload struct {
			Error  string `json:"error"`
			Reason string `json:"reason"`
		}
		_ = json.Unmarshal(data, &payload)
		message := strings.TrimSpace(payload.Error)
		if message == "" {
			message = strings.TrimSpace(string(data))
		}
		if len(message) > maxAPIErrorBody {
			message = message[:maxAPIErrorBody] + "..."
		}
		return &APIError{Method: method, Path: path, Status: resp.StatusCode, Message: message, Reason: strings.TrimSpace(payload.Reason)}
	}
	if resp.StatusCode == http.StatusNoContent {
		return nil
	}
	if output == nil {
		if _, err := io.Copy(io.Discard, resp.Body); err != nil {
			return fmt.Errorf("read response: %w", err)
		}
		return nil
	}
	reader := bufio.NewReader(resp.Body)
	if _, err := reader.Peek(1); err != nil {
		if errors.Is(err, io.EOF) {
			return nil
		}
		return fmt.Errorf("read response: %w", err)
	}
	if err := json.UnmarshalRead(reader, output); err != nil {
		return fmt.Errorf("decode response: %w", err)
	}
	return nil
}

func (c *Client) Projects(ctx context.Context) (project.ProjectsResponse, error) {
	var v project.ProjectsResponse
	return v, c.DoNoBody(ctx, http.MethodGet, "/api/v1/projects", &v)
}
func (c *Client) Project(ctx context.Context, id int64) (project.Project, error) {
	var v project.Project
	return v, c.DoNoBody(ctx, http.MethodGet, fmt.Sprintf("/api/v1/projects/%d", id), &v)
}
func (c *Client) CreateProject(ctx context.Context, in project.Input) (project.Project, error) {
	var v project.Project
	return v, c.Do(ctx, http.MethodPost, "/api/v1/projects", in, &v)
}
func (c *Client) UpdateProject(ctx context.Context, id int64, in project.Patch) (project.Project, error) {
	var v project.Project
	return v, c.Do(ctx, http.MethodPut, fmt.Sprintf("/api/v1/projects/%d", id), in, &v)
}
func (c *Client) DeleteProject(ctx context.Context, id int64) error {
	return c.DoNoBody(ctx, http.MethodDelete, fmt.Sprintf("/api/v1/projects/%d", id), (*struct{})(nil))
}

func (c *Client) Subprojects(ctx context.Context, projectID *int64) (subproject.SubprojectsResponse, error) {
	var v subproject.SubprojectsResponse
	path := "/api/v1/subprojects"
	if projectID != nil {
		path += "?project_id=" + url.QueryEscape(fmt.Sprint(*projectID))
	}
	return v, c.DoNoBody(ctx, http.MethodGet, path, &v)
}
func (c *Client) Subproject(ctx context.Context, id int64) (subproject.Subproject, error) {
	var v subproject.Subproject
	return v, c.DoNoBody(ctx, http.MethodGet, fmt.Sprintf("/api/v1/subprojects/%d", id), &v)
}
func (c *Client) CreateSubproject(ctx context.Context, in subproject.Input) (subproject.Subproject, error) {
	var v subproject.Subproject
	return v, c.Do(ctx, http.MethodPost, "/api/v1/subprojects", in, &v)
}
func (c *Client) UpdateSubproject(ctx context.Context, id int64, in subproject.Patch) (subproject.Subproject, error) {
	var v subproject.Subproject
	return v, c.Do(ctx, http.MethodPut, fmt.Sprintf("/api/v1/subprojects/%d", id), in, &v)
}
func (c *Client) DeleteSubproject(ctx context.Context, id int64) error {
	return c.DoNoBody(ctx, http.MethodDelete, fmt.Sprintf("/api/v1/subprojects/%d", id), (*struct{})(nil))
}

func (c *Client) Tasks(ctx context.Context, ideas bool, projectID, subprojectID *int64) (task.TasksResponse, error) {
	path := "/api/v1/tasks"
	values := url.Values{}
	if ideas {
		values.Set("ideas", "true")
	}
	if projectID != nil {
		values.Set("project_id", fmt.Sprint(*projectID))
	}
	if subprojectID != nil {
		values.Set("subproject_id", fmt.Sprint(*subprojectID))
	}
	if len(values) > 0 {
		path += "?" + values.Encode()
	}
	var v task.TasksResponse
	return v, c.DoNoBody(ctx, http.MethodGet, path, &v)
}
func (c *Client) Task(ctx context.Context, id int64) (task.Task, error) {
	var v task.Task
	return v, c.DoNoBody(ctx, http.MethodGet, fmt.Sprintf("/api/v1/tasks/%d", id), &v)
}
func (c *Client) CreateTask(ctx context.Context, in task.Input) (task.Task, error) {
	var v task.Task
	return v, c.Do(ctx, http.MethodPost, "/api/v1/tasks", in, &v)
}
func (c *Client) UpdateTask(ctx context.Context, id int64, in task.Patch) (task.Task, error) {
	var v task.Task
	return v, c.Do(ctx, http.MethodPut, fmt.Sprintf("/api/v1/tasks/%d", id), in, &v)
}
func (c *Client) DeleteTask(ctx context.Context, id int64) error {
	return c.DoNoBody(ctx, http.MethodDelete, fmt.Sprintf("/api/v1/tasks/%d", id), (*struct{})(nil))
}
func (c *Client) UpdateTaskWeek(ctx context.Context, id int64, week string, in weekly.Patch) (weekly.Cell, error) {
	var v weekly.Cell
	return v, c.Do(ctx, http.MethodPut, fmt.Sprintf("/api/v1/tasks/%d/weeks/%s", id, url.PathEscape(week)), in, &v)
}
func (c *Client) SCurve(ctx context.Context, id int64) (project.SCurve, error) {
	var v project.SCurve
	return v, c.DoNoBody(ctx, http.MethodGet, fmt.Sprintf("/api/v1/projects/%d/s-curve", id), &v)
}
