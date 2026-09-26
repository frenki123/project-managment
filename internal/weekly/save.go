package weekly

import (
	"context"
	"database/sql"
	"errors"
	"time"

	"cad-development/internal/db"
	"cad-development/internal/monthlock"
	"cad-development/internal/validation"
)

type Patch struct {
	PlannedHours *float64 `json:"planned_hours"`
	SpentHours   *float64 `json:"spent_hours"`
	Progress     *float64 `json:"progress"`
}

type Cell struct {
	TaskID       int64    `json:"task_id"`
	WeekStart    string   `json:"week_start"`
	PlannedHours float64  `json:"planned_hours"`
	SpentHours   float64  `json:"spent_hours"`
	Progress     *float64 `json:"progress"`
}

func Save(ctx context.Context, q *db.Queries, taskID int64, weekStart string, patch Patch, now time.Time, unlocked map[string]bool) (Cell, error) {
	if _, err := ParseMonday(weekStart); err != nil {
		return Cell{}, err
	}
	if patch.PlannedHours == nil && patch.SpentHours == nil && patch.Progress == nil {
		return Cell{}, Invalid("at least one value is required")
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
	if monthlock.WeekLocked(weekStart, now, unlocked) {
		return Cell{}, Locked("month is locked")
	}

	var result Cell
	err := q.InTx(ctx, func(txq *db.Queries) error {
		task, err := txq.GetTask(ctx, taskID)
		if errors.Is(err, sql.ErrNoRows) {
			return Missing("task not found")
		}
		if err != nil {
			return err
		}
		if !task.ProjectID.Valid {
			return Invalid("ideas cannot be planned")
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
		inRange := false
		for _, validWeek := range validWeeks {
			if validWeek == weekStart {
				inRange = true
				break
			}
		}
		if !inRange {
			return Invalid("week is outside the project date range")
		}

		existing, err := txq.GetTaskWeek(ctx, db.GetTaskWeekParams{TaskID: taskID, WeekStart: weekStart})
		if errors.Is(err, sql.ErrNoRows) {
			existing = db.TaskWeek{TaskID: taskID, WeekStart: weekStart}
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
				return Invalid("progress cannot be less than the week before")
			}
			progress = sql.NullFloat64{Float64: *patch.Progress, Valid: true}
		}
		var later []db.TaskWeek
		if patch.Progress != nil {
			later, err = txq.ListTaskWeeksAfter(ctx, db.ListTaskWeeksAfterParams{TaskID: taskID, WeekStart: weekStart})
			if err != nil {
				return err
			}
			for _, laterWeek := range later {
				if laterWeek.Progress.Valid && laterWeek.Progress.Float64 < *patch.Progress && monthlock.WeekLocked(laterWeek.WeekStart, now, unlocked) {
					return ConflictError("progress conflicts with a locked later week")
				}
			}
		}

		row, err := txq.UpsertTaskWeek(ctx, db.UpsertTaskWeekParams{
			TaskID: taskID, WeekStart: weekStart, PlannedHours: planned, SpentHours: spent, Progress: progress,
		})
		if err != nil {
			return err
		}
		if patch.Progress != nil {
			for _, laterWeek := range later {
				if laterWeek.Progress.Valid && laterWeek.Progress.Float64 < *patch.Progress {
					if err := txq.UpdateTaskWeekProgress(ctx, db.UpdateTaskWeekProgressParams{
						Progress: sql.NullFloat64{Float64: *patch.Progress, Valid: true}, TaskID: taskID, WeekStart: laterWeek.WeekStart,
					}); err != nil {
						return err
					}
				}
			}
		}
		result = toCell(row)
		return nil
	})
	return result, err
}

func lastProgressBefore(ctx context.Context, q *db.Queries, taskID int64, weekStart string) (float64, error) {
	progress, err := q.GetLastProgressBefore(ctx, db.GetLastProgressBeforeParams{TaskID: taskID, WeekStart: weekStart})
	if errors.Is(err, sql.ErrNoRows) || !progress.Valid {
		return 0, nil
	}
	return progress.Float64, err
}

func validHours(value float64) error {
	if !validation.NonNegativeFinite(value) {
		return Invalid("hours cannot be negative")
	}
	return nil
}

func validProgress(value float64) error {
	if !validation.NonNegativeFinite(value) || value > 100 {
		return Invalid("progress must be between 0 and 100")
	}
	return nil
}

func toCell(week db.TaskWeek) Cell {
	cell := Cell{TaskID: week.TaskID, WeekStart: week.WeekStart, PlannedHours: week.PlannedHours, SpentHours: week.SpentHours}
	if week.Progress.Valid {
		cell.Progress = new(week.Progress.Float64)
	}
	return cell
}

func Totals(weeks []db.TaskWeek) (planned, spent, progress float64) {
	for _, week := range weeks {
		planned += week.PlannedHours
		spent += week.SpentHours
		if week.Progress.Valid {
			progress = week.Progress.Float64
		}
	}
	return planned, spent, progress
}
