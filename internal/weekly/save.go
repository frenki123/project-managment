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
	TaskID         int64                 `json:"task_id"`
	WeekStart      WeekStart             `json:"week_start"`
	PlannedHours   float64               `json:"planned_hours"`
	SpentHours     float64               `json:"spent_hours"`
	Progress       *float64              `json:"progress"`
	StoredProgress *float64              `json:"stored_progress"`
	Attributions   []DeveloperAllocation `json:"attributions"`
}

type DeveloperAllocation struct {
	PersonID     int64   `json:"person_id"`
	Name         string  `json:"name"`
	PlannedHours float64 `json:"planned_hours"`
	SpentHours   float64 `json:"spent_hours"`
}

type AttributionPatch struct {
	PlannedHours float64 `json:"planned_hours"`
	SpentHours   float64 `json:"spent_hours"`
	Unlock       bool    `json:"unlock,omitzero"`
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
		contextRow, err := writeContext(ctx, txq, taskID, weekStart, patch.Unlock, now)
		if err != nil {
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
		if contextRow.RowExists == 0 {
			if err := autoAttribute(ctx, txq, taskID, weekStart, planned, spent); err != nil {
				return err
			}
		}
		result = savedCell(taskID, weekStart, planned, spent, stored, contextRow.PreviousProgress)
		result.Attributions, err = loadAllocations(ctx, txq, taskID, weekStart.String())
		if err != nil {
			return err
		}
		return nil
	})
	return result, err
}

func autoAttribute(ctx context.Context, txq *db.Queries, taskID int64, weekStart WeekStart, planned, spent float64) error {
	if planned == 0 && spent == 0 {
		return nil
	}
	personID, err := txq.FirstTaskDeveloper(ctx, taskID)
	if errors.Is(err, sql.ErrNoRows) {
		return nil
	}
	if err != nil {
		return err
	}
	return txq.UpsertTaskWeekDeveloper(ctx, db.UpsertTaskWeekDeveloperParams{
		TaskID: taskID, WeekStart: weekStart.String(), PersonID: personID,
		PlannedHours: planned, SpentHours: spent,
	})
}

func loadAllocations(ctx context.Context, txq *db.Queries, taskID int64, weekStart string) ([]DeveloperAllocation, error) {
	rows, err := txq.ListTaskWeekDevelopers(ctx, db.ListTaskWeekDevelopersParams{TaskID: taskID, WeekStart: weekStart})
	if err != nil {
		return nil, err
	}
	out := make([]DeveloperAllocation, 0, len(rows))
	for _, row := range rows {
		out = append(out, DeveloperAllocation{PersonID: row.PersonID, Name: row.PersonName, PlannedHours: row.PlannedHours, SpentHours: row.SpentHours})
	}
	return out, nil
}

func SaveAttribution(ctx context.Context, q *db.Queries, taskID int64, weekStart WeekStart, personID int64, patch AttributionPatch, now time.Time) (DeveloperAllocation, error) {
	if weekStart.IsZero() {
		return DeveloperAllocation{}, web.Invalid("week_start must be a Monday")
	}
	if err := validHours(patch.PlannedHours); err != nil {
		return DeveloperAllocation{}, err
	}
	if err := validHours(patch.SpentHours); err != nil {
		return DeveloperAllocation{}, err
	}
	var result DeveloperAllocation
	err := q.InTx(ctx, func(txq *db.Queries) error {
		contextRow, err := writeContext(ctx, txq, taskID, weekStart, patch.Unlock, now)
		if err != nil {
			return err
		}
		if err := requirePersonOnTask(ctx, txq, taskID, personID); err != nil {
			return err
		}
		if _, err := txq.UpsertTaskWeek(ctx, db.UpsertTaskWeekParams{
			TaskID: taskID, WeekStart: weekStart.String(),
			PlannedHours: contextRow.PlannedHours, SpentHours: contextRow.SpentHours, Progress: contextRow.Progress,
		}); err != nil {
			return err
		}
		if err := txq.UpsertTaskWeekDeveloper(ctx, db.UpsertTaskWeekDeveloperParams{
			TaskID: taskID, WeekStart: weekStart.String(), PersonID: personID,
			PlannedHours: patch.PlannedHours, SpentHours: patch.SpentHours,
		}); err != nil {
			return err
		}
		allocations, err := loadAllocations(ctx, txq, taskID, weekStart.String())
		if err != nil {
			return err
		}
		result = allocationForPerson(allocations, personID)
		return nil
	})
	return result, err
}

func allocationForPerson(allocations []DeveloperAllocation, personID int64) DeveloperAllocation {
	for _, allocation := range allocations {
		if allocation.PersonID == personID {
			return allocation
		}
	}
	return DeveloperAllocation{}
}

func ClearAttribution(ctx context.Context, q *db.Queries, taskID int64, weekStart WeekStart, personID int64, unlock bool, now time.Time) error {
	if weekStart.IsZero() {
		return web.Invalid("week_start must be a Monday")
	}
	return q.InTx(ctx, func(txq *db.Queries) error {
		if _, err := writeContext(ctx, txq, taskID, weekStart, unlock, now); err != nil {
			return err
		}
		rows, err := txq.DeleteTaskWeekDeveloper(ctx, db.DeleteTaskWeekDeveloperParams{TaskID: taskID, WeekStart: weekStart.String(), PersonID: personID})
		if err != nil {
			return err
		}
		if rows == 0 {
			return web.Missing("allocation not found")
		}
		return nil
	})
}

func writeContext(ctx context.Context, txq *db.Queries, taskID int64, weekStart WeekStart, unlock bool, now time.Time) (db.WeekWriteContextRow, error) {
	if !unlock && weekStart.IsLocked(now) {
		return db.WeekWriteContextRow{}, web.Locked("historical editing is not enabled")
	}
	contextRow, err := txq.WeekWriteContext(ctx, db.WeekWriteContextParams{TaskID: taskID, WeekStart: weekStart.String()})
	if errors.Is(err, sql.ErrNoRows) {
		return db.WeekWriteContextRow{}, web.Missing("task not found")
	}
	if err != nil {
		return db.WeekWriteContextRow{}, err
	}
	if err := web.HTTPErrorFromReason(int(contextRow.Status), contextRow.Reason); err != nil {
		return db.WeekWriteContextRow{}, err
	}
	return contextRow, nil
}

func requirePersonOnTask(ctx context.Context, txq *db.Queries, taskID, personID int64) error {
	personCtx, err := txq.PersonOnTask(ctx, db.PersonOnTaskParams{TaskID: taskID, PersonID: personID})
	if err != nil {
		return err
	}
	return web.HTTPErrorFromReason(int(personCtx.Status), personCtx.Reason)
}

func savedCell(taskID int64, weekStart WeekStart, planned, spent float64, stored sql.NullFloat64, previous float64) Cell {
	effective := previous
	if stored.Valid {
		effective = max(effective, stored.Float64)
	}
	cell := Cell{
		TaskID: taskID, WeekStart: weekStart,
		PlannedHours: planned, SpentHours: spent,
		Progress:     new(effective),
		Attributions: []DeveloperAllocation{},
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