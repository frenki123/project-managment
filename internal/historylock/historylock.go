package historylock

import (
	"time"
)

func IsWeekLocked(weekStart string, now time.Time) bool {
	week, err := time.Parse(time.DateOnly, weekStart)
	if err != nil || week.Weekday() != time.Monday {
		return true
	}
	return IsPastMonth(week, now)
}

func IsPastMonth(month time.Time, now time.Time) bool {
	return month.Year() < now.Year() || month.Year() == now.Year() && month.Month() < now.Month()
}
