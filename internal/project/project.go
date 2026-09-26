package project

import (
	"context"
	"database/sql"
	"errors"
	"net/http"
	"strings"
	"time"

	"cad-development/internal/apperr"
	"cad-development/internal/db"
	"cad-development/internal/validation"
	"cad-development/internal/weekly"
)

type Project struct {
	ID                int64   `json:"id"`
	Name              string  `json:"name"`
	PurchaseOrderName string  `json:"purchase_order_name"`
	TotalHours        float64 `json:"total_hours"`
	StartDate         string  `json:"start_date"`
	EndDate           string  `json:"end_date"`
}

type Input struct {
	Name              string  `json:"name"`
	PurchaseOrderName string  `json:"purchase_order_name"`
	TotalHours        float64 `json:"total_hours"`
	StartDate         string  `json:"start_date"`
	EndDate           string  `json:"end_date"`
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
		return in, apperr.New(http.StatusBadRequest, "name is required")
	}
	if !validation.NonNegativeFinite(in.TotalHours) {
		return in, apperr.New(http.StatusBadRequest, "hours cannot be negative")
	}
	start, err := weekly.ParseDate(in.StartDate)
	if err != nil {
		return in, apperr.New(http.StatusBadRequest, "invalid start_date")
	}
	end, err := weekly.ParseDate(in.EndDate)
	if err != nil {
		return in, apperr.New(http.StatusBadRequest, "invalid end_date")
	}
	if end.Before(start) {
		return in, apperr.New(http.StatusBadRequest, "end_date must be on or after start_date")
	}
	in.StartDate = start.Format("2006-01-02")
	in.EndDate = end.Format("2006-01-02")
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
		TotalHours:        in.TotalHours,
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
		return Project{}, apperr.New(http.StatusNotFound, "project not found")
	}
	if err != nil {
		return Project{}, err
	}
	return FromDB(row), nil
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

func Update(ctx context.Context, q *db.Queries, id int64, in Input) (Project, error) {
	in, err := validate(in)
	if err != nil {
		return Project{}, err
	}
	var result Project
	err = q.InTx(ctx, func(txq *db.Queries) error {
		if _, err := txq.GetProject(ctx, id); errors.Is(err, sql.ErrNoRows) {
			return apperr.New(http.StatusNotFound, "project not found")
		} else if err != nil {
			return err
		}
		sum, err := txq.SumSubprojectHoursByProject(ctx, id)
		if err != nil {
			return err
		}
		if in.TotalHours < sum {
			return apperr.New(http.StatusBadRequest, "project hours cannot be less than subproject hours")
		}
		start, _ := time.Parse(time.DateOnly, in.StartDate)
		end, _ := time.Parse(time.DateOnly, in.EndDate)
		outside, err := txq.CountTaskWeeksOutsideRange(ctx, db.CountTaskWeeksOutsideRangeParams{
			ProjectID: id,
			FirstWeek: weekly.MondayOnOrBefore(start).Format(time.DateOnly),
			LastWeek:  weekly.MondayOnOrBefore(end).Format(time.DateOnly),
		})
		if err != nil {
			return err
		}
		if outside > 0 {
			return apperr.New(http.StatusBadRequest, "project dates cannot exclude existing weekly data")
		}
		row, err := txq.UpdateProject(ctx, db.UpdateProjectParams{
			Name: in.Name, PurchaseOrderName: in.PurchaseOrderName, TotalHours: in.TotalHours,
			StartDate: in.StartDate, EndDate: in.EndDate, ID: id,
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
	return q.InTx(ctx, func(txq *db.Queries) error {
		if err := txq.ClearTaskWeeksByProject(ctx, id); err != nil {
			return err
		}
		return txq.DeleteProject(ctx, id)
	})
}
