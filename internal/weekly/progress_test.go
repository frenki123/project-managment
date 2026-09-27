package weekly_test

import (
	"database/sql"
	"testing"

	"cad-development/internal/db"
	"cad-development/internal/weekly"
)

func TestEffectiveProgressCarriesAcrossMissingWeeks(t *testing.T) {
	stored := []db.TaskWeek{
		{WeekStart: "2026-01-05", Progress: sql.NullFloat64{Float64: 20, Valid: true}},
		{WeekStart: "2026-01-19", Progress: sql.NullFloat64{Float64: 50, Valid: true}},
	}
	got := weekly.EffectiveProgress(stored, []string{"2026-01-05", "2026-01-12", "2026-01-19", "2026-01-26"})
	want := map[string]float64{
		"2026-01-05": 20,
		"2026-01-12": 20,
		"2026-01-19": 50,
		"2026-01-26": 50,
	}
	for week, expected := range want {
		if got[week] != expected {
			t.Errorf("week %s: got %v, want %v", week, got[week], expected)
		}
	}
}
