package subproject

import (
	"context"
	"database/sql"
	"errors"
	"strings"

	"cad-development/internal/app"
	"cad-development/internal/db"
	"cad-development/internal/nullable"
)

type Subproject struct {
	ID           int64   `json:"id"`
	ProjectID    int64   `json:"project_id"`
	Name         string  `json:"name"`
	TotalHours   float64 `json:"total_hours"`
	PlannedHours float64 `json:"planned_hours"`
	SpentHours   float64 `json:"spent_hours"`
}

type Input struct {
	ProjectID  int64    `json:"project_id"`
	Name       string   `json:"name"`
	TotalHours *float64 `json:"total_hours"`
}

type Patch struct {
	ProjectID  nullable.Optional[int64]   `json:"project_id"`
	Name       nullable.Optional[string]  `json:"name"`
	TotalHours nullable.Optional[float64] `json:"total_hours"`
}

func (in Input) Patch() Patch {
	patch := Patch{ProjectID: *nullable.Set(in.ProjectID), Name: *nullable.Set(in.Name)}
	if in.TotalHours != nil {
		patch.TotalHours = *nullable.Set(*in.TotalHours)
	}
	return patch
}

type SubprojectsResponse struct {
	Subprojects []Subproject `json:"subprojects"`
}

func FromDB(s db.Subproject) Subproject {
	return Subproject{
		ID:         s.ID,
		ProjectID:  s.ProjectID,
		Name:       s.Name,
		TotalHours: s.TotalHours,
	}
}

func fromTotals(id, projectID int64, name string, totalHours, plannedHours, spentHours float64) Subproject {
	return Subproject{
		ID: id, ProjectID: projectID, Name: name, TotalHours: totalHours,
		PlannedHours: plannedHours, SpentHours: spentHours,
	}
}

func validate(ctx context.Context, q *db.Queries, in Input, exceptID int64) (Input, error) {
	in.Name = strings.TrimSpace(in.Name)
	if in.Name == "" {
		return in, app.Invalid("name is required")
	}
	if in.ProjectID < 1 {
		return in, app.Invalid("project is required")
	}
	if in.TotalHours == nil {
		return in, app.Invalid("total_hours is required")
	}
	if !app.NonNegativeFinite(*in.TotalHours) {
		return in, app.Invalid("hours cannot be negative")
	}
	proj, err := q.GetProject(ctx, in.ProjectID)
	if errors.Is(err, sql.ErrNoRows) {
		return in, app.Missing("project not found")
	}
	if err != nil {
		return in, err
	}
	var used float64
	if exceptID > 0 {
		used, err = q.SumSubprojectHoursByProjectExcept(ctx, db.SumSubprojectHoursByProjectExceptParams{
			ProjectID: in.ProjectID,
			ID:        exceptID,
		})
	} else {
		used, err = q.SumSubprojectHoursByProject(ctx, in.ProjectID)
	}
	if err != nil {
		return in, err
	}
	if used+*in.TotalHours > proj.TotalHours {
		return in, app.Conflict("subproject hours exceed project hours")
	}
	return in, nil
}

func Create(ctx context.Context, q *db.Queries, in Input) (Subproject, error) {
	var result Subproject
	err := q.InTx(ctx, func(txq *db.Queries) error {
		validated, err := validate(ctx, txq, in, 0)
		if err != nil {
			return err
		}
		row, err := txq.CreateSubproject(ctx, db.CreateSubprojectParams{
			ProjectID: validated.ProjectID, Name: validated.Name, TotalHours: *validated.TotalHours,
		})
		if err == nil {
			result = FromDB(row)
		}
		return err
	})
	if err != nil {
		return Subproject{}, err
	}
	return Get(ctx, q, result.ID)
}

func Get(ctx context.Context, q *db.Queries, id int64) (Subproject, error) {
	row, err := q.GetSubproject(ctx, id)
	if errors.Is(err, sql.ErrNoRows) {
		return Subproject{}, app.Missing("subproject not found")
	}
	if err != nil {
		return Subproject{}, err
	}
	result := FromDB(row)
	if err := populateTotals(ctx, q, &result); err != nil {
		return Subproject{}, err
	}
	return result, nil
}

func populateTotals(ctx context.Context, q *db.Queries, s *Subproject) error {
	totals, err := q.GetSubprojectTotals(ctx, s.ID)
	if err != nil {
		return err
	}
	s.PlannedHours = totals.PlannedHours
	s.SpentHours = totals.SpentHours
	return nil
}

func ListWithTotals(ctx context.Context, q *db.Queries) ([]Subproject, error) {
	rows, err := q.ListSubprojectsWithTotals(ctx)
	if err != nil {
		return nil, err
	}
	out := make([]Subproject, 0, len(rows))
	for _, row := range rows {
		out = append(out, fromTotals(row.ID, row.ProjectID, row.Name, row.TotalHours, row.PlannedHours, row.SpentHours))
	}
	return out, nil
}

func List(ctx context.Context, q *db.Queries) ([]Subproject, error) {
	rows, err := q.ListSubprojects(ctx)
	if err != nil {
		return nil, err
	}
	out := make([]Subproject, 0, len(rows))
	for _, row := range rows {
		out = append(out, FromDB(row))
	}
	return out, nil
}

func ListByProject(ctx context.Context, q *db.Queries, projectID int64) ([]Subproject, error) {
	rows, err := q.ListSubprojectsByProject(ctx, projectID)
	if err != nil {
		return nil, err
	}
	out := make([]Subproject, 0, len(rows))
	for _, row := range rows {
		out = append(out, FromDB(row))
	}
	return out, nil
}

func ListByProjectWithTotals(ctx context.Context, q *db.Queries, projectID int64) ([]Subproject, error) {
	rows, err := q.ListSubprojectsByProjectWithTotals(ctx, projectID)
	if err != nil {
		return nil, err
	}
	out := make([]Subproject, 0, len(rows))
	for _, row := range rows {
		out = append(out, fromTotals(row.ID, row.ProjectID, row.Name, row.TotalHours, row.PlannedHours, row.SpentHours))
	}
	return out, nil
}

func Update(ctx context.Context, q *db.Queries, id int64, patch Patch) (Subproject, error) {
	err := q.InTx(ctx, func(txq *db.Queries) error {
		current, err := txq.GetSubproject(ctx, id)
		if errors.Is(err, sql.ErrNoRows) {
			return app.Missing("subproject not found")
		}
		if err != nil {
			return err
		}
		in := Input{
			ProjectID:  patch.ProjectID.Apply(current.ProjectID),
			Name:       patch.Name.Apply(current.Name),
			TotalHours: new(patch.TotalHours.Apply(current.TotalHours)),
		}
		validated, err := validate(ctx, txq, in, id)
		if err != nil {
			return err
		}
		if patch.ProjectID.Set && current.ProjectID != validated.ProjectID {
			taskCount, err := txq.CountTasksBySubproject(ctx, id)
			if err != nil {
				return err
			}
			if taskCount > 0 {
				return app.Conflict("cannot move subproject with tasks")
			}
		}
		_, err = txq.UpdateSubproject(ctx, db.UpdateSubprojectParams{
			ProjectID: validated.ProjectID, Name: validated.Name, TotalHours: *validated.TotalHours, ID: id,
		})
		return err
	})
	if err != nil {
		return Subproject{}, err
	}
	return Get(ctx, q, id)
}

func Delete(ctx context.Context, q *db.Queries, id int64) error {
	return q.InTx(ctx, func(txq *db.Queries) error {
		tasks, err := txq.CountTasksBySubproject(ctx, id)
		if err != nil {
			return err
		}
		if tasks > 0 {
			return app.Conflict("cannot delete a subproject with tasks")
		}
		_, err = txq.DeleteSubproject(ctx, id)
		if errors.Is(err, sql.ErrNoRows) {
			return app.Missing("subproject not found")
		}
		return err
	})
}
