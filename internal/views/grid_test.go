package views

import (
	"strings"
	"testing"

	"cad-development/internal/task"
	"cad-development/internal/weekly"
)

func TestGridClass(t *testing.T) {
	tests := []struct {
		name string
		data task.Grid
		want string
	}{
		{name: "project", want: "grid-main"},
		{name: "ideas", data: task.Grid{Ideas: true}, want: "grid-main ideas"},
		{name: "unlocked ideas", data: task.Grid{Ideas: true, HistoryUnlocked: true}, want: "grid-main ideas history-unlocked"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := gridClass(tt.data); got != tt.want {
				t.Fatalf("gridClass() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestGridRendersDistinctIdeasAndProjectColumns(t *testing.T) {
	row := task.GridRow{Name: "Task", ProjectName: "Project", Subproject: "Subproject"}
	ideas := renderGrid(t, task.Grid{Ideas: true, Rows: []task.GridRow{row}})
	if !strings.Contains(ideas, "Assignment") || strings.Contains(ideas, "Planned [h]") || strings.Contains(ideas, "week-cell") {
		t.Fatalf("unexpected ideas grid: %s", ideas)
	}
	if strings.Contains(ideas, "Weekly inputs") || strings.Contains(ideas, "Scroll horizontally") {
		t.Fatalf("removed grid guidance is still rendered: %s", ideas)
	}

	project := renderGrid(t, task.Grid{
		Rows:  []task.GridRow{{Name: "Task", ProjectName: "Project", Subproject: "Subproject", Cells: []task.GridCell{{SavePath: "/tasks/1/weeks/2026-01-05"}}}},
		Weeks: []weekly.WeekInfo{{Number: 1, Date: "05.01", Start: "2026-01-05"}},
	})
	for _, label := range []string{"Planned [h]", "Spent [h]", "Progress [%]", "W1"} {
		if !strings.Contains(project, label) {
			t.Fatalf("project grid is missing %q: %s", label, project)
		}
	}
}

func renderGrid(t *testing.T, data task.Grid) string {
	t.Helper()
	var output strings.Builder
	if err := Grid(data).Render(t.Context(), &output); err != nil {
		t.Fatal(err)
	}
	return output.String()
}
