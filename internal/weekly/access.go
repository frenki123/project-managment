package weekly

import "time"

func IsWeekLocked(weekStart string, now time.Time) bool {
	week, err := time.Parse(time.DateOnly, weekStart)
	if err != nil || week.Weekday() != time.Monday {
		return true
	}
	return isPreviousCalendarMonth(week, now)
}

func isPreviousCalendarMonth(date time.Time, now time.Time) bool {
	return date.Year() < now.Year() || date.Year() == now.Year() && date.Month() < now.Month()
}
