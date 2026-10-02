package weekly

import (
	"encoding/json"
	"time"

	"cad-development/internal/web"
)

type WeekStart struct {
	t time.Time
}

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
		return WeekStart{}, web.Invalid("week_start must be a Monday")
	}
	return WeekStart{t: t}, nil
}

func fromMonday(t time.Time) WeekStart {
	return WeekStart{t: time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, time.UTC)}
}

func (w WeekStart) Time() time.Time { return w.t }

func (w WeekStart) IsZero() bool { return w.t.IsZero() }

func (w WeekStart) Next() WeekStart { return fromMonday(w.t.AddDate(0, 0, 7)) }

func (w WeekStart) IsLocked(now time.Time) bool {
	if w.t.IsZero() {
		return true
	}
	return w.t.Year() < now.Year() || w.t.Year() == now.Year() && w.t.Month() < now.Month()
}

func (w WeekStart) Info() (WeekInfo, error) {
	if w.t.IsZero() {
		return WeekInfo{}, web.Invalid("week_start must be a Monday")
	}
	_, number := w.t.ISOWeek() //nolint:droppedvalue -- only the ISO week number is used, not the ISO year
	return WeekInfo{w, number, w.t.Format("02.01"), w.t.Format("2006-01"), w.t.Format("January 2006")}, nil
}

func (w WeekStart) String() string {
	if w.t.IsZero() {
		return ""
	}
	return w.t.Format(time.DateOnly)
}

func (w WeekStart) MarshalJSON() ([]byte, error) { return json.Marshal(w.String()) }

func (w *WeekStart) UnmarshalJSON(data []byte) error {
	var s string
	if err := json.Unmarshal(data, &s); err != nil {
		return err
	}
	parsed, err := Parse(s)
	if err != nil {
		return err
	}
	*w = parsed
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
	w := fromMonday(MondayOnOrBefore(start))
	last := MondayOnOrBefore(end)
	var out []WeekStart
	for !w.Time().After(last) {
		out = append(out, w)
		w = w.Next()
	}
	return out
}
