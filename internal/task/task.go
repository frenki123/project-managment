package task

import (
	"context"
	"database/sql"
	"errors"
	"net/http"
	"strings"

	"cad-development/internal/db"
	"cad-development/internal/nullable"
	"cad-development/internal/person"
	"cad-development/internal/web"
	"cad-development/internal/weekly"
)

type Task struct {
	ID                  int64         `json:"id"`
	Name                string        `json:"name"`
	Description         string        `json:"description"`
	ImplementationNotes string        `json:"implementation_notes"`
	Department          string        `json:"department"`
	Developers          []person.Ref  `json:"developers"`
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
	Name                string  `json:"name"`
	Description         string  `json:"description"`
	ImplementationNotes string  `json:"implementation_notes"`
	Department          string  `json:"department"`
	DeveloperIDs        []int64 `json:"developer_ids"`
	Priority            string  `json:"priority"`
	ProjectID           *int64  `json:"project_id"`
	SubprojectID        *int64  `json:"subproject_id"`
}

type Patch struct {
	Name                nullable.Optional[string]  `json:"name,omitzero"`
	Description         nullable.Optional[string]  `json:"description,omitzero"`
	ImplementationNotes nullable.Optional[string]  `json:"implementation_notes,omitzero"`
	Department          nullable.Optional[string]  `json:"department,omitzero"`
	DeveloperIDs        nullable.Optional[[]int64] `json:"developer_ids,omitzero"`
	Priority            nullable.Optional[string]  `json:"priority,omitzero"`
	ProjectID           nullable.Optional[*int64]  `json:"project_id,omitzero"`
	SubprojectID        nullable.Optional[*int64]  `json:"subproject_id,omitzero"`
}

func PatchFromInput(in Input) Patch {
	return Patch{
		Name:                nullable.Present(in.Name),
		Description:         nullable.Present(in.Description),
		ImplementationNotes: nullable.Present(in.ImplementationNotes),
		Department:          nullable.Present(in.Department),
		DeveloperIDs:        nullable.Present(in.DeveloperIDs),
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
		Developers:          []person.Ref{},
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
	seen := make(map[int64]struct{}, len(in.DeveloperIDs))
	for _, id := range in.DeveloperIDs {
		if id < 1 {
			return in, web.Invalid("invalid developer")
		}
		if _, ok := seen[id]; ok {
			return in, web.Invalid("duplicate developer")
		}
		seen[id] = struct{}{}
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
		if err := validateDevelopersExist(ctx, txq, validated.DeveloperIDs); err != nil {
			return err
		}
		row, err := txq.CreateTask(ctx, db.CreateTaskParams{
			Name:                validated.Name,
			Description:         validated.Description,
			ImplementationNotes: validated.ImplementationNotes,
			Department:          validated.Department,
			Priority:            validated.Priority,
			ProjectID:           nullable.Int64(validated.ProjectID),
			SubprojectID:        nullable.Int64(validated.SubprojectID),
		})
		if err != nil {
			return err
		}
		id = row.ID
		for _, personID := range validated.DeveloperIDs {
			if _, err := txq.AddTaskDeveloper(ctx, db.AddTaskDeveloperParams{TaskID: id, PersonID: personID}); err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		return Task{}, err
	}
	return Get(ctx, q, id)
}

func validateDevelopersExist(ctx context.Context, txq *db.Queries, ids []int64) error {
	for _, id := range ids {
		exists, err := txq.PersonExists(ctx, id)
		if err != nil {
			return err
		}
		if exists == 0 {
			return web.HTTPErrorFromReason(http.StatusNotFound, "person-not-found")
		}
	}
	return nil
}

func refs(rows []db.ListTaskDevelopersRow) []person.Ref {
	out := make([]person.Ref, 0, len(rows))
	for _, row := range rows {
		out = append(out, person.Ref{ID: row.ID, Name: row.Name})
	}
	return out
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
	weeks, err := LoadWeeks(ctx, q, id)
	if err != nil {
		return Task{}, err
	}
	developerRows, err := q.ListTaskDevelopers(ctx, id)
	if err != nil {
		return Task{}, err
	}
	out.Developers = refs(developerRows)
	out.TotalHours = totals.PlannedHours
	out.SpentHours = totals.SpentHours
	out.Progress = totals.Progress
	out.Status = Status(out.Progress)
	out.Weeks = weeks
	return out, nil
}

func List(ctx context.Context, q *db.Queries) ([]Task, error) {
	return listScoped(ctx, q, sql.NullInt64{}, sql.NullInt64{})
}

func ListIdeas(ctx context.Context, q *db.Queries) ([]Task, error) {
	rows, err := q.ListIdeaTasks(ctx)
	if err != nil {
		return nil, err
	}
	totals, err := q.ListTaskTotalsIdeas(ctx)
	if err != nil {
		return nil, err
	}
	return mergeTotals(ctx, q, rows, totals)
}

func ListByProject(ctx context.Context, q *db.Queries, projectID int64) ([]Task, error) {
	return listScoped(ctx, q, nullable.Int64(&projectID), sql.NullInt64{})
}

func ListBySubproject(ctx context.Context, q *db.Queries, subprojectID int64) ([]Task, error) {
	return listScoped(ctx, q, sql.NullInt64{}, nullable.Int64(&subprojectID))
}

func listScoped(ctx context.Context, q *db.Queries, projectID, subprojectID sql.NullInt64) ([]Task, error) {
	scope := db.ListTaskTotalsScopedParams{ProjectID: projectID, SubprojectID: subprojectID}
	rows, err := q.ListTasksScoped(ctx, db.ListTasksScopedParams(scope))
	if err != nil {
		return nil, err
	}
	totals, err := q.ListTaskTotalsScoped(ctx, scope)
	if err != nil {
		return nil, err
	}
	return mergeTotals(ctx, q, rows, totals)
}

func mergeTotals(ctx context.Context, q *db.Queries, rows []db.Task, totals []db.VTaskTotal) ([]Task, error) {
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
	developerRows, err := q.ListAllTaskDevelopers(ctx)
	if err != nil {
		return nil, err
	}
	byTask := make(map[int64][]person.Ref)
	for _, row := range developerRows {
		byTask[row.TaskID] = append(byTask[row.TaskID], person.Ref{ID: row.ID, Name: row.Name})
	}
	for i := range out {
		if devs, ok := byTask[out[i].ID]; ok {
			out[i].Developers = devs
		}
	}
	return out, nil
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
		in := mergeInput(current, patch)
		if patch.DeveloperIDs.Present {
			in = applyDeveloperPatch(in, patch)
			if err := validateDevelopersExist(ctx, txq, in.DeveloperIDs); err != nil {
				return err
			}
		}
		in, err = resolveAssignment(ctx, txq, id, current, patch, in)
		if err != nil {
			return err
		}
		return updateTaskWithDevelopers(ctx, txq, id, in, patch.DeveloperIDs.Present)
	})
	if err != nil {
		return Task{}, err
	}
	return Get(ctx, q, id)
}

func applyDeveloperPatch(in Input, patch Patch) Input {
	if patch.DeveloperIDs.Value == nil {
		in.DeveloperIDs = nil
	} else {
		in.DeveloperIDs = *patch.DeveloperIDs.Value
	}
	return in
}

func updateTaskWithDevelopers(ctx context.Context, txq *db.Queries, id int64, in Input, replaceDevelopers bool) error {
	_, err := txq.UpdateTask(ctx, db.UpdateTaskParams{ // the returned row cannot replace the post-update Get; only the error is needed
		Name: in.Name, Description: in.Description,
		ImplementationNotes: in.ImplementationNotes, Department: in.Department,
		Priority: in.Priority, ProjectID: nullable.Int64(in.ProjectID),
		SubprojectID: nullable.Int64(in.SubprojectID), ID: id,
	})
	if err != nil {
		return err
	}
	if !replaceDevelopers {
		return nil
	}
	if err := txq.ReplaceTaskDevelopers(ctx, id); err != nil {
		return err
	}
	for _, personID := range in.DeveloperIDs {
		if _, err := txq.AddTaskDeveloper(ctx, db.AddTaskDeveloperParams{TaskID: id, PersonID: personID}); err != nil {
			return err
		}
	}
	return nil
}

func mergeInput(current db.Task, patch Patch) Input {
	currentInput := Input{
		Name: current.Name, Description: current.Description,
		ImplementationNotes: current.ImplementationNotes, Department: current.Department,
		Priority: current.Priority,
		ProjectID:    nullable.Int64Pointer(current.ProjectID),
		SubprojectID: nullable.Int64Pointer(current.SubprojectID),
	}
	return Input{
		Name:                patch.Name.Apply(currentInput.Name),
		Description:         patch.Description.Apply(currentInput.Description),
		ImplementationNotes: patch.ImplementationNotes.Apply(currentInput.ImplementationNotes),
		Department:          patch.Department.Apply(currentInput.Department),
		Priority:            patch.Priority.Apply(currentInput.Priority),
		ProjectID:           patch.ProjectID.Apply(currentInput.ProjectID),
		SubprojectID:        patch.SubprojectID.Apply(currentInput.SubprojectID),
	}
}

func resolveAssignment(ctx context.Context, txq *db.Queries, id int64, current db.Task, patch Patch, in Input) (Input, error) {
	in, err := validate(in)
	if err != nil {
		return in, err
	}
	conflict, err := txq.TaskAssignmentConflict(ctx, db.TaskAssignmentConflictParams{
		ProjectID: nullable.Int64(in.ProjectID), SubprojectID: nullable.Int64(in.SubprojectID),
	})
	if err != nil {
		return in, err
	}
	if err := web.HTTPErrorFromReason(int(conflict.Status), conflict.Reason); err != nil {
		return in, err
	}
	projectChanged := patch.ProjectID.Present && current.ProjectID != nullable.Int64(in.ProjectID)
	subprojectChanged := patch.SubprojectID.Present && current.SubprojectID != nullable.Int64(in.SubprojectID)
	if projectChanged || subprojectChanged {
		reassign, err := txq.TaskReassignConflict(ctx, id)
		if err != nil {
			return in, err
		}
		if err := web.HTTPErrorFromReason(int(reassign.Status), reassign.Reason); err != nil {
			return in, err
		}
	}
	return in, nil
}

func Delete(ctx context.Context, q *db.Queries, id int64) error {
	err := q.InTx(ctx, func(txq *db.Queries) error {
		if err := txq.DeleteTaskDevelopers(ctx, id); err != nil {
			return err
		}
		rows, err := txq.DeleteTask(ctx, id)
		if err != nil {
			return err
		}
		if rows == 0 {
			return web.Missing("task not found")
		}
		return nil
	})
	return web.ReferencedConflict(err)
}