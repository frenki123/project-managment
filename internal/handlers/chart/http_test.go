package charthandler

import (
	"testing"

	projectdomain "cad-development/internal/project"
)

func TestSeriesUsesClearHourLabels(t *testing.T) {
	chart := series(projectdomain.SCurve{Weeks: []projectdomain.SCurveWeek{{WeekStart: "2026-01-05", PlannedHours: 10, SpentHours: 8, EarnedHours: 6}}})
	if len(chart.Datasets) != 3 {
		t.Fatalf("expected three chart datasets, got %d", len(chart.Datasets))
	}
	for i, want := range []string{"Planned [h]", "Spent [h]", "Earned [h]"} {
		if chart.Datasets[i].Label != want {
			t.Errorf("dataset %d label = %q, want %q", i, chart.Datasets[i].Label, want)
		}
	}
}
