package weekly

import "cad-development/internal/db"

// MapWeekSeries maps v_task_week_series rows to the shared weekly read model.
func MapWeekSeries(rows []db.VTaskWeekSeries) ([]Cell, error) {
	out := make([]Cell, 0, len(rows))
	for _, row := range rows {
		weekStart, err := Parse(row.WeekStart)
		if err != nil {
			return nil, err
		}
		cell := Cell{
			TaskID: row.TaskID, WeekStart: weekStart,
			PlannedHours: row.PlannedHours, SpentHours: row.SpentHours,
			Progress: new(row.EffectiveProgress),
		}
		if row.StoredProgress.Valid {
			cell.StoredProgress = new(row.StoredProgress.Float64)
		}
		out = append(out, cell)
	}
	return out, nil
}
