package views

import "strconv"

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

func distinctMonths(weeks []WeekHeader) []MonthGroup {
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
