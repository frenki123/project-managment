package task

import (
	"context"
	"database/sql"
	"errors"
	"strings"

	"cad-development/internal/app"
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
	Status              string        `json:"status"`
	Weeks               []weekly.Cell `json:"weeks,omitempty"`
}

// Status returns the task status derived from its cumulative progress.
func Status(progress float64) string {
	switch {
	case progress <= 0:
		return "Planned"
	case progress < 80:
		return "Development"
	case progress < 100:
		return "Internal Testing"
	default:
		return "Deployment"
	}
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

type Patch struct {
	Name                nullable.Optional[string] `json:"name"`
	Description         nullable.Optional[string] `json:"description"`
	ImplementationNotes nullable.Optional[string] `json:"implementation_notes"`
	Department          nullable.Optional[string] `json:"department"`
	Developers          nullable.Optional[string] `json:"developers"`
	Priority            nullable.Optional[string] `json:"priority"`
	ProjectID           nullable.Optional[*int64] `json:"project_id"`
	SubprojectID        nullable.Optional[*int64] `json:"subproject_id"`
}

func (in Input) Patch() Patch {
	return Patch{
		Name:                *nullable.Set(in.Name),
		Description:         *nullable.Set(in.Description),
		ImplementationNotes: *nullable.Set(in.ImplementationNotes),
		Department:          *nullable.Set(in.Department),
		Developers:          *nullable.Set(in.Developers),
		Priority:            *nullable.Set(in.Priority),
		ProjectID:           optionalID(in.ProjectID),
		SubprojectID:        optionalID(in.SubprojectID),
	}
}

func optionalID(value *int64) nullable.Optional[*int64] {
	return nullable.Optional[*int64]{Value: &value, Set: true}
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
		return in, app.Invalid("name is required")
	}
	if in.ProjectID != nil && *in.ProjectID < 1 {
		return in, app.Invalid("invalid project_id")
	}
	if in.SubprojectID != nil && *in.SubprojectID < 1 {
		return in, app.Invalid("invalid subproject_id")
	}
	if in.SubprojectID != nil {
		sp, err := q.GetSubproject(ctx, *in.SubprojectID)
		if errors.Is(err, sql.ErrNoRows) {
			return in, app.Missing("subproject not found")
		}
		if err != nil {
			return in, err
		}
		if in.ProjectID != nil && *in.ProjectID != sp.ProjectID {
			return in, app.Invalid("subproject does not belong to project")
		}
		in.ProjectID = new(sp.ProjectID)
	}
	if in.ProjectID != nil {
		if _, err := q.GetProject(ctx, *in.ProjectID); errors.Is(err, sql.ErrNoRows) {
			return in, app.Missing("project not found")
		} else if err != nil {
			return in, err
		}
	}
	return in, nil
}

func Create(ctx context.Context, q *db.Queries, in Input) (Task, error) {
	validated, err := validate(ctx, q, in)
	if err != nil {
		return Task{}, err
	}
	row, err := q.CreateTask(ctx, db.CreateTaskParams{
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
		return Task{}, err
	}
	return Get(ctx, q, row.ID)
}

func Get(ctx context.Context, q *db.Queries, id int64) (Task, error) {
	row, err := q.GetTask(ctx, id)
	if errors.Is(err, sql.ErrNoRows) {
		return Task{}, app.Missing("task not found")
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
	out.Status = Status(out.Progress)
	out.Weeks = make([]weekly.Cell, 0, len(weeks))
	requested := make([]string, 0, len(weeks))
	for _, w := range weeks {
		requested = append(requested, w.WeekStart)
	}
	effective := weekly.EffectiveProgress(weeks, requested)
	for _, w := range weeks {
		c := weekly.Cell{
			TaskID:       w.TaskID,
			WeekStart:    weekly.WeekStart(w.WeekStart),
			PlannedHours: w.PlannedHours,
			SpentHours:   w.SpentHours,
		}
		c.Progress = new(effective[w.WeekStart])
		out.Weeks = append(out.Weeks, c)
	}
	return out, nil
}

func List(ctx context.Context, q *db.Queries) ([]Task, error) {
	rows, err := q.ListTasks(ctx)
	if err != nil {
		return nil, err
	}
	return mapTasks(ctx, q, rows, "all", 0)
}

func ListIdeas(ctx context.Context, q *db.Queries) ([]Task, error) {
	rows, err := q.ListIdeaTasks(ctx)
	if err != nil {
		return nil, err
	}
	return mapTasks(ctx, q, rows, "ideas", 0)
}

func ListByProject(ctx context.Context, q *db.Queries, projectID int64) ([]Task, error) {
	rows, err := q.ListTasksByProject(ctx, nullable.Int64(&projectID))
	if err != nil {
		return nil, err
	}
	return mapTasks(ctx, q, rows, "project", projectID)
}

func ListBySubproject(ctx context.Context, q *db.Queries, subprojectID int64) ([]Task, error) {
	rows, err := q.ListTasksBySubproject(ctx, nullable.Int64(&subprojectID))
	if err != nil {
		return nil, err
	}
	return mapTasks(ctx, q, rows, "subproject", subprojectID)
}

func Update(ctx context.Context, q *db.Queries, id int64, patch Patch) (Task, error) {
	err := q.InTx(ctx, func(txq *db.Queries) error {
		current, err := txq.GetTask(ctx, id)
		if errors.Is(err, sql.ErrNoRows) {
			return app.Missing("task not found")
		}
		if err != nil {
			return err
		}
		currentInput := Input{
			Name: current.Name, Description: current.Description,
			ImplementationNotes: current.ImplementationNotes, Department: current.Department,
			Developers: current.Developers, Priority: current.Priority,
			ProjectID:    nullable.Int64Pointer(current.ProjectID),
			SubprojectID: nullable.Int64Pointer(current.SubprojectID),
		}
		in := Input{
			Name:                patch.Name.Apply(currentInput.Name),
			Description:         patch.Description.Apply(currentInput.Description),
			ImplementationNotes: patch.ImplementationNotes.Apply(currentInput.ImplementationNotes),
			Department:          patch.Department.Apply(currentInput.Department),
			Developers:          patch.Developers.Apply(currentInput.Developers),
			Priority:            patch.Priority.Apply(currentInput.Priority),
			ProjectID:           patch.ProjectID.Apply(currentInput.ProjectID),
			SubprojectID:        patch.SubprojectID.Apply(currentInput.SubprojectID),
		}
		in, err = validate(ctx, txq, in)
		if err != nil {
			return err
		}
		projectChanged := patch.ProjectID.Set && current.ProjectID != nullable.Int64(in.ProjectID)
		subprojectChanged := patch.SubprojectID.Set && current.SubprojectID != nullable.Int64(in.SubprojectID)
		if projectChanged || subprojectChanged {
			weekCount, err := txq.CountTaskWeeksByTask(ctx, id)
			if err != nil {
				return err
			}
			if weekCount > 0 {
				return app.Conflict("cannot reassign task with weekly data")
			}
		}
		_, err = txq.UpdateTask(ctx, db.UpdateTaskParams{
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
		return err
	})
	if err != nil {
		return Task{}, err
	}
	return Get(ctx, q, id)
}

func Delete(ctx context.Context, q *db.Queries, id int64) error {
	return q.InTx(ctx, func(txq *db.Queries) error {
		if _, err := txq.GetTask(ctx, id); errors.Is(err, sql.ErrNoRows) {
			return app.Missing("task not found")
		} else if err != nil {
			return err
		}
		weeks, err := txq.CountTaskWeeksByTask(ctx, id)
		if err != nil {
			return err
		}
		if weeks > 0 {
			return app.Conflict("cannot delete a task with weekly history")
		}
		_, err = txq.DeleteTask(ctx, id)
		return err
	})
}

func mapTasks(ctx context.Context, q *db.Queries, rows []db.Task, scope string, ownerID int64) ([]Task, error) {
	if len(rows) == 0 {
		return []Task{}, nil
	}
	totals, err := q.ListTaskTotals(ctx, db.ListTaskTotalsParams{Scope: scope, OwnerID: ownerID})
	if err != nil {
		return nil, err
	}
	byID := make(map[int64]db.ListTaskTotalsRow, len(totals))
	for _, total := range totals {
		byID[total.TaskID] = total
	}
	out := make([]Task, 0, len(rows))
	for _, row := range rows {
		t := FromDB(row)
		total := byID[t.ID]
		t.TotalHours = total.PlannedHours
		t.SpentHours = total.SpentHours
		t.Progress = total.Progress
		t.Status = Status(t.Progress)
		out = append(out, t)
	}
	return out, nil
}
