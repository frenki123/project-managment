package monthlock

import (
	"context"
	"net/http"
	"time"

	"cad-development/internal/apperr"
	"cad-development/internal/db"
)

func YearMonth(t time.Time) string {
	return t.Format("2006-01")
}

func ParseYearMonth(s string) (time.Time, error) {
	t, err := time.Parse("2006-01", s)
	if err != nil {
		return time.Time{}, apperr.New(http.StatusBadRequest, "invalid year_month")
	}
	return t, nil
}

func PreviousMonth(now time.Time) string {
	y, m, _ := now.Date()
	if m == time.January {
		return time.Date(y-1, time.December, 1, 0, 0, 0, 0, now.Location()).Format("2006-01")
	}
	return time.Date(y, m-1, 1, 0, 0, 0, 0, now.Location()).Format("2006-01")
}

func IsPastMonth(yearMonth string, now time.Time) bool {
	parsed, err := ParseYearMonth(yearMonth)
	return err == nil && parsed.Format("2006-01") < YearMonth(now)
}

func Locked(yearMonth string, now time.Time, unlocked map[string]bool) bool {
	if !IsPastMonth(yearMonth, now) {
		return false
	}
	return !unlocked[yearMonth]
}

func WeekLocked(weekStart string, now time.Time, unlocked map[string]bool) bool {
	week, err := time.Parse(time.DateOnly, weekStart)
	if err != nil || week.Weekday() != time.Monday {
		return true
	}
	return Locked(week.Format("2006-01"), now, unlocked)
}

func UnlockedSet(ctx context.Context, q *db.Queries) (map[string]bool, error) {
	months, err := q.ListUnlockedMonths(ctx)
	if err != nil {
		return nil, err
	}
	out := make(map[string]bool, len(months))
	for _, m := range months {
		out[m] = true
	}
	return out, nil
}

func Set(ctx context.Context, q *db.Queries, yearMonth string, unlocked bool, now time.Time) error {
	if _, err := ParseYearMonth(yearMonth); err != nil {
		return err
	}
	if !IsPastMonth(yearMonth, now) {
		return apperr.New(http.StatusBadRequest, "only past months can be unlocked")
	}
	u := int64(0)
	if unlocked {
		u = 1
	}
	_, err := q.UpsertMonthLock(ctx, db.UpsertMonthLockParams{
		YearMonth: yearMonth,
		Unlocked:  u,
	})
	return err
}

func MonthRange(from, to time.Time) []string {
	if to.Before(from) {
		from, to = to, from
	}
	cur := time.Date(from.Year(), from.Month(), 1, 0, 0, 0, 0, time.UTC)
	end := time.Date(to.Year(), to.Month(), 1, 0, 0, 0, 0, time.UTC)
	var out []string
	for !cur.After(end) {
		out = append(out, YearMonth(cur))
		cur = cur.AddDate(0, 1, 0)
	}
	return out
}

func PastMonths(from time.Time, now time.Time) []string {
	prev, err := time.Parse("2006-01", PreviousMonth(now))
	if err != nil {
		return nil
	}
	start := time.Date(from.Year(), from.Month(), 1, 0, 0, 0, 0, time.UTC)
	if start.After(prev) {
		return nil
	}
	return MonthRange(start, prev)
}
