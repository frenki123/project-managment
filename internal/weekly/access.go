package weekly

import "time"

func IsWeekLocked(weekStart string, now time.Time) bool {
	week, err := time.Parse(time.DateOnly, weekStart)
	if err != nil || week.Weekday() != time.Monday {
		return true
	}
	return week.Year() < now.Year() || week.Year() == now.Year() && week.Month() < now.Month()
}
