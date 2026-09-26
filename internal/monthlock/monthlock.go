package monthlock

import (
	"context"
	"time"

	"cad-development/internal/db"
)

const yearMonthLayout = "2006-01"

type YearMonth string

type MonthLock struct {
	YearMonth YearMonth `json:"year_month"`
	Unlocked  bool      `json:"unlocked"`
}

type Set map[YearMonth]bool

func (s Set) Contains(month YearMonth) bool {
	return s[month]
}

func NewSet(months ...YearMonth) Set {
	set := make(Set, len(months))
	for _, month := range months {
		set[month] = true
	}
	return set
}

type LocksResponse struct {
	MonthLocks []MonthLock `json:"month_locks"`
	LastMonth  YearMonth   `json:"last_month"`
}

type SetResponse struct {
	YearMonth YearMonth `json:"year_month"`
	Unlocked  bool      `json:"unlocked"`
}

func Of(t time.Time) YearMonth {
	return YearMonth(t.Format(yearMonthLayout))
}

func ParseYearMonth(s YearMonth) (time.Time, error) {
	t, err := time.Parse(yearMonthLayout, string(s))
	if err != nil {
		return time.Time{}, Invalid("invalid year_month")
	}
	return t, nil
}

func PreviousMonth(now time.Time) YearMonth {
	y, m, _ := now.Date()
	if m == time.January {
		return Of(time.Date(y-1, time.December, 1, 0, 0, 0, 0, now.Location()))
	}
	return Of(time.Date(y, m-1, 1, 0, 0, 0, 0, now.Location()))
}

func IsPastMonth(yearMonth YearMonth, now time.Time) bool {
	parsed, err := ParseYearMonth(yearMonth)
	return err == nil && parsed.Format(yearMonthLayout) < string(Of(now))
}

func Locked(yearMonth YearMonth, now time.Time, unlocked Set) bool {
	if !IsPastMonth(yearMonth, now) {
		return false
	}
	return !unlocked.Contains(yearMonth)
}

func WeekLocked(weekStart string, now time.Time, unlocked Set) bool {
	week, err := time.Parse(time.DateOnly, weekStart)
	if err != nil || week.Weekday() != time.Monday {
		return true
	}
	return Locked(YearMonth(week.Format(yearMonthLayout)), now, unlocked)
}

func UnlockedSet(ctx context.Context, q *db.Queries) (Set, error) {
	months, err := q.ListUnlockedMonths(ctx)
	if err != nil {
		return nil, err
	}
	out := make(Set, len(months))
	for _, month := range months {
		out[YearMonth(month)] = true
	}
	return out, nil
}

func List(ctx context.Context, q *db.Queries) ([]MonthLock, error) {
	rows, err := q.ListMonthLocks(ctx)
	if err != nil {
		return nil, err
	}
	if rows == nil {
		return []MonthLock{}, nil
	}
	out := make([]MonthLock, 0, len(rows))
	for _, row := range rows {
		out = append(out, MonthLock{YearMonth: YearMonth(row.YearMonth), Unlocked: row.Unlocked != 0})
	}
	return out, nil
}

func SetMonth(ctx context.Context, q *db.Queries, yearMonth YearMonth, unlocked bool, now time.Time) error {
	if _, err := ParseYearMonth(yearMonth); err != nil {
		return err
	}
	if !IsPastMonth(yearMonth, now) {
		return Invalid("only past months can be unlocked")
	}
	unlockedValue := int64(0)
	if unlocked {
		unlockedValue = 1
	}
	_, err := q.UpsertMonthLock(ctx, db.UpsertMonthLockParams{
		YearMonth: string(yearMonth),
		Unlocked:  unlockedValue,
	})
	return err
}

func MonthRange(from, to time.Time) []YearMonth {
	if to.Before(from) {
		from, to = to, from
	}
	cur := time.Date(from.Year(), from.Month(), 1, 0, 0, 0, 0, time.UTC)
	end := time.Date(to.Year(), to.Month(), 1, 0, 0, 0, 0, time.UTC)
	var out []YearMonth
	for !cur.After(end) {
		out = append(out, Of(cur))
		cur = cur.AddDate(0, 1, 0)
	}
	return out
}

func PastMonths(from time.Time, now time.Time) []YearMonth {
	prev := time.Date(now.Year(), now.Month()-1, 1, 0, 0, 0, 0, time.UTC)
	start := time.Date(from.Year(), from.Month(), 1, 0, 0, 0, 0, time.UTC)
	if start.After(prev) {
		return nil
	}
	return MonthRange(start, prev)
}
