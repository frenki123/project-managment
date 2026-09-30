package project

import (
	"context"
	"database/sql"
	"errors"
	"strings"
	"time"

	"cad-development/internal/app"
	"cad-development/internal/db"
	"cad-development/internal/nullable"
	"cad-development/internal/weekly"
)

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

type Input struct {
	Name              string   `json:"name"`
	PurchaseOrderName string   `json:"purchase_order_name"`
	TotalHours        *float64 `json:"total_hours"`
	StartDate         string   `json:"start_date"`
	EndDate           string   `json:"end_date"`
}

type Patch struct {
	Name              nullable.Optional[string]  `json:"name"`
	PurchaseOrderName nullable.Optional[string]  `json:"purchase_order_name"`
	TotalHours        nullable.Optional[float64] `json:"total_hours"`
	StartDate         nullable.Optional[string]  `json:"start_date"`
	EndDate           nullable.Optional[string]  `json:"end_date"`
}

func (in Input) Patch() Patch {
	patch := Patch{
		Name: *nullable.Set(in.Name), PurchaseOrderName: *nullable.Set(in.PurchaseOrderName),
		StartDate: *nullable.Set(in.StartDate), EndDate: *nullable.Set(in.EndDate),
	}
	if in.TotalHours != nil {
		patch.TotalHours = *nullable.Set(*in.TotalHours)
	}
	return patch
}

type ProjectsResponse struct {
	Projects []Project `json:"projects"`
}

func FromDB(p db.Project) Project {
	return Project{
		ID:                p.ID,
		Name:              p.Name,
		PurchaseOrderName: p.PurchaseOrderName,
		TotalHours:        p.TotalHours,
		StartDate:         p.StartDate,
		EndDate:           p.EndDate,
	}
}

func fromTotals(row db.VProjectTotal) Project {
	return Project{
		ID: row.ID, Name: row.Name, PurchaseOrderName: row.PurchaseOrderName,
		TotalHours: row.TotalHours, StartDate: row.StartDate, EndDate: row.EndDate,
		PlannedHours: row.PlannedHours, SpentHours: row.SpentHours,
		ProgressPct: row.Progress, EarnedHours: row.Progress * row.TotalHours / 100,
	}
}

func validateName(name string) (string, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return name, app.Invalid("name is required")
	}
	return name, nil
}

func validateFields(in Input) (Input, error) {
	in.PurchaseOrderName = strings.TrimSpace(in.PurchaseOrderName)
	if in.TotalHours == nil {
		return in, app.Invalid("total_hours is required")
	}
	if !app.NonNegativeFinite(*in.TotalHours) {
		return in, app.Invalid("hours cannot be negative")
	}
	start, err := weekly.ParseDate(in.StartDate)
	if err != nil {
		return in, app.Invalid("invalid start_date")
	}
	end, err := weekly.ParseDate(in.EndDate)
	if err != nil {
		return in, app.Invalid("invalid end_date")
	}
	if end.Before(start) {
		return in, app.Invalid("end_date must be on or after start_date")
	}
	in.StartDate = start.Format(time.DateOnly)
	in.EndDate = end.Format(time.DateOnly)
	return in, nil
}

func Create(ctx context.Context, q *db.Queries, in Input) (Project, error) {
	name, err := validateName(in.Name)
	if err != nil {
		return Project{}, err
	}
	in.Name = name
	var id int64
	err = q.InTx(ctx, func(txq *db.Queries) error {
		taken, err := txq.ProjectNameConflict(ctx, db.ProjectNameConflictParams{Name: in.Name, ExceptID: 0})
		if err != nil {
			return err
		}
		if taken {
			return app.Conflict("project name already exists")
		}
		validated, err := validateFields(in)
		if err != nil {
			return err
		}
		row, err := txq.CreateProject(ctx, db.CreateProjectParams{
			Name:              validated.Name,
			PurchaseOrderName: validated.PurchaseOrderName,
			TotalHours:        *validated.TotalHours,
			StartDate:         validated.StartDate,
			EndDate:           validated.EndDate,
		})
		if err == nil {
			id = row.ID
		}
		return err
	})
	if err != nil {
		if db.UniqueViolation(err, "projects.name") {
			return Project{}, app.Conflict("project name already exists")
		}
		return Project{}, err
	}
	return Get(ctx, q, id)
}

func Get(ctx context.Context, q *db.Queries, id int64) (Project, error) {
	row, err := q.GetProject(ctx, id)
	if errors.Is(err, sql.ErrNoRows) {
		return Project{}, app.Missing("project not found")
	}
	if err != nil {
		return Project{}, err
	}
	result := FromDB(row)
	if err := populateTotals(ctx, q, &result); err != nil {
		return Project{}, err
	}
	return result, nil
}

func populateTotals(ctx context.Context, q *db.Queries, p *Project) error {
	totals, err := q.GetProjectTotals(ctx, p.ID)
	if err != nil {
		return err
	}
	p.PlannedHours = totals.PlannedHours
	p.SpentHours = totals.SpentHours
	p.ProgressPct = totals.Progress
	p.EarnedHours = totals.Progress * p.TotalHours / 100
	return nil
}

func List(ctx context.Context, q *db.Queries) ([]Project, error) {
	rows, err := q.ListProjects(ctx)
	if err != nil {
		return nil, err
	}
	out := make([]Project, 0, len(rows))
	for _, row := range rows {
		out = append(out, FromDB(row))
	}
	return out, nil
}

func ListWithTotals(ctx context.Context, q *db.Queries) ([]Project, error) {
	rows, err := q.ListProjectsWithTotals(ctx)
	if err != nil {
		return nil, err
	}
	out := make([]Project, 0, len(rows))
	for _, row := range rows {
		out = append(out, fromTotals(row))
	}
	return out, nil
}

func Update(ctx context.Context, q *db.Queries, id int64, patch Patch) (Project, error) {
	err := q.InTx(ctx, func(txq *db.Queries) error {
		current, err := txq.GetProject(ctx, id)
		if errors.Is(err, sql.ErrNoRows) {
			return app.Missing("project not found")
		}
		if err != nil {
			return err
		}
		currentHours := current.TotalHours
		in := Input{
			Name:              patch.Name.Apply(current.Name),
			PurchaseOrderName: patch.PurchaseOrderName.Apply(current.PurchaseOrderName),
			TotalHours:        new(patch.TotalHours.Apply(currentHours)),
			StartDate:         patch.StartDate.Apply(current.StartDate),
			EndDate:           patch.EndDate.Apply(current.EndDate),
		}
		in.Name, err = validateName(in.Name)
		if err != nil {
			return err
		}
		taken, err := txq.ProjectNameConflict(ctx, db.ProjectNameConflictParams{Name: in.Name, ExceptID: id})
		if err != nil {
			return err
		}
		if taken {
			return app.Conflict("project name already exists")
		}
		validated, err := validateFields(in)
		if err != nil {
			return err
		}
		conflict, err := txq.ProjectUpdateConflict(ctx, db.ProjectUpdateConflictParams{
			ProjectID:  id,
			StartDate:  validated.StartDate,
			EndDate:    validated.EndDate,
			TotalHours: *validated.TotalHours,
		})
		if err != nil {
			return err
		}
		if err := app.FromStatus(int(conflict.Status), conflict.Reason); err != nil {
			return err
		}
		_, err = txq.UpdateProject(ctx, db.UpdateProjectParams{
			Name: validated.Name, PurchaseOrderName: validated.PurchaseOrderName, TotalHours: *validated.TotalHours,
			StartDate: validated.StartDate, EndDate: validated.EndDate, ID: id,
		})
		if errors.Is(err, sql.ErrNoRows) {
			return app.Missing("project not found")
		}
		if db.UniqueViolation(err, "projects.name") {
			return app.Conflict("project name already exists")
		}
		return err
	})
	if err != nil {
		return Project{}, err
	}
	return Get(ctx, q, id)
}

func Delete(ctx context.Context, q *db.Queries, id int64) error {
	err := q.InTx(ctx, func(txq *db.Queries) error {
		_, err := txq.DeleteProject(ctx, id)
		if errors.Is(err, sql.ErrNoRows) {
			return app.Missing("project not found")
		}
		return err
	})
	return app.ReferencedConflict(err)
}
