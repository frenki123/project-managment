package weekly

import (
	"encoding/json"
	"time"

	"cad-development/internal/app"
)

type WeekStart string

type WeekInfo struct {
	Start      WeekStart
	Number     int
	Date       string
	Month      string
	MonthLabel string
}

func Parse(s string) (WeekStart, error) {
	t, err := time.Parse(time.DateOnly, s)
	if err != nil || t.Weekday() != time.Monday {
		return "", app.Invalid("week_start must be a Monday")
	}
	return WeekStart(t.Format(time.DateOnly)), nil
}

func (w WeekStart) Time() time.Time {
	t, _ := time.Parse(time.DateOnly, string(w))
	return t
}

func (w WeekStart) Next() WeekStart {
	return WeekStart(w.Time().AddDate(0, 0, 7).Format(time.DateOnly))
}

func (w WeekStart) IsLocked(now time.Time) bool {
	t := w.Time()
	if t.IsZero() || t.Weekday() != time.Monday {
		return true
	}
	return t.Year() < now.Year() || t.Year() == now.Year() && t.Month() < now.Month()
}

func (w WeekStart) Info() (WeekInfo, error) {
	t := w.Time()
	if t.IsZero() || t.Weekday() != time.Monday {
		return WeekInfo{}, app.Invalid("week_start must be a Monday")
	}
	_, number := t.ISOWeek()
	return WeekInfo{w, number, t.Format("02.01"), t.Format("2006-01"), t.Format("January 2006")}, nil
}

func (w WeekStart) String() string { return string(w) }

func (w WeekStart) MarshalJSON() ([]byte, error) { return json.Marshal(string(w)) }

func (w *WeekStart) UnmarshalJSON(data []byte) error {
	var s string
	if err := json.Unmarshal(data, &s); err != nil {
		return err
	}
	*w = WeekStart(s)
	return nil
}

func MondayOnOrBefore(t time.Time) time.Time {
	t = time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, time.UTC)
	wd := t.Weekday()
	if wd == time.Sunday {
		return t.AddDate(0, 0, -6)
	}
	return t.AddDate(0, 0, -int(wd-time.Monday))
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
