package project

import (
	"context"

	"cad-development/internal/db"
)

type SCurve struct {
	Project Project      `json:"project"`
	Weeks   []SCurveWeek `json:"weeks"`
}

type SCurveWeek struct {
	WeekStart    string  `json:"week_start"`
	PlannedHours float64 `json:"planned_hours"`
	SpentHours   float64 `json:"spent_hours"`
	EarnedHours  float64 `json:"earned_hours"`
}

func LoadSCurve(ctx context.Context, q *db.Queries, id int64) (SCurve, error) {
	p, err := Get(ctx, q, id)
	if err != nil {
		return SCurve{}, err
	}
	rows, err := q.ListProjectWeekTotals(ctx, id)
	if err != nil {
		return SCurve{}, err
	}
	curve := SCurve{Project: p, Weeks: make([]SCurveWeek, 0, len(rows))}
	for _, row := range rows {
		curve.Weeks = append(curve.Weeks, SCurveWeek{
			WeekStart:    row.WeekStart,
			PlannedHours: row.CumulativePlannedHours,
			SpentHours:   row.CumulativeSpentHours,
			EarnedHours:  row.EarnedHours,
		})
	}
	return curve, nil
}
