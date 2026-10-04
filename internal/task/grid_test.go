package task_test

import (
	"errors"
	"net/http"
	"strconv"
	"testing"
	"time"

	"cad-development/internal/db/testkit"
	"cad-development/internal/nullable"
	"cad-development/internal/project"
	"cad-development/internal/subproject"
	"cad-development/internal/task"
	"cad-development/internal/web"
	"cad-development/internal/weekly"
)

func TestGridCarriesProgressAcrossMissingWeeks(t *testing.T) {
	ctx := t.Context()
	q := testkit.Open(t)
	p, err := project.Create(ctx, q, project.Input{
		Name: "Progress gaps", TotalHours: new(10.0), StartDate: "2026-09-07", EndDate: "2026-09-28",
	})
	if err != nil {
		t.Fatal(err)
	}
	tk, err := task.Create(ctx, q, task.Input{Name: "Tracked", ProjectID: &p.ID})
	if err != nil {
		t.Fatal(err)
	}
	now := time.Date(2026, 9, 26, 0, 0, 0, 0, time.UTC)
	first, later := 20.0, 40.0
	cell, err := weekly.Save(ctx, q, tk.ID, testkit.MustWeek(t, "2026-09-07"), weekly.Patch{Progress: nullable.Present(first)}, now)
	if err != nil {
		t.Fatal(err)
	}
	if cell.Progress == nil || *cell.Progress != first {
		t.Fatalf("week 1 save should store progress %v: %#v", first, cell)
	}
	cell, err = weekly.Save(ctx, q, tk.ID, testkit.MustWeek(t, "2026-09-21"), weekly.Patch{Progress: nullable.Present(later)}, now)
	if err != nil {
		t.Fatal(err)
	}
	if cell.Progress == nil || *cell.Progress != later {
		t.Fatalf("later save should store progress %v: %#v", later, cell)
	}
	grid, err := task.LoadGrid(ctx, q, mustFilter(t, strconv.FormatInt(p.ID, 10), ""), now, false)
	if err != nil {
		t.Fatal(err)
	}
	for i, week := range grid.Weeks {
		if week.Start.String() == "2026-09-14" {
			if grid.Rows[0].Cells[i].Progress != first || grid.Rows[0].Cells[i].Stored {
				t.Fatalf("missing week did not carry SQL progress: %#v", grid.Rows[0].Cells[i])
			}
			return
		}
	}
	t.Fatal("missing gap week")
}

func TestGridReportsHoursAndProgressSeparately(t *testing.T) {
	ctx := t.Context()
	q := testkit.Open(t)
	p, err := project.Create(ctx, q, project.Input{
		Name: "P", TotalHours: new(300.0), StartDate: "2026-09-01", EndDate: "2026-10-31",
	})
	if err != nil {
		t.Fatal(err)
	}
	first, err := task.Create(ctx, q, task.Input{Name: "Half complete", ProjectID: &p.ID})
	if err != nil {
		t.Fatal(err)
	}
	second, err := task.Create(ctx, q, task.Input{Name: "Not started", ProjectID: &p.ID})
	if err != nil {
		t.Fatal(err)
	}
	now := time.Date(2026, 9, 26, 0, 0, 0, 0, time.UTC)
	firstPlanned := 150.0
	firstSpent := 100.0
	secondPlanned := 50.0
	secondSpent := 100.0
	progress := 50.0
	cell, err := weekly.Save(ctx, q, first.ID, testkit.MustWeek(t, "2026-09-07"), weekly.Patch{PlannedHours: nullable.Present(firstPlanned), SpentHours: nullable.Present(firstSpent), Progress: nullable.Present(progress)}, now)
	if err != nil {
		t.Fatal(err)
	}
	if cell.PlannedHours != firstPlanned || cell.SpentHours != firstSpent || cell.Progress == nil || *cell.Progress != progress {
		t.Fatalf("first task save returned unexpected cell: %#v", cell)
	}
	cell, err = weekly.Save(ctx, q, second.ID, testkit.MustWeek(t, "2026-09-07"), weekly.Patch{PlannedHours: nullable.Present(secondPlanned), SpentHours: nullable.Present(secondSpent)}, now)
	if err != nil {
		t.Fatal(err)
	}
	if cell.PlannedHours != secondPlanned || cell.SpentHours != secondSpent || cell.Progress == nil || *cell.Progress != 0 {
		t.Fatalf("second task save should carry zero progress: %#v", cell)
	}

	grid, err := task.LoadGrid(ctx, q, mustFilter(t, strconv.FormatInt(p.ID, 10), ""), now, false)
	if err != nil {
		t.Fatal(err)
	}
	if grid.Kind != task.ViewProject {
		t.Fatalf("unexpected project view kind: %v", grid.Kind)
	}
	if grid.PlannedHours != 200 || grid.SpentHours != 200 {
		t.Fatalf("unexpected hour totals: planned=%v spent=%v", grid.PlannedHours, grid.SpentHours)
	}
	if grid.ProgressPct == nil || *grid.ProgressPct != 25 {
		t.Fatalf("expected 25%% progress, got %v", grid.ProgressPct)
	}
	for i, week := range grid.Weeks {
		if week.Start.String() != "2026-09-07" {
			continue
		}
		total := grid.WeekTotals[i]
		if total.Planned != 200 || total.Spent != 200 {
			t.Fatalf("unexpected SQL week total: %#v", total)
		}
		goto weekTotalFound
	}
	t.Fatal("missing SQL week total")

weekTotalFound:
	foundCarryForward := false
	for _, row := range grid.Rows {
		if row.Name != "Half complete" {
			continue
		}
		for _, cell := range row.Cells {
			if cell.WeekStart.String() == "2026-09-14" {
				foundCarryForward = true
				if cell.Progress != 50 {
					t.Fatalf("expected progress to carry forward, got %v", cell.Progress)
				}
			}
		}
	}
	if !foundCarryForward {
		t.Fatal("expected a week without stored progress")
	}
}

func TestUpdateRejectsReassignmentWithWeeklyData(t *testing.T) {
	ctx := t.Context()
	q := testkit.Open(t)
	first, err := project.Create(ctx, q, project.Input{Name: "First", TotalHours: new(10.0), StartDate: "2026-01-05", EndDate: "2026-02-01"})
	if err != nil {
		t.Fatal(err)
	}
	second, err := project.Create(ctx, q, project.Input{Name: "Second", TotalHours: new(10.0), StartDate: "2026-01-05", EndDate: "2026-02-01"})
	if err != nil {
		t.Fatal(err)
	}
	firstID := first.ID
	item, err := task.Create(ctx, q, task.Input{Name: "Tracked", ProjectID: &firstID})
	if err != nil {
		t.Fatal(err)
	}
	hours := 2.0
	if _, err := weekly.Save(ctx, q, item.ID, testkit.MustWeek(t, "2026-01-05"), weekly.Patch{PlannedHours: nullable.Present(hours)}, time.Date(2026, 1, 10, 0, 0, 0, 0, time.UTC)); err != nil {
		t.Fatal(err)
	}
	secondID := second.ID
	_, err = task.Update(ctx, q, item.ID, task.Patch{
		Name: nullable.Present(item.Name), ProjectID: nullable.Present(&secondID),
	})
	httpErr, ok := errors.AsType[web.HTTPError](err)
	if !ok || httpErr.Status != http.StatusConflict {
		t.Fatalf("got %v", err)
	}
	got, err := task.Get(ctx, q, item.ID)
	if err != nil {
		t.Fatal(err)
	}
	if got.ProjectID == nil || *got.ProjectID != first.ID {
		t.Fatalf("task moved after rejected update: %v", got.ProjectID)
	}
}

func TestGridUsesSQLSubprojectTotals(t *testing.T) {
	ctx := t.Context()
	q := testkit.Open(t)
	p, err := project.Create(ctx, q, project.Input{Name: "Project", TotalHours: new(20.0), StartDate: "2026-09-01", EndDate: "2026-10-31"})
	if err != nil {
		t.Fatal(err)
	}
	sp, err := subproject.Create(ctx, q, subproject.Input{ProjectID: p.ID, Name: "Subproject", TotalHours: new(10.0)})
	if err != nil {
		t.Fatal(err)
	}
	projectID := p.ID
	subprojectID := sp.ID
	item, err := task.Create(ctx, q, task.Input{Name: "Tracked", ProjectID: &projectID, SubprojectID: &subprojectID})
	if err != nil {
		t.Fatal(err)
	}
	planned, spent, progress := 6.0, 4.0, 50.0
	now := time.Date(2026, 9, 26, 0, 0, 0, 0, time.UTC)
	cell, err := weekly.Save(ctx, q, item.ID, testkit.MustWeek(t, "2026-09-07"), weekly.Patch{PlannedHours: nullable.Present(planned), SpentHours: nullable.Present(spent), Progress: nullable.Present(progress)}, now)
	if err != nil {
		t.Fatal(err)
	}
	if cell.PlannedHours != planned || cell.SpentHours != spent || cell.Progress == nil || *cell.Progress != progress {
		t.Fatalf("save returned unexpected cell: %#v", cell)
	}
	grid, err := task.LoadGrid(ctx, q, mustFilter(t, strconv.FormatInt(p.ID, 10), strconv.FormatInt(subprojectID, 10)), now, false)
	if err != nil {
		t.Fatal(err)
	}
	if grid.Kind != task.ViewSubproject {
		t.Fatalf("unexpected subproject view kind: %v", grid.Kind)
	}
	if grid.PlannedHours != planned || grid.SpentHours != spent {
		t.Fatalf("unexpected subproject totals: planned=%v spent=%v", grid.PlannedHours, grid.SpentHours)
	}
	if grid.ProgressPct != nil || len(grid.Rows) != 1 || grid.Rows[0].TotalHours != planned || grid.Rows[0].SpentHours != spent || grid.Rows[0].Progress != progress {
		t.Fatalf("unexpected subproject grid: %#v", grid)
	}
}

func TestIdeasGridHasNoWeeklyData(t *testing.T) {
	ctx := t.Context()
	q := testkit.Open(t)
	idea, err := task.Create(ctx, q, task.Input{Name: "Idea"})
	if err != nil {
		t.Fatal(err)
	}
	if idea.Name != "Idea" || idea.ProjectID != nil {
		t.Fatalf("unexpected idea task: %#v", idea)
	}
	grid, err := task.LoadGrid(ctx, q, mustFilter(t, "ideas", ""), time.Date(2026, 9, 26, 0, 0, 0, 0, time.UTC), false)
	if err != nil {
		t.Fatal(err)
	}
	if grid.Kind != task.ViewIdeas || len(grid.Weeks) != 0 || len(grid.Rows) != 1 || grid.Rows[0].ProjectName != "" {
		t.Fatalf("unexpected ideas grid: %#v", grid)
	}
}

func TestGridRejectsSubprojectFromAnotherProject(t *testing.T) {
	ctx := t.Context()
	q := testkit.Open(t)
	first, err := project.Create(ctx, q, project.Input{Name: "First", TotalHours: new(10.0), StartDate: "2026-01-05", EndDate: "2026-02-01"})
	if err != nil {
		t.Fatal(err)
	}
	second, err := project.Create(ctx, q, project.Input{Name: "Second", TotalHours: new(10.0), StartDate: "2026-01-05", EndDate: "2026-02-01"})
	if err != nil {
		t.Fatal(err)
	}
	sp, err := subproject.Create(ctx, q, subproject.Input{ProjectID: second.ID, Name: "Second subproject", TotalHours: new(1.0)})
	if err != nil {
		t.Fatal(err)
	}
	grid, err := task.LoadGrid(ctx, q, mustFilter(t, strconv.FormatInt(first.ID, 10), strconv.FormatInt(sp.ID, 10)), time.Date(2026, 1, 10, 0, 0, 0, 0, time.UTC), false)
	httpErr, ok := errors.AsType[web.HTTPError](err)
	if !ok || httpErr.Status != http.StatusBadRequest {
		t.Fatalf("got %v", err)
	}
	if grid.Kind != task.ViewAll || len(grid.Rows) != 0 {
		t.Fatalf("rejected grid load returned grid data: %#v", grid)
	}
}

func TestGridUsesRequestHistoricalEditingAccess(t *testing.T) {
	ctx := t.Context()
	q := testkit.Open(t)
	p, err := project.Create(ctx, q, project.Input{
		Name: "Selected", TotalHours: new(10.0), StartDate: "2026-04-27", EndDate: "2026-05-11",
	})
	if err != nil {
		t.Fatal(err)
	}
	other, err := project.Create(ctx, q, project.Input{
		Name: "Earlier", TotalHours: new(10.0), StartDate: "2026-03-30", EndDate: "2026-04-06",
	})
	if err != nil {
		t.Fatal(err)
	}
	for _, id := range []int64{p.ID, other.ID} {
		item, err := task.Create(ctx, q, task.Input{Name: "Tracked", ProjectID: &id})
		if err != nil {
			t.Fatal(err)
		}
		if item.ProjectID == nil || *item.ProjectID != id {
			t.Fatalf("unexpected task assignment: %#v", item)
		}
	}
	now := time.Date(2026, 5, 2, 0, 0, 0, 0, time.UTC)
	for _, allowHistoricalEditing := range []bool{false, true} {
		for _, id := range []int64{p.ID, other.ID} {
			grid, err := task.LoadGrid(ctx, q, mustFilter(t, strconv.FormatInt(id, 10), ""), now, allowHistoricalEditing)
			if err != nil {
				t.Fatal(err)
			}
			if grid.HistoricalEditingAllowed != allowHistoricalEditing {
				t.Fatalf("project %d: historical editing = %v, want %v", id, grid.HistoricalEditingAllowed, allowHistoricalEditing)
			}
			for _, cell := range grid.Rows[0].Cells {
				wantLocked := !allowHistoricalEditing && cell.WeekStart.String() < "2026-05-01"
				if cell.Locked != wantLocked {
					t.Fatalf("project %d: unexpected lock for %s: %#v", id, cell.WeekStart.String(), cell)
				}
			}
		}
	}
}

func TestGridMarksCurrentWeek(t *testing.T) {
	tests := []struct {
		name      string
		startDate string
		endDate   string
	}{
		{name: "current week in project range", startDate: "2026-09-07", endDate: "2026-09-28"},
		{name: "current week outside project range", startDate: "2026-01-05", endDate: "2026-01-26"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := t.Context()
			q := testkit.Open(t)
			p, err := project.Create(ctx, q, project.Input{
				Name: "Current", TotalHours: new(10.0), StartDate: tt.startDate, EndDate: tt.endDate,
			})
			if err != nil {
				t.Fatal(err)
			}
			if _, err := task.Create(ctx, q, task.Input{Name: "Tracked", ProjectID: &p.ID}); err != nil {
				t.Fatal(err)
			}
			now := time.Date(2026, 9, 16, 0, 0, 0, 0, time.UTC)
			grid, err := task.LoadGrid(ctx, q, mustFilter(t, strconv.FormatInt(p.ID, 10), ""), now, false)
			if err != nil {
				t.Fatal(err)
			}
			if grid.CurrentWeek != "2026-09-14" {
				t.Fatalf("current week = %q, want %q", grid.CurrentWeek, "2026-09-14")
			}
		})
	}
}

func TestIdeasGridHasNoCurrentWeek(t *testing.T) {
	ctx := t.Context()
	q := testkit.Open(t)
	if _, err := task.Create(ctx, q, task.Input{Name: "Idea"}); err != nil {
		t.Fatal(err)
	}
	grid, err := task.LoadGrid(ctx, q, mustFilter(t, "ideas", ""), time.Date(2026, 9, 16, 0, 0, 0, 0, time.UTC), false)
	if err != nil {
		t.Fatal(err)
	}
	if grid.CurrentWeek != "" {
		t.Fatalf("ideas grid current week = %q, want empty", grid.CurrentWeek)
	}
}

func mustFilter(t *testing.T, projectKey, subprojectKey string) task.Filter {
	t.Helper()
	filter, err := task.ParseFilter(projectKey, subprojectKey)
	if err != nil {
		t.Fatal(err)
	}
	return filter
}
