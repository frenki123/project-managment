package weekly

import "cad-development/internal/db"

// EffectiveProgress returns the carried-forward progress for each requested week.
// Both input slices must be ordered by week_start.
func EffectiveProgress(stored []db.TaskWeek, requested []string) map[string]float64 {
	effective := make(map[string]float64, len(requested))
	previous := 0.0
	storedIndex := 0
	for _, week := range requested {
		for storedIndex < len(stored) && stored[storedIndex].WeekStart <= week {
			if stored[storedIndex].Progress.Valid {
				previous = stored[storedIndex].Progress.Float64
			}
			storedIndex++
		}
		effective[week] = previous
	}
	return effective
}
