package task

import (
	"context"

	"cad-development/internal/db"
	"cad-development/internal/weekly"
)

// LoadWeeks loads the canonical, project-bounded week series for a task.
func LoadWeeks(ctx context.Context, q *db.Queries, taskID int64) ([]weekly.Cell, error) {
	rows, err := q.ListTaskWeekSeriesByTask(ctx, taskID)
	if err != nil {
		return nil, err
	}
	return weekly.MapWeekSeries(rows)
}
