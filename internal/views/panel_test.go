package views

import (
	"strings"
	"testing"

	"cad-development/internal/db/testkit"
	"cad-development/internal/task"
	"cad-development/internal/weekly"
)

func TestTaskPanelWeeklyNotes(t *testing.T) {
	data := TaskPanelData{
		Task: task.GridRow{Name: "T"},
		Weeks: []weekly.Cell{
			{WeekStart: testkit.MustWeek(t, "2026-01-05"), Note: "blocked on drawings"},
			{WeekStart: testkit.MustWeek(t, "2026-01-12")},
		},
	}
	html := renderPanel(t, data)
	for _, want := range []string{"Weekly notes", "W2 · 05.01", "blocked on drawings"} {
		if !strings.Contains(html, want) {
			t.Fatalf("panel is missing %q: %s", want, html)
		}
	}
	if strings.Contains(html, "W3") {
		t.Fatalf("panel should not list weeks without notes: %s", html)
	}

	week := testkit.MustWeek(t, "2026-01-05")
	for _, noNotes := range []TaskPanelData{
		{Task: task.GridRow{Name: "T"}},
		{Task: task.GridRow{Name: "T"}, Weeks: []weekly.Cell{{WeekStart: week}}},
	} {
		if html := renderPanel(t, noNotes); strings.Contains(html, "Weekly notes") {
			t.Fatalf("panel without notes should not render the section: %s", html)
		}
	}
}

func renderPanel(t *testing.T, data TaskPanelData) string {
	t.Helper()
	var output strings.Builder
	if err := TaskPanel(data).Render(t.Context(), &output); err != nil {
		t.Fatal(err)
	}
	return output.String()
}
