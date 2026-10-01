package task

import (
	"context"
	"database/sql"
	"errors"
	"strings"

	"cad-development/internal/db"
	"cad-development/internal/nullable"
	"cad-development/internal/web"
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
	Name                nullable.Optional[string] `json:"name,omitzero"`
	Description         nullable.Optional[string] `json:"description,omitzero"`
	ImplementationNotes nullable.Optional[string] `json:"implementation_notes,omitzero"`
	Department          nullable.Optional[string] `json:"department,omitzero"`
	Developers          nullable.Optional[string] `json:"developers,omitzero"`
	Priority            nullable.Optional[string] `json:"priority,omitzero"`
	ProjectID           nullable.Optional[*int64] `json:"project_id,omitzero"`
	SubprojectID        nullable.Optional[*int64] `json:"subproject_id,omitzero"`
}

func PatchFromInput(in Input) Patch {
	return Patch{
		Name:                nullable.Present(in.Name),
		Description:         nullable.Present(in.Description),
		ImplementationNotes: nullable.Present(in.ImplementationNotes),
		Department:          nullable.Present(in.Department),
		Developers:          nullable.Present(in.Developers),
		Priority:            nullable.Present(in.Priority),
		ProjectID:           nullable.Present(in.ProjectID),
		SubprojectID:        nullable.Present(in.SubprojectID),
	}
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

func validate(in Input) (Input, error) {
	in.Name = strings.TrimSpace(in.Name)
	in.Description = strings.TrimSpace(in.Description)
	in.ImplementationNotes = strings.TrimSpace(in.ImplementationNotes)
	in.Department = strings.TrimSpace(in.Department)
	in.Developers = strings.TrimSpace(in.Developers)
	in.Priority = strings.TrimSpace(in.Priority)
	if in.Name == "" {
		return in, web.Invalid("name is required")
	}
	if in.ProjectID != nil && *in.ProjectID < 1 {
		return in, web.Invalid("invalid project")
	}
	if in.SubprojectID != nil && *in.SubprojectID < 1 {
		return in, web.Invalid("invalid subproject")
	}
	return in, nil
}

func Create(ctx context.Context, q *db.Queries, in Input) (Task, error) {
	validated, err := validate(in)
	if err != nil {
		return Task{}, err
	}
	var id int64
	err = q.InTx(ctx, func(txq *db.Queries) error {
		conflict, err := txq.TaskAssignmentConflict(ctx, db.TaskAssignmentConflictParams{
			ProjectID: nullable.Int64(validated.ProjectID), SubprojectID: nullable.Int64(validated.SubprojectID),
		})
		if err != nil {
			return err
		}
		if err := web.HTTPErrorFromReason(int(conflict.Status), conflict.Reason); err != nil {
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
		if err == nil {
			id = row.ID
		}
		return err
	})
	if err != nil {
		return Task{}, err
	}
	return Get(ctx, q, id)
}

func Get(ctx context.Context, q *db.Queries, id int64) (Task, error) {
	row, err := q.GetTask(ctx, id)
	if errors.Is(err, sql.ErrNoRows) {
		return Task{}, web.Missing("task not found")
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
	for _, w := range weeks {
		ws, err := weekly.Parse(w.WeekStart)
		if err != nil {
			return Task{}, err
		}
		c := weekly.Cell{
			TaskID:       w.TaskID,
			WeekStart:    ws,
			PlannedHours: w.PlannedHours,
			SpentHours:   w.SpentHours,
		}
		c.Progress = new(w.EffectiveProgress)
		out.Weeks = append(out.Weeks, c)
	}
	return out, nil
}

func List(ctx context.Context, q *db.Queries) ([]Task, error) {
	return withTotals(ctx, q, q.ListTasks, q.ListTaskTotalsAll)
}

func ListIdeas(ctx context.Context, q *db.Queries) ([]Task, error) {
	return withTotals(ctx, q, q.ListIdeaTasks, q.ListTaskTotalsIdeas)
}

func ListByProject(ctx context.Context, q *db.Queries, projectID int64) ([]Task, error) {
	return withTotals(ctx, q, func(c context.Context) ([]db.Task, error) {
		return q.ListTasksByProject(c, nullable.Int64(&projectID))
	}, func(c context.Context) ([]db.VTaskTotal, error) {
		return q.ListTaskTotalsByProject(c, projectID)
	})
}

func ListBySubproject(ctx context.Context, q *db.Queries, subprojectID int64) ([]Task, error) {
	return withTotals(ctx, q, func(c context.Context) ([]db.Task, error) {
		return q.ListTasksBySubproject(c, nullable.Int64(&subprojectID))
	}, func(c context.Context) ([]db.VTaskTotal, error) {
		return q.ListTaskTotalsBySubproject(c, subprojectID)
	})
}

func Update(ctx context.Context, q *db.Queries, id int64, patch Patch) (Task, error) {
	err := q.InTx(ctx, func(txq *db.Queries) error {
		current, err := txq.GetTask(ctx, id)
		if errors.Is(err, sql.ErrNoRows) {
			return web.Missing("task not found")
		}
		if err != nil {
			return err
		}
		if patch.ProjectID.Present && patch.ProjectID.Value == nil && current.SubprojectID.Valid &&
			(!patch.SubprojectID.Present || patch.SubprojectID.Value != nil) {
			return web.Invalid("project cannot be cleared while subproject is assigned")
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
		in, err = validate(in)
		if err != nil {
			return err
		}
		conflict, err := txq.TaskAssignmentConflict(ctx, db.TaskAssignmentConflictParams{
			ProjectID: nullable.Int64(in.ProjectID), SubprojectID: nullable.Int64(in.SubprojectID),
		})
		if err != nil {
			return err
		}
		if err := web.HTTPErrorFromReason(int(conflict.Status), conflict.Reason); err != nil {
			return err
		}
		projectChanged := patch.ProjectID.Present && current.ProjectID != nullable.Int64(in.ProjectID)
		subprojectChanged := patch.SubprojectID.Present && current.SubprojectID != nullable.Int64(in.SubprojectID)
		if projectChanged || subprojectChanged {
			reassign, err := txq.TaskReassignConflict(ctx, id)
			if err != nil {
				return err
			}
			if err := web.HTTPErrorFromReason(int(reassign.Status), reassign.Reason); err != nil {
				return err
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
	err := q.InTx(ctx, func(txq *db.Queries) error {
		_, err := txq.DeleteTask(ctx, id)
		if errors.Is(err, sql.ErrNoRows) {
			return web.Missing("task not found")
		}
		return err
	})
	return web.ReferencedConflict(err)
}

func withTotals(ctx context.Context, q *db.Queries, list func(context.Context) ([]db.Task, error), loadTotals func(context.Context) ([]db.VTaskTotal, error)) ([]Task, error) {
	rows, err := list(ctx)
	if err != nil {
		return nil, err
	}
	totals, err := loadTotals(ctx)
	if err != nil {
		return nil, err
	}
	byID := make(map[int64]db.VTaskTotal, len(totals))
	for _, total := range totals {
		byID[total.TaskID] = total
	}
	out := make([]Task, 0, len(rows))
	for _, row := range rows {
		t := FromDB(row)
		total := byID[t.ID]
		t.TotalHours, t.SpentHours, t.Progress, t.Status = total.PlannedHours, total.SpentHours, total.Progress, Status(total.Progress)
		out = append(out, t)
	}
	return out, nil
}
