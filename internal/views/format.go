package views

import (
	"strconv"
	"time"

	"cad-development/internal/weekly"
)

type MonthGroup struct {
	Month string
	Label string
	Count int
}

func formatNum(v float64) string {
	return strconv.FormatFloat(v, 'f', -1, 64)
}

func overrunClass(overrun bool) string {
	if overrun {
		return "overrun"
	}
	return ""
}

func progressClass(stored bool) string {
	if stored {
		return "stored-progress"
	}
	return "carried-progress"
}

func progressTitle(stored bool) string {
	if stored {
		return "Progress entered for this week"
	}
	return "Progress carried forward from an earlier week"
}

func monthLabel(ym string) string {
	if parsed, err := time.Parse("2006-01", ym); err == nil {
		return parsed.Format("January 2006")
	}
	return ym
}

func dateLabel(value string) string {
	if parsed, err := time.Parse(time.DateOnly, value); err == nil {
		return parsed.Format("02 Jan 2006")
	}
	return value
}

func distinctMonths(weeks []weekly.WeekInfo) []MonthGroup {
	var groups []MonthGroup
	for _, week := range weeks {
		if len(groups) > 0 && groups[len(groups)-1].Month == week.Month {
			groups[len(groups)-1].Count++
			continue
		}
		groups = append(groups, MonthGroup{Month: week.Month, Label: week.MonthLabel, Count: 1})
	}
	return groups
}
