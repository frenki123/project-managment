package weekly

import (
	"context"
	"database/sql"
	"errors"
	"time"

	"cad-development/internal/db"
	"cad-development/internal/nullable"
	"cad-development/internal/web"
)

type Patch struct {
	PlannedHours nullable.Optional[float64] `json:"planned_hours,omitzero"`
	SpentHours   nullable.Optional[float64] `json:"spent_hours,omitzero"`
	Progress     nullable.Optional[float64] `json:"progress,omitzero"`
	Unlock       bool                       `json:"unlock,omitzero"`
}

type Cell struct {
	TaskID         int64     `json:"task_id"`
	WeekStart      WeekStart `json:"week_start"`
	PlannedHours   float64   `json:"planned_hours"`
	SpentHours     float64   `json:"spent_hours"`
	Progress       *float64  `json:"progress"`
	StoredProgress *float64  `json:"stored_progress"`
}

func Save(ctx context.Context, q *db.Queries, taskID int64, weekStart WeekStart, patch Patch, now time.Time) (Cell, error) {
	if weekStart.IsZero() {
		return Cell{}, web.Invalid("week_start must be a Monday")
	}
	if err := validatePatch(patch); err != nil {
		return Cell{}, err
	}
	var result Cell
	err := q.InTx(ctx, func(txq *db.Queries) error {
		if !patch.Unlock && weekStart.IsLocked(now) {
			return web.Locked("historical editing is not enabled")
		}
		contextRow, err := txq.WeekWriteContext(ctx, db.WeekWriteContextParams{
			TaskID: taskID, WeekStart: weekStart.String(),
		})
		if errors.Is(err, sql.ErrNoRows) {
			return web.Missing("task not found")
		}
		if err != nil {
			return err
		}
		if err := web.HTTPErrorFromReason(int(contextRow.Status), contextRow.Reason); err != nil {
			return err
		}

		planned := patch.PlannedHours.ValueOr(contextRow.PlannedHours)
		spent := patch.SpentHours.ValueOr(contextRow.SpentHours)
		stored, store := resolveProgress(patch, contextRow)
		_, err = txq.UpsertTaskWeek(ctx, db.UpsertTaskWeekParams{
			TaskID: taskID, WeekStart: weekStart.String(), PlannedHours: planned, SpentHours: spent, Progress: stored,
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
		result = savedCell(taskID, weekStart, planned, spent, stored, contextRow.PreviousProgress)
		return nil
	})
	return result, err
}

func savedCell(taskID int64, weekStart WeekStart, planned, spent float64, stored sql.NullFloat64, previous float64) Cell {
	effective := previous
	if stored.Valid {
		effective = max(effective, stored.Float64)
	}
	cell := Cell{
		TaskID: taskID, WeekStart: weekStart,
		PlannedHours: planned, SpentHours: spent,
		Progress: new(effective),
	}
	if stored.Valid {
		cell.StoredProgress = new(stored.Float64)
	}
	return cell
}

func validatePatch(patch Patch) error {
	if !patch.PlannedHours.HasValue() && !patch.SpentHours.HasValue() && !patch.Progress.Present {
		return web.Invalid("at least one value is required")
	}
	if patch.PlannedHours.HasValue() {
		if err := validHours(*patch.PlannedHours.Value); err != nil {
			return err
		}
	}
	if patch.SpentHours.HasValue() {
		if err := validHours(*patch.SpentHours.Value); err != nil {
			return err
		}
	}
	if patch.Progress.HasValue() {
		if err := validProgress(*patch.Progress.Value); err != nil {
			return err
		}
	}
	return nil
}

func resolveProgress(patch Patch, contextRow db.WeekWriteContextRow) (stored sql.NullFloat64, store bool) {
	carried := contextRow.PreviousProgress
	currentEffective := carried
	if p := nullable.Float64Pointer(contextRow.Progress); p != nil {
		currentEffective = max(carried, *p)
	}
	store = patch.Progress.HasValue() && *patch.Progress.Value > currentEffective
	stored = contextRow.Progress
	if store {
		stored = nullable.Float64(patch.Progress.Value)
	} else if patch.Progress.Present && patch.Progress.Value == nil {
		stored = sql.NullFloat64{}
	}
	return stored, store
}

func validHours(value float64) error {
	if !web.NonNegativeFinite(value) {
		return web.Invalid("hours cannot be negative")
	}
	return nil
}

func validProgress(value float64) error {
	if !web.NonNegativeFinite(value) || value > 100 {
		return web.Invalid("progress must be between 0 and 100")
	}
	return nil
}
