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
	row := task.GridRow{Name: "Task", ProjectName: "Project", Subproject: "Subproject", Status: "Development"}
	ideas := renderGrid(t, task.Grid{Ideas: true, Rows: []task.GridRow{row}})
	if !strings.Contains(ideas, "Assignment") || strings.Contains(ideas, "Planned [h]") || strings.Contains(ideas, "week-cell") {
		t.Fatalf("unexpected ideas grid: %s", ideas)
	}
	if strings.Contains(ideas, "Weekly inputs") || strings.Contains(ideas, "Scroll horizontally") || strings.Contains(ideas, "History unlocked") || strings.Contains(ideas, "Unlock history") || strings.Contains(ideas, "Lock history") {
		t.Fatalf("removed grid guidance is still rendered: %s", ideas)
	}
	if strings.Count(ideas, `<col class="`) != 4 {
		t.Fatalf("ideas grid should have four fixed columns: %s", ideas)
	}

	project := renderGrid(t, task.Grid{
		Rows:  []task.GridRow{{Name: "Task", ProjectName: "Project", Subproject: "Subproject", Status: "Development", Cells: []task.GridCell{{SavePath: "/tasks/1/weeks/2026-01-05"}}}},
		Weeks: []weekly.WeekInfo{{Number: 1, Date: "05.01", Start: "2026-01-05"}},
	})
	for _, label := range []string{"Status", "Summary", "W1"} {
		if !strings.Contains(project, label) {
			t.Fatalf("project grid is missing %q: %s", label, project)
		}
	}
	if strings.Count(project, `<col class="`) != 6 || !strings.Contains(project, `colspan="5" class="sticky c0"`) {
		t.Fatalf("project grid has unexpected fixed/footer structure: %s", project)
	}
	if !strings.Contains(project, "Status") || !strings.Contains(project, ">Development<") || !strings.Contains(project, "Planned 0 h") || !strings.Contains(project, "Spent 0 h") || !strings.Contains(project, "Progress 0%") {
		t.Fatalf("project grid is missing task status: %s", project)
	}
	for _, label := range []string{"Plan", "Spent"} {
		if !strings.Contains(project, ">"+label+"<") {
			t.Fatalf("project footer is missing %q: %s", label, project)
		}
	}
	if strings.Contains(project, ">Totals<") {
		t.Fatalf("redundant totals label is still rendered: %s", project)
	}
	if strings.Contains(project, ">Earned<") {
		t.Fatalf("project footer should not render earned values: %s", project)
	}
	if strings.Contains(project, "Cum plan") || strings.Contains(project, "Cum spent") {
		t.Fatalf("cumulative footer values are still rendered: %s", project)
	}

	subproject := renderGrid(t, task.Grid{Rows: []task.GridRow{{Name: "Task"}}, Weeks: []weekly.WeekInfo{{Number: 1, Date: "05.01", Start: "2026-01-05"}}})
	if strings.Contains(subproject, ">Earned<") {
		t.Fatalf("subproject footer should not render earned values: %s", subproject)
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
