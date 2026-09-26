package weekly

import (
	"time"
)

type WeekStart string

func ParseDate(s string) (time.Time, error) {
	t, err := time.Parse(time.DateOnly, s)
	if err != nil {
		return time.Time{}, Invalid("invalid date")
	}
	return t, nil
}

func MondayOnOrBefore(t time.Time) time.Time {
	t = time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, time.UTC)
	wd := t.Weekday()
	if wd == time.Sunday {
		return t.AddDate(0, 0, -6)
	}
	return t.AddDate(0, 0, -int(wd-time.Monday))
}

func ParseMonday(s string) (time.Time, error) {
	t, err := ParseDate(s)
	if err != nil {
		return time.Time{}, err
	}
	if t.Weekday() != time.Monday {
		return time.Time{}, Invalid("week_start must be a Monday")
	}
	return t, nil
}

func ParseWeekStart(s WeekStart) (time.Time, error) {
	return ParseMonday(string(s))
}

func WeekStarts(start, end time.Time) []WeekStart {
	w := MondayOnOrBefore(start)
	last := MondayOnOrBefore(end)
	var out []WeekStart
	for !w.After(last) {
		out = append(out, WeekStart(w.Format(time.DateOnly)))
		w = w.AddDate(0, 0, 7)
	}
	return out
}
