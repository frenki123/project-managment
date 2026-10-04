package task

import (
	"context"

	"cad-development/internal/db"
	"cad-development/internal/weekly"
)

// LoadWeeks loads the canonical, project-bounded week series for a task, enriched with developer attributions.
func LoadWeeks(ctx context.Context, q *db.Queries, taskID int64) ([]weekly.Cell, error) {
	rows, err := q.ListTaskWeekSeriesByTask(ctx, taskID)
	if err != nil {
		return nil, err
	}
	cells, err := weekly.MapWeekSeries(rows)
	if err != nil {
		return nil, err
	}
	attributions, err := q.ListTaskWeekDevelopersByTask(ctx, taskID)
	if err != nil {
		return nil, err
	}
	return attachAttributions(cells, attributions), nil
}

func attachAttributions(cells []weekly.Cell, rows []db.ListTaskWeekDevelopersByTaskRow) []weekly.Cell {
	byWeek := make(map[string][]weekly.DeveloperAllocation, len(rows))
	for _, row := range rows {
		byWeek[row.WeekStart] = append(byWeek[row.WeekStart], weekly.DeveloperAllocation{
			PersonID: row.PersonID, Name: row.PersonName, PlannedHours: row.PlannedHours, SpentHours: row.SpentHours,
		})
	}
	for i := range cells {
		if allocations, ok := byWeek[cells[i].WeekStart.String()]; ok {
			cells[i].Attributions = allocations
		}
	}
	return cells
}