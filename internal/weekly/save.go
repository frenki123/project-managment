package weekly

import (
	"context"
	"database/sql"
	"errors"
	"slices"
	"time"

	"cad-development/internal/app"
	"cad-development/internal/db"
	"cad-development/internal/monthlock"
	"cad-development/internal/nullable"
)

type Patch struct {
	PlannedHours *float64 `json:"planned_hours"`
	SpentHours   *float64 `json:"spent_hours"`
	Progress     *float64 `json:"progress"`
}

type Cell struct {
	TaskID       int64     `json:"task_id"`
	WeekStart    WeekStart `json:"week_start"`
	PlannedHours float64   `json:"planned_hours"`
	SpentHours   float64   `json:"spent_hours"`
	Progress     *float64  `json:"progress"`
}

func Save(ctx context.Context, q *db.Queries, taskID int64, weekStart WeekStart, patch Patch, now time.Time, unlocked monthlock.Set) (Cell, error) {
	if _, err := ParseWeekStart(weekStart); err != nil {
		return Cell{}, err
	}
	if patch.PlannedHours == nil && patch.SpentHours == nil && patch.Progress == nil {
		return Cell{}, app.Invalid("at least one value is required")
	}
	if patch.PlannedHours != nil {
		if err := validHours(*patch.PlannedHours); err != nil {
			return Cell{}, err
		}
	}
	if patch.SpentHours != nil {
		if err := validHours(*patch.SpentHours); err != nil {
			return Cell{}, err
		}
	}
	if patch.Progress != nil {
		if err := validProgress(*patch.Progress); err != nil {
			return Cell{}, err
		}
	}
	var result Cell
	err := q.InTx(ctx, func(txq *db.Queries) error {
		if unlocked == nil {
			var err error
			unlocked, err = monthlock.UnlockedSet(ctx, txq)
			if err != nil {
				return err
			}
		}
		if monthlock.WeekLocked(string(weekStart), now, unlocked) {
			return app.Locked("month is locked")
		}
		task, err := txq.GetTask(ctx, taskID)
		if errors.Is(err, sql.ErrNoRows) {
			return app.Missing("task not found")
		}
		if err != nil {
			return err
		}
		if !task.ProjectID.Valid {
			return app.Invalid("ideas cannot be planned")
		}
		project, err := txq.GetProject(ctx, task.ProjectID.Int64)
		if err != nil {
			return err
		}
		start, err := ParseDate(project.StartDate)
		if err != nil {
			return err
		}
		end, err := ParseDate(project.EndDate)
		if err != nil {
			return err
		}
		validWeeks := WeekStarts(start, end)
		inRange := slices.Contains(validWeeks, weekStart)
		if !inRange {
			return app.Invalid("week is outside the project date range")
		}

		existing, err := txq.GetTaskWeek(ctx, db.GetTaskWeekParams{TaskID: taskID, WeekStart: string(weekStart)})
		if errors.Is(err, sql.ErrNoRows) {
			existing = db.TaskWeek{TaskID: taskID, WeekStart: string(weekStart)}
		} else if err != nil {
			return err
		}
		planned, spent, progress := existing.PlannedHours, existing.SpentHours, existing.Progress
		if patch.PlannedHours != nil {
			planned = *patch.PlannedHours
		}
		if patch.SpentHours != nil {
			spent = *patch.SpentHours
		}
		if patch.Progress != nil {
			previous, err := lastProgressBefore(ctx, txq, taskID, weekStart)
			if err != nil {
				return err
			}
			if *patch.Progress < previous {
				return app.Invalid("progress cannot be less than the week before")
			}
			progress = nullable.Float64(patch.Progress)
		}
		var later []db.TaskWeek
		if patch.Progress != nil {
			later, err = txq.ListTaskWeeksAfter(ctx, db.ListTaskWeeksAfterParams{TaskID: taskID, WeekStart: string(weekStart)})
			if err != nil {
				return err
			}
			for _, laterWeek := range later {
				if laterWeek.Progress.Valid && laterWeek.Progress.Float64 < *patch.Progress && monthlock.WeekLocked(laterWeek.WeekStart, now, unlocked) {
					return app.Conflict("progress conflicts with a locked later week")
				}
			}
		}

		row, err := txq.UpsertTaskWeek(ctx, db.UpsertTaskWeekParams{
			TaskID: taskID, WeekStart: string(weekStart), PlannedHours: planned, SpentHours: spent, Progress: progress,
		})
		if err != nil {
			return err
		}
		if patch.Progress != nil {
			if err := txq.UpdateTaskWeeksProgressAfter(ctx, db.UpdateTaskWeeksProgressAfterParams{
				Progress:   nullable.Float64(patch.Progress),
				TaskID:     taskID,
				WeekStart:  string(weekStart),
				Progress_2: nullable.Float64(patch.Progress),
			}); err != nil {
				return err
			}
		}
		effective := nullable.Float64Pointer(row.Progress)
		if effective == nil {
			prev, err := lastProgressBefore(ctx, txq, taskID, weekStart)
			if err != nil {
				return err
			}
			effective = new(prev)
		}
		result = toCell(row, effective)
		return nil
	})
	return result, err
}

func lastProgressBefore(ctx context.Context, q *db.Queries, taskID int64, weekStart WeekStart) (float64, error) {
	progress, err := q.GetLastProgressBefore(ctx, db.GetLastProgressBeforeParams{TaskID: taskID, WeekStart: string(weekStart)})
	if errors.Is(err, sql.ErrNoRows) || !progress.Valid {
		return 0, nil
	}
	return progress.Float64, err
}

func validHours(value float64) error {
	if !app.NonNegativeFinite(value) {
		return app.Invalid("hours cannot be negative")
	}
	return nil
}

func validProgress(value float64) error {
	if !app.NonNegativeFinite(value) || value > 100 {
		return app.Invalid("progress must be between 0 and 100")
	}
	return nil
}

func toCell(week db.TaskWeek, progress *float64) Cell {
	return Cell{
		TaskID:       week.TaskID,
		WeekStart:    WeekStart(week.WeekStart),
		PlannedHours: week.PlannedHours,
		SpentHours:   week.SpentHours,
		Progress:     progress,
	}
}
