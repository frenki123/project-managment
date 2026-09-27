package monthlock

import (
	"context"
	"time"

	"cad-development/internal/db"
)

type State struct {
	Unlocked bool `json:"unlocked"`
}

func Locked(month time.Time, now time.Time, unlocked bool) bool {
	return !unlocked && (month.Year() < now.Year() || month.Year() == now.Year() && month.Month() < now.Month())
}

func WeekLocked(weekStart string, now time.Time, unlocked bool) bool {
	week, err := time.Parse(time.DateOnly, weekStart)
	if err != nil || week.Weekday() != time.Monday {
		return true
	}
	return Locked(week, now, unlocked)
}

func Unlocked(ctx context.Context, q *db.Queries) (bool, error) {
	value, err := q.GetGlobalUnlock(ctx)
	return value != 0, err
}

func Set(ctx context.Context, q *db.Queries, unlocked bool) error {
	value := int64(0)
	if unlocked {
		value = 1
	}
	return q.SetGlobalUnlock(ctx, value)
}
