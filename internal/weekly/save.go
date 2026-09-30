package weekly

import (
	"context"
	"database/sql"
	"errors"
	"time"

	"cad-development/internal/app"
	"cad-development/internal/db"
	"cad-development/internal/nullable"
)

type Patch struct {
	PlannedHours  *float64 `json:"planned_hours"`
	SpentHours    *float64 `json:"spent_hours"`
	Progress      *float64 `json:"progress"`
	ClearProgress bool     `json:"clear_progress,omitempty"`
	Unlock        bool     `json:"unlock,omitempty"`
}

type Cell struct {
	TaskID       int64     `json:"task_id"`
	WeekStart    WeekStart `json:"week_start"`
	PlannedHours float64   `json:"planned_hours"`
	SpentHours   float64   `json:"spent_hours"`
	Progress     *float64  `json:"progress"`
}

func Save(ctx context.Context, q *db.Queries, taskID int64, weekStart WeekStart, patch Patch, now time.Time) (Cell, error) {
	if _, err := ParseWeekStart(weekStart); err != nil {
		return Cell{}, err
	}
	if patch.PlannedHours == nil && patch.SpentHours == nil && patch.Progress == nil && !patch.ClearProgress {
		return Cell{}, app.Invalid("at least one value is required")
	}
	if patch.Progress != nil && patch.ClearProgress {
		return Cell{}, app.Invalid("progress and clear_progress cannot both be set")
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
		if !patch.Unlock && IsWeekLocked(string(weekStart), now) {
			return app.Locked("historical editing is not enabled")
		}
		contextRow, err := txq.WeekWriteContext(ctx, db.WeekWriteContextParams{
			TaskID: taskID, WeekStart: string(weekStart), Progress: nullable.Float64(patch.Progress),
		})
		if errors.Is(err, sql.ErrNoRows) {
			return app.Missing("task not found")
		}
		if err != nil {
			return err
		}
		if err := app.FromStatus(int(contextRow.Status), contextRow.Reason); err != nil {
			return err
		}

		planned, spent, progress := contextRow.PlannedHours, contextRow.SpentHours, contextRow.Progress
		if patch.PlannedHours != nil {
			planned = *patch.PlannedHours
		}
		if patch.SpentHours != nil {
			spent = *patch.SpentHours
		}
		if patch.Progress != nil {
			progress = nullable.Float64(patch.Progress)
		} else if patch.ClearProgress {
			progress = sql.NullFloat64{}
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
			effective = new(contextRow.PreviousProgress)
		}
		result = toCell(row, effective)
		return nil
	})
	return result, err
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
