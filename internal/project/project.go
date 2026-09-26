package project

import (
	"context"
	"database/sql"
	"errors"
	"strings"
	"time"

	"cad-development/internal/app"
	"cad-development/internal/db"
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

func validate(in Input) (Input, error) {
	in.Name = strings.TrimSpace(in.Name)
	in.PurchaseOrderName = strings.TrimSpace(in.PurchaseOrderName)
	if in.Name == "" {
		return in, app.Invalid("name is required")
	}
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
	in, err := validate(in)
	if err != nil {
		return Project{}, err
	}
	row, err := q.CreateProject(ctx, db.CreateProjectParams{
		Name:              in.Name,
		PurchaseOrderName: in.PurchaseOrderName,
		TotalHours:        *in.TotalHours,
		StartDate:         in.StartDate,
		EndDate:           in.EndDate,
	})
	if err != nil {
		return Project{}, err
	}
	return FromDB(row), nil
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
		out = append(out, Project{
			ID: row.ID, Name: row.Name, PurchaseOrderName: row.PurchaseOrderName,
			TotalHours: row.TotalHours, StartDate: row.StartDate, EndDate: row.EndDate,
			PlannedHours: row.PlannedHours, SpentHours: row.SpentHours,
			ProgressPct: row.Progress, EarnedHours: row.Progress * row.TotalHours / 100,
		})
	}
	return out, nil
}

func Update(ctx context.Context, q *db.Queries, id int64, in Input) (Project, error) {
	in, err := validate(in)
	if err != nil {
		return Project{}, err
	}
	err = q.InTx(ctx, func(txq *db.Queries) error {
		if _, err := txq.GetProject(ctx, id); errors.Is(err, sql.ErrNoRows) {
			return app.Missing("project not found")
		} else if err != nil {
			return err
		}
		sum, err := txq.SumSubprojectHoursByProject(ctx, id)
		if err != nil {
			return err
		}
		if *in.TotalHours < sum {
			return app.Conflict("project hours cannot be less than subproject hours")
		}
		start, err := weekly.ParseDate(in.StartDate)
		if err != nil {
			return err
		}
		end, err := weekly.ParseDate(in.EndDate)
		if err != nil {
			return err
		}
		outside, err := txq.CountTaskWeeksOutsideRange(ctx, db.CountTaskWeeksOutsideRangeParams{
			ProjectID: id,
			FirstWeek: weekly.MondayOnOrBefore(start).Format(time.DateOnly),
			LastWeek:  weekly.MondayOnOrBefore(end).Format(time.DateOnly),
		})
		if err != nil {
			return err
		}
		if outside > 0 {
			return app.Conflict("project dates cannot exclude existing weekly data")
		}
		_, err = txq.UpdateProject(ctx, db.UpdateProjectParams{
			Name: in.Name, PurchaseOrderName: in.PurchaseOrderName, TotalHours: *in.TotalHours,
			StartDate: in.StartDate, EndDate: in.EndDate, ID: id,
		})
		return err
	})
	if err != nil {
		return Project{}, err
	}
	return Get(ctx, q, id)
}

func Delete(ctx context.Context, q *db.Queries, id int64) error {
	return q.InTx(ctx, func(txq *db.Queries) error {
		if _, err := txq.GetProject(ctx, id); errors.Is(err, sql.ErrNoRows) {
			return app.Missing("project not found")
		} else if err != nil {
			return err
		}
		tasks, err := txq.CountTasksByProject(ctx, id)
		if err != nil {
			return err
		}
		if tasks > 0 {
			return app.Conflict("cannot delete a project with tasks")
		}
		subprojects, err := txq.CountSubprojectsByProject(ctx, id)
		if err != nil {
			return err
		}
		if subprojects > 0 {
			return app.Conflict("cannot delete a project with subprojects")
		}
		_, err = txq.DeleteProject(ctx, id)
		return err
	})
}
