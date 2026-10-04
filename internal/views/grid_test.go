package views

import (
	"strings"
	"testing"

	"cad-development/internal/db/testkit"
	"cad-development/internal/task"
	"cad-development/internal/weekly"
)

func TestGridClass(t *testing.T) {
	tests := []struct {
		name string
		data task.Grid
		want string
	}{
		{name: "project", data: task.Grid{Kind: task.ViewProject}, want: "grid-main"},
		{name: "ideas", data: task.Grid{Kind: task.ViewIdeas}, want: "grid-main ideas"},
		{name: "ideas ignore historical editing", data: task.Grid{Kind: task.ViewIdeas, HistoricalEditingAllowed: true}, want: "grid-main ideas"},
		{name: "summary ignore historical editing", data: task.Grid{Kind: task.ViewAll, HistoricalEditingAllowed: true}, want: "grid-main"},
		{name: "historical editing allowed", data: task.Grid{Kind: task.ViewProject, HistoricalEditingAllowed: true}, want: "grid-main historical-editing-allowed"},
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
	ideas := renderGrid(t, task.Grid{Kind: task.ViewIdeas, Rows: []task.GridRow{row}})
	if !strings.Contains(ideas, "Assignment") || strings.Contains(ideas, "Planned [h]") || strings.Contains(ideas, "week-cell") || strings.Contains(ideas, "jump-week") || strings.Contains(ideas, "data-current-week") || strings.Contains(ideas, "week-past") || strings.Contains(ideas, "week-current") || strings.Contains(ideas, "week-future") {
		t.Fatalf("unexpected ideas grid: %s", ideas)
	}
	if strings.Contains(ideas, "Weekly inputs") || strings.Contains(ideas, "Scroll horizontally") || strings.Contains(ideas, "Historical editing allowed") || strings.Contains(ideas, "Allow historical editing") || strings.Contains(ideas, "Stop historical editing") {
		t.Fatalf("removed grid guidance is still rendered: %s", ideas)
	}
	if strings.Count(ideas, `<col class="`) != 4 {
		t.Fatalf("ideas grid should have four fixed columns: %s", ideas)
	}

	project := renderGrid(t, task.Grid{
		Kind:        task.ViewProject,
		Rows:        []task.GridRow{{Name: "Task", ProjectName: "Project", Subproject: "Subproject", Status: "Development", Cells: []task.GridCell{{SavePath: "/tasks/1/weeks/2026-01-05"}}}},
		Weeks:       []weekly.WeekInfo{{Number: 1, Date: "05.01", Start: testkit.MustWeek(t, "2026-01-05")}},
		CurrentWeek: "2026-01-05",
	})
	for _, label := range []string{"Status", "Summary", "W1"} {
		if !strings.Contains(project, label) {
			t.Fatalf("project grid is missing %q: %s", label, project)
		}
	}
	if strings.Count(project, `<col class="`) != 6 || !strings.Contains(project, `colspan="5" class="sticky c0"`) {
		t.Fatalf("project grid has unexpected fixed/footer structure: %s", project)
	}
	if !strings.Contains(project, "Status") || !strings.Contains(project, ">Development<") || !strings.Contains(project, `<dl class="task-summary">`) || !strings.Contains(project, "<dt>Planned</dt><dd>0 h</dd>") || !strings.Contains(project, "<dt>Progress</dt><dd>0%</dd>") {
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
	if !strings.Contains(project, `class="jump-week"`) || !strings.Contains(project, `data-current-week="2026-01-05"`) {
		t.Fatalf("project grid is missing the week jump control: %s", project)
	}

	subproject := renderGrid(t, task.Grid{Kind: task.ViewSubproject, Rows: []task.GridRow{{Name: "Task"}}, Weeks: []weekly.WeekInfo{{Number: 1, Date: "05.01", Start: testkit.MustWeek(t, "2026-01-05")}}})
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

func mustWeekInfos(t *testing.T, dates ...string) []weekly.WeekInfo {
	t.Helper()
	infos := make([]weekly.WeekInfo, len(dates))
	for i, date := range dates {
		info, err := testkit.MustWeek(t, date).Info()
		if err != nil {
			t.Fatal(err)
		}
		infos[i] = info
	}
	return infos
}

func TestGridRendersWeekMarkers(t *testing.T) {
	weeks := mustWeekInfos(t, "2026-09-07", "2026-09-14", "2026-09-21")
	data := task.Grid{
		Kind:        task.ViewProject,
		Weeks:       weeks,
		WeekTotals:  []task.GridWeekTotal{{}, {}, {}},
		CurrentWeek: "2026-09-14",
		Rows: []task.GridRow{{
			Name:        "Task",
			ProjectName: "Project",
			Subproject:  "Subproject",
			Status:      "Development",
			Cells: []task.GridCell{
				{WeekStart: weeks[0].Start},
				{WeekStart: weeks[1].Start},
				{WeekStart: weeks[2].Start},
			},
		}},
	}
	out := renderGrid(t, data)
	for _, marker := range []string{
		`class="week-col week-past"`,
		`class="week-col week-current"`,
		`class="week-col week-future"`,
		`data-week="2026-09-14"`,
		`class="week-cell week-past"`,
		`class="week-cell week-current"`,
		`class="week-cell week-future"`,
		`class="week-total week-past"`,
		`class="week-total week-current"`,
		`class="week-total week-future"`,
		`class="jump-week"`,
		`data-current-week="2026-09-14"`,
		`>Jump to current week<`,
	} {
		if !strings.Contains(out, marker) {
			t.Fatalf("grid is missing %q: %s", marker, out)
		}
	}
}

func TestGridRendersNoWeekMarkersWhenCurrentWeekEmpty(t *testing.T) {
	weeks := mustWeekInfos(t, "2026-09-07", "2026-09-14", "2026-09-21")
	data := task.Grid{
		Kind:       task.ViewProject,
		Weeks:      weeks,
		WeekTotals: []task.GridWeekTotal{{}, {}, {}},
		Rows: []task.GridRow{{
			Name: "Task",
			Cells: []task.GridCell{
				{WeekStart: weeks[0].Start},
				{WeekStart: weeks[1].Start},
				{WeekStart: weeks[2].Start},
			},
		}},
	}
	out := renderGrid(t, data)
	for _, marker := range []string{"week-current", "week-past", "week-future"} {
		if strings.Contains(out, marker) {
			t.Fatalf("grid should not render %q: %s", marker, out)
		}
	}
	if !strings.Contains(out, `class="jump-week"`) || !strings.Contains(out, `>Jump to current week<`) {
		t.Fatalf("grid should still render the jump button: %s", out)
	}
}
