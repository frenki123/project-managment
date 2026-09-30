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
	PlannedHours nullable.Optional[float64] `json:"planned_hours,omitzero"`
	SpentHours   nullable.Optional[float64] `json:"spent_hours,omitzero"`
	Progress     nullable.Optional[float64] `json:"progress,omitzero"`
	Unlock       bool                       `json:"unlock,omitzero"`
}

type Cell struct {
	TaskID       int64     `json:"task_id"`
	WeekStart    WeekStart `json:"week_start"`
	PlannedHours float64   `json:"planned_hours"`
	SpentHours   float64   `json:"spent_hours"`
	Progress     *float64  `json:"progress"`
}

func Save(ctx context.Context, q *db.Queries, taskID int64, weekStart WeekStart, patch Patch, now time.Time) (Cell, error) {
	if _, err := Parse(weekStart.String()); err != nil {
		return Cell{}, err
	}
	if (!patch.PlannedHours.Present || patch.PlannedHours.Value == nil) &&
		(!patch.SpentHours.Present || patch.SpentHours.Value == nil) &&
		!patch.Progress.Present {
		return Cell{}, app.Invalid("at least one value is required")
	}
	if patch.PlannedHours.Present && patch.PlannedHours.Value != nil {
		if err := validHours(*patch.PlannedHours.Value); err != nil {
			return Cell{}, err
		}
	}
	if patch.SpentHours.Present && patch.SpentHours.Value != nil {
		if err := validHours(*patch.SpentHours.Value); err != nil {
			return Cell{}, err
		}
	}
	if patch.Progress.Present && patch.Progress.Value != nil {
		if err := validProgress(*patch.Progress.Value); err != nil {
			return Cell{}, err
		}
	}
	var result Cell
	err := q.InTx(ctx, func(txq *db.Queries) error {
		if !patch.Unlock && weekStart.IsLocked(now) {
			return app.Locked("historical editing is not enabled")
		}
		contextRow, err := txq.WeekWriteContext(ctx, db.WeekWriteContextParams{
			TaskID: taskID, WeekStart: weekStart.String(),
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

		planned, spent := contextRow.PlannedHours, contextRow.SpentHours
		if patch.PlannedHours.Present && patch.PlannedHours.Value != nil {
			planned = *patch.PlannedHours.Value
		}
		if patch.SpentHours.Present && patch.SpentHours.Value != nil {
			spent = *patch.SpentHours.Value
		}
		carried := contextRow.PreviousProgress
		store := patch.Progress.Present && patch.Progress.Value != nil && *patch.Progress.Value > carried
		progress := contextRow.Progress
		switch {
		case store:
			progress = nullable.Float64(patch.Progress.Value)
		case patch.Progress.Present:
			progress = sql.NullFloat64{}
		}
		row, err := txq.UpsertTaskWeek(ctx, db.UpsertTaskWeekParams{
			TaskID: taskID, WeekStart: weekStart.String(), PlannedHours: planned, SpentHours: spent, Progress: progress,
		})
		if err != nil {
			return err
		}
		if store {
			if err := txq.UpdateTaskWeeksProgressAfter(ctx, db.UpdateTaskWeeksProgressAfterParams{
				TaskID:    taskID,
				WeekStart: weekStart.String(),
				Progress:  nullable.Float64(patch.Progress.Value),
			}); err != nil {
				return err
			}
		}
		effective := nullable.Float64Pointer(row.Progress)
		if effective == nil {
			effective = new(carried)
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
