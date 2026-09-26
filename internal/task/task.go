package task

import (
	"context"
	"database/sql"
	"errors"
	"strings"

	"cad-development/internal/db"
	"cad-development/internal/nullable"
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

type TasksResponse struct {
	Tasks []Task `json:"tasks"`
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
	out.ProjectID = nullable.Int64Pointer(t.ProjectID)
	out.SubprojectID = nullable.Int64Pointer(t.SubprojectID)
	return out
}

func validate(ctx context.Context, q *db.Queries, in Input) (Input, error) {
	in.Name = strings.TrimSpace(in.Name)
	in.Description = strings.TrimSpace(in.Description)
	in.ImplementationNotes = strings.TrimSpace(in.ImplementationNotes)
	in.Department = strings.TrimSpace(in.Department)
	in.Developers = strings.TrimSpace(in.Developers)
	in.Priority = strings.TrimSpace(in.Priority)
	if in.Name == "" {
		return in, Invalid("name is required")
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
			return in, Missing("subproject not found")
		}
		if err != nil {
			return in, err
		}
		if in.ProjectID != nil && *in.ProjectID != sp.ProjectID {
			return in, Invalid("subproject does not belong to project")
		}
		in.ProjectID = new(sp.ProjectID)
	}
	if in.ProjectID != nil {
		if _, err := q.GetProject(ctx, *in.ProjectID); errors.Is(err, sql.ErrNoRows) {
			return in, Missing("project not found")
		} else if err != nil {
			return in, err
		}
	}
	return in, nil
}

func Create(ctx context.Context, q *db.Queries, in Input) (Task, error) {
	var result Task
	err := q.InTx(ctx, func(txq *db.Queries) error {
		validated, err := validate(ctx, txq, in)
		if err != nil {
			return err
		}
		row, err := txq.CreateTask(ctx, db.CreateTaskParams{
			Name:                validated.Name,
			Description:         validated.Description,
			ImplementationNotes: validated.ImplementationNotes,
			Department:          validated.Department,
			Developers:          validated.Developers,
			Priority:            validated.Priority,
			ProjectID:           nullable.Int64(validated.ProjectID),
			SubprojectID:        nullable.Int64(validated.SubprojectID),
		})
		if err != nil {
			return err
		}
		result = FromDB(row)
		return nil
	})
	return result, err
}

func Get(ctx context.Context, q *db.Queries, id int64) (Task, error) {
	row, err := q.GetTask(ctx, id)
	if errors.Is(err, sql.ErrNoRows) {
		return Task{}, Missing("task not found")
	}
	if err != nil {
		return Task{}, err
	}
	out := FromDB(row)
	totals, err := q.GetTaskTotals(ctx, id)
	if err != nil {
		return Task{}, err
	}
	weeks, err := q.ListTaskWeeksByTask(ctx, id)
	if err != nil {
		return Task{}, err
	}
	out.TotalHours = totals.PlannedHours
	out.SpentHours = totals.SpentHours
	out.Progress = totals.Progress
	out.Weeks = make([]weekly.Cell, 0, len(weeks))
	for _, w := range weeks {
		c := weekly.Cell{
			TaskID:       w.TaskID,
			WeekStart:    weekly.WeekStart(w.WeekStart),
			PlannedHours: w.PlannedHours,
			SpentHours:   w.SpentHours,
		}
		c.Progress = nullable.Float64Pointer(w.Progress)
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
	rows, err := q.ListTasksByProject(ctx, nullable.Int64(&projectID))
	if err != nil {
		return nil, err
	}
	return mapTasks(rows), nil
}

func ListBySubproject(ctx context.Context, q *db.Queries, subprojectID int64) ([]Task, error) {
	rows, err := q.ListTasksBySubproject(ctx, nullable.Int64(&subprojectID))
	if err != nil {
		return nil, err
	}
	return mapTasks(rows), nil
}

func Update(ctx context.Context, q *db.Queries, id int64, in Input) (Task, error) {
	var result Task
	err := q.InTx(ctx, func(txq *db.Queries) error {
		current, err := txq.GetTask(ctx, id)
		if errors.Is(err, sql.ErrNoRows) {
			return Missing("task not found")
		}
		if err != nil {
			return err
		}
		in, err = validate(ctx, txq, in)
		if err != nil {
			return err
		}
		projectChanged := current.ProjectID != nullable.Int64(in.ProjectID)
		subprojectChanged := current.SubprojectID != nullable.Int64(in.SubprojectID)
		if projectChanged || subprojectChanged {
			weekCount, err := txq.CountTaskWeeksByTask(ctx, id)
			if err != nil {
				return err
			}
			if weekCount > 0 {
				return ConflictError("cannot reassign task with weekly data")
			}
		}
		row, err := txq.UpdateTask(ctx, db.UpdateTaskParams{
			Name:                in.Name,
			Description:         in.Description,
			ImplementationNotes: in.ImplementationNotes,
			Department:          in.Department,
			Developers:          in.Developers,
			Priority:            in.Priority,
			ProjectID:           nullable.Int64(in.ProjectID),
			SubprojectID:        nullable.Int64(in.SubprojectID),
			ID:                  id,
		})
		if err != nil {
			return err
		}
		result = FromDB(row)
		return nil
	})
	return result, err
}

func Delete(ctx context.Context, q *db.Queries, id int64) error {
	return q.InTx(ctx, func(txq *db.Queries) error {
		if _, err := txq.GetTask(ctx, id); errors.Is(err, sql.ErrNoRows) {
			return Missing("task not found")
		} else if err != nil {
			return err
		}
		weeks, err := txq.CountTaskWeeksByTask(ctx, id)
		if err != nil {
			return err
		}
		if weeks > 0 {
			return ConflictError("cannot delete a task with weekly history")
		}
		_, err = txq.DeleteTask(ctx, id)
		return err
	})
}

func mapTasks(rows []db.Task) []Task {
	out := make([]Task, 0, len(rows))
	for _, row := range rows {
		out = append(out, FromDB(row))
	}
	return out
}
