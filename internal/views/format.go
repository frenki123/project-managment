package views

import (
	"math"
	"strconv"
	"strings"
	"time"

	"cad-development/internal/weekly"
)

type MonthGroup struct {
	Month string
	Label string
	Count int
}

// hours renders hours with at most one decimal, dropping a trailing ".0".
func hours(v float64) string {
	return strings.TrimSuffix(strconv.FormatFloat(v, 'f', 1, 64), ".0")
}

// percent renders a percentage as a whole number, rounding down.
func percent(v float64) string {
	return strconv.FormatFloat(math.Floor(v), 'f', 0, 64)
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
