package subproject

import (
	"context"
	"database/sql"
	"errors"
	"strings"

	"cad-development/internal/db"
	"cad-development/internal/nullable"
	"cad-development/internal/web"
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
	ProjectID  nullable.Optional[int64]   `json:"project_id,omitzero"`
	Name       nullable.Optional[string]  `json:"name,omitzero"`
	TotalHours nullable.Optional[float64] `json:"total_hours,omitzero"`
}

func PatchFromInput(in Input) Patch {
	patch := Patch{ProjectID: nullable.Present(in.ProjectID), Name: nullable.Present(in.Name)}
	if in.TotalHours != nil {
		patch.TotalHours = nullable.Present(*in.TotalHours)
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

func fromTotalsRow(row db.ListSubprojectsWithTotalsRow) Subproject {
	return Subproject{
		ID: row.ID, ProjectID: row.ProjectID, Name: row.Name, TotalHours: row.TotalHours,
		PlannedHours: row.PlannedHours, SpentHours: row.SpentHours,
	}
}

func validate(in Input) (Input, error) {
	in.Name = strings.TrimSpace(in.Name)
	if in.Name == "" {
		return in, web.Invalid("name is required")
	}
	if in.ProjectID < 1 {
		return in, web.Invalid("project is required")
	}
	if in.TotalHours == nil {
		return in, web.Invalid("total hours is required")
	}
	if !web.NonNegativeFinite(*in.TotalHours) {
		return in, web.Invalid("hours cannot be negative")
	}
	return in, nil
}

func Create(ctx context.Context, q *db.Queries, in Input) (Subproject, error) {
	var result Subproject
	err := q.InTx(ctx, func(txq *db.Queries) error {
		validated, err := validate(in)
		if err != nil {
			return err
		}
		conflict, err := txq.SubprojectHoursConflict(ctx, db.SubprojectHoursConflictParams{
			ProjectID: in.ProjectID, ExceptID: 0, NewHours: *in.TotalHours,
		})
		if err != nil {
			return err
		}
		if err := web.HTTPErrorFromReason(int(conflict.Status), conflict.Reason); err != nil {
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
		return Subproject{}, web.Missing("subproject not found")
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
	rows, err := q.ListSubprojectsWithTotals(ctx, sql.NullInt64{})
	if err != nil {
		return nil, err
	}
	out := make([]Subproject, 0, len(rows))
	for _, row := range rows {
		out = append(out, fromTotalsRow(row))
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
	rows, err := q.ListSubprojectsWithTotals(ctx, nullable.Int64(&projectID))
	if err != nil {
		return nil, err
	}
	out := make([]Subproject, 0, len(rows))
	for _, row := range rows {
		out = append(out, fromTotalsRow(row))
	}
	return out, nil
}

func Update(ctx context.Context, q *db.Queries, id int64, patch Patch) (Subproject, error) {
	err := q.InTx(ctx, func(txq *db.Queries) error {
		current, err := txq.GetSubproject(ctx, id)
		if errors.Is(err, sql.ErrNoRows) {
			return web.Missing("subproject not found")
		}
		if err != nil {
			return err
		}
		in := Input{
			ProjectID:  patch.ProjectID.Apply(current.ProjectID),
			Name:       patch.Name.Apply(current.Name),
			TotalHours: new(patch.TotalHours.Apply(current.TotalHours)),
		}
		validated, err := validate(in)
		if err != nil {
			return err
		}
		conflict, err := txq.SubprojectHoursConflict(ctx, db.SubprojectHoursConflictParams{
			ProjectID: validated.ProjectID, ExceptID: id, NewHours: *validated.TotalHours,
		})
		if err != nil {
			return err
		}
		if err := web.HTTPErrorFromReason(int(conflict.Status), conflict.Reason); err != nil {
			return err
		}
		_, err = txq.UpdateSubproject(ctx, db.UpdateSubprojectParams{ //nolint:droppedvalue -- the returned row cannot replace the post-update Get; only the error is needed
			ProjectID: validated.ProjectID, Name: validated.Name, TotalHours: *validated.TotalHours, ID: id,
		})
		if errors.Is(err, sql.ErrNoRows) {
			return web.Missing("subproject not found")
		}
		return err
	})
	if err != nil {
		return Subproject{}, web.ReferencedConflict(err)
	}
	return Get(ctx, q, id)
}

func Delete(ctx context.Context, q *db.Queries, id int64) error {
	err := q.InTx(ctx, func(txq *db.Queries) error {
		rows, err := txq.DeleteSubproject(ctx, id)
		if err != nil {
			return err
		}
		if rows == 0 {
			return web.Missing("subproject not found")
		}
		return nil
	})
	return web.ReferencedConflict(err)
}
