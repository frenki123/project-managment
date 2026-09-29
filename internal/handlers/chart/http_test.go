package charthandler

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"cad-development/internal/app/testkit"
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

func TestUnknownProjectIsNotRecoveredToAnotherChart(t *testing.T) {
	q := testkit.Open(t)
	if _, err := projectdomain.Create(t.Context(), q, projectdomain.Input{
		Name: "Known", TotalHours: new(10.0), StartDate: "2026-01-05", EndDate: "2026-01-12",
	}); err != nil {
		t.Fatal(err)
	}
	rr := httptest.NewRecorder()
	page(q).ServeHTTP(rr, httptest.NewRequest(http.MethodGet, "/chart?project=999", nil))
	if rr.Code != http.StatusNotFound || !strings.Contains(rr.Body.String(), "project not found") {
		t.Fatalf("got %d %q", rr.Code, rr.Body.String())
	}
}
