package task

import (
	"context"
	"database/sql"
	"errors"
	"net/http"
	"strings"

	"cad-development/internal/apperr"
	"cad-development/internal/db"
	"cad-development/internal/weekly"
)

type Task struct {
	ID                  int64         `json:"id"`
	Name                string        `json:"name"`
	Description         string        `json:"description"`
	ImplementationNotes string        `json:"implementation_notes"`
	Department          string        `json:"department"`
	Developers          string        `json:"developers"`
	Priority            string        `json:"priority"`
	ProjectID           *int64        `json:"project_id"`
	SubprojectID        *int64        `json:"subproject_id"`
	TotalHours          float64       `json:"total_hours,omitempty"`
	SpentHours          float64       `json:"spent_hours,omitempty"`
	Progress            float64       `json:"progress,omitempty"`
	Weeks               []weekly.Cell `json:"weeks,omitempty"`
}

type Input struct {
	Name                string `json:"name"`
	Description         string `json:"description"`
	ImplementationNotes string `json:"implementation_notes"`
	Department          string `json:"department"`
	Developers          string `json:"developers"`
	Priority            string `json:"priority"`
	ProjectID           *int64 `json:"project_id"`
	SubprojectID        *int64 `json:"subproject_id"`
}

func FromDB(t db.Task) Task {
	out := Task{
		ID:                  t.ID,
		Name:                t.Name,
		Description:         t.Description,
		ImplementationNotes: t.ImplementationNotes,
		Department:          t.Department,
		Developers:          t.Developers,
		Priority:            t.Priority,
	}
	if t.ProjectID.Valid {
		id := t.ProjectID.Int64
		out.ProjectID = &id
	}
	if t.SubprojectID.Valid {
		id := t.SubprojectID.Int64
		out.SubprojectID = &id
	}
	return out
}

func nullID(p *int64) sql.NullInt64 {
	if p == nil || *p < 1 {
		return sql.NullInt64{}
	}
	return sql.NullInt64{Int64: *p, Valid: true}
}

func validate(ctx context.Context, q *db.Queries, in Input) (Input, error) {
	in.Name = strings.TrimSpace(in.Name)
	in.Description = strings.TrimSpace(in.Description)
	in.ImplementationNotes = strings.TrimSpace(in.ImplementationNotes)
	in.Department = strings.TrimSpace(in.Department)
	in.Developers = strings.TrimSpace(in.Developers)
	in.Priority = strings.TrimSpace(in.Priority)
	if in.Name == "" {
		return in, apperr.New(http.StatusBadRequest, "name is required")
	}
	if in.ProjectID != nil && *in.ProjectID < 1 {
		in.ProjectID = nil
	}
	if in.SubprojectID != nil && *in.SubprojectID < 1 {
		in.SubprojectID = nil
	}
	if in.SubprojectID != nil {
		sp, err := q.GetSubproject(ctx, *in.SubprojectID)
		if errors.Is(err, sql.ErrNoRows) {
			return in, apperr.New(http.StatusBadRequest, "subproject not found")
		}
		if err != nil {
			return in, err
		}
		if in.ProjectID != nil && *in.ProjectID != sp.ProjectID {
			return in, apperr.New(http.StatusBadRequest, "subproject does not belong to project")
		}
		pid := sp.ProjectID
		in.ProjectID = &pid
	}
	if in.ProjectID != nil {
		if _, err := q.GetProject(ctx, *in.ProjectID); errors.Is(err, sql.ErrNoRows) {
			return in, apperr.New(http.StatusBadRequest, "project not found")
		} else if err != nil {
			return in, err
		}
	}
	return in, nil
}

func Create(ctx context.Context, q *db.Queries, in Input) (Task, error) {
	in, err := validate(ctx, q, in)
	if err != nil {
		return Task{}, err
	}
	row, err := q.CreateTask(ctx, db.CreateTaskParams{
		Name:                in.Name,
		Description:         in.Description,
		ImplementationNotes: in.ImplementationNotes,
		Department:          in.Department,
		Developers:          in.Developers,
		Priority:            in.Priority,
		ProjectID:           nullID(in.ProjectID),
		SubprojectID:        nullID(in.SubprojectID),
	})
	if err != nil {
		return Task{}, err
	}
	return FromDB(row), nil
}

func Get(ctx context.Context, q *db.Queries, id int64) (Task, error) {
	row, err := q.GetTask(ctx, id)
	if errors.Is(err, sql.ErrNoRows) {
		return Task{}, apperr.New(http.StatusNotFound, "task not found")
	}
	if err != nil {
		return Task{}, err
	}
	out := FromDB(row)
	weeks, err := q.ListTaskWeeksByTask(ctx, id)
	if err != nil {
		return Task{}, err
	}
	out.TotalHours, out.SpentHours, out.Progress = weekly.Totals(weeks)
	out.Weeks = make([]weekly.Cell, 0, len(weeks))
	for _, w := range weeks {
		c := weekly.Cell{
			TaskID:       w.TaskID,
			WeekStart:    w.WeekStart,
			PlannedHours: w.PlannedHours,
			SpentHours:   w.SpentHours,
		}
		if w.Progress.Valid {
			v := w.Progress.Float64
			c.Progress = &v
		}
		out.Weeks = append(out.Weeks, c)
	}
	return out, nil
}

func List(ctx context.Context, q *db.Queries) ([]Task, error) {
	rows, err := q.ListTasks(ctx)
	if err != nil {
		return nil, err
	}
	return mapTasks(rows), nil
}

func ListIdeas(ctx context.Context, q *db.Queries) ([]Task, error) {
	rows, err := q.ListIdeaTasks(ctx)
	if err != nil {
		return nil, err
	}
	return mapTasks(rows), nil
}

func ListByProject(ctx context.Context, q *db.Queries, projectID int64) ([]Task, error) {
	rows, err := q.ListTasksByProject(ctx, sql.NullInt64{Int64: projectID, Valid: true})
	if err != nil {
		return nil, err
	}
	return mapTasks(rows), nil
}

func ListBySubproject(ctx context.Context, q *db.Queries, subprojectID int64) ([]Task, error) {
	rows, err := q.ListTasksBySubproject(ctx, sql.NullInt64{Int64: subprojectID, Valid: true})
	if err != nil {
		return nil, err
	}
	return mapTasks(rows), nil
}

func Update(ctx context.Context, q *db.Queries, id int64, in Input) (Task, error) {
	if _, err := q.GetTask(ctx, id); errors.Is(err, sql.ErrNoRows) {
		return Task{}, apperr.New(http.StatusNotFound, "task not found")
	} else if err != nil {
		return Task{}, err
	}
	in, err := validate(ctx, q, in)
	if err != nil {
		return Task{}, err
	}
	row, err := q.UpdateTask(ctx, db.UpdateTaskParams{
		Name:                in.Name,
		Description:         in.Description,
		ImplementationNotes: in.ImplementationNotes,
		Department:          in.Department,
		Developers:          in.Developers,
		Priority:            in.Priority,
		ProjectID:           nullID(in.ProjectID),
		SubprojectID:        nullID(in.SubprojectID),
		ID:                  id,
	})
	if err != nil {
		return Task{}, err
	}
	return FromDB(row), nil
}

func Delete(ctx context.Context, q *db.Queries, id int64) error {
	if _, err := q.GetTask(ctx, id); errors.Is(err, sql.ErrNoRows) {
		return apperr.New(http.StatusNotFound, "task not found")
	} else if err != nil {
		return err
	}
	return q.DeleteTask(ctx, id)
}

func mapTasks(rows []db.Task) []Task {
	out := make([]Task, 0, len(rows))
	for _, row := range rows {
		out = append(out, FromDB(row))
	}
	return out
}
