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
