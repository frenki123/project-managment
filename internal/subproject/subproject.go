package subproject

import (
	"context"
	"database/sql"
	"errors"
	"strings"

	"cad-development/internal/db"
	"cad-development/internal/validation"
)

type Subproject struct {
	ID         int64   `json:"id"`
	ProjectID  int64   `json:"project_id"`
	Name       string  `json:"name"`
	TotalHours float64 `json:"total_hours"`
}

type Input struct {
	ProjectID  int64   `json:"project_id"`
	Name       string  `json:"name"`
	TotalHours float64 `json:"total_hours"`
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

func validate(ctx context.Context, q *db.Queries, in Input, exceptID int64) (Input, error) {
	in.Name = strings.TrimSpace(in.Name)
	if in.Name == "" {
		return in, Invalid("name is required")
	}
	if in.ProjectID < 1 {
		return in, Invalid("project is required")
	}
	if !validation.NonNegativeFinite(in.TotalHours) {
		return in, Invalid("hours cannot be negative")
	}
	proj, err := q.GetProject(ctx, in.ProjectID)
	if errors.Is(err, sql.ErrNoRows) {
		return in, Missing("project not found")
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
	if used+in.TotalHours > proj.TotalHours {
		return in, ConflictError("subproject hours exceed project hours")
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
			ProjectID: validated.ProjectID, Name: validated.Name, TotalHours: validated.TotalHours,
		})
		if err == nil {
			result = FromDB(row)
		}
		return err
	})
	return result, err
}

func Get(ctx context.Context, q *db.Queries, id int64) (Subproject, error) {
	row, err := q.GetSubproject(ctx, id)
	if errors.Is(err, sql.ErrNoRows) {
		return Subproject{}, Missing("subproject not found")
	}
	if err != nil {
		return Subproject{}, err
	}
	return FromDB(row), nil
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

func Update(ctx context.Context, q *db.Queries, id int64, in Input) (Subproject, error) {
	if _, err := Get(ctx, q, id); err != nil {
		return Subproject{}, err
	}
	var result Subproject
	err := q.InTx(ctx, func(txq *db.Queries) error {
		validated, err := validate(ctx, txq, in, id)
		if err != nil {
			return err
		}
		row, err := txq.UpdateSubproject(ctx, db.UpdateSubprojectParams{
			ProjectID: validated.ProjectID, Name: validated.Name, TotalHours: validated.TotalHours, ID: id,
		})
		if err == nil {
			result = FromDB(row)
		}
		return err
	})
	return result, err
}

func Delete(ctx context.Context, q *db.Queries, id int64) error {
	if _, err := Get(ctx, q, id); err != nil {
		return err
	}
	return q.DeleteSubproject(ctx, id)
}
