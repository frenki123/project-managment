package task_test

import (
	"errors"
	"net/http"
	"strconv"
	"testing"
	"time"

	"cad-development/internal/app"
	"cad-development/internal/app/testkit"
	"cad-development/internal/project"
	"cad-development/internal/subproject"
	"cad-development/internal/task"
	"cad-development/internal/weekly"
)

func TestGridReportsHoursAndProgressSeparately(t *testing.T) {
	ctx := t.Context()
	q := testkit.Open(t)
	p, err := project.Create(ctx, q, project.Input{
		Name: "P", TotalHours: new(300.0), StartDate: "2026-09-01", EndDate: "2026-10-31",
	})
	if err != nil {
		t.Fatal(err)
	}
	firstID := p.ID
	secondID := p.ID
	first, err := task.Create(ctx, q, task.Input{Name: "Half complete", ProjectID: &firstID})
	if err != nil {
		t.Fatal(err)
	}
	second, err := task.Create(ctx, q, task.Input{Name: "Not started", ProjectID: &secondID})
	if err != nil {
		t.Fatal(err)
	}
	now := time.Date(2026, 9, 26, 0, 0, 0, 0, time.UTC)
	firstPlanned := 150.0
	firstSpent := 100.0
	secondPlanned := 50.0
	secondSpent := 100.0
	progress := 50.0
	if _, err := weekly.Save(ctx, q, first.ID, "2026-09-07", weekly.Patch{PlannedHours: &firstPlanned, SpentHours: &firstSpent, Progress: &progress}, now, nil); err != nil {
		t.Fatal(err)
	}
	if _, err := weekly.Save(ctx, q, second.ID, "2026-09-07", weekly.Patch{PlannedHours: &secondPlanned, SpentHours: &secondSpent}, now, nil); err != nil {
		t.Fatal(err)
	}

	grid, err := task.LoadGrid(ctx, q, strconv.FormatInt(p.ID, 10), nil, now)
	if err != nil {
		t.Fatal(err)
	}
	if grid.PlannedHours != 200 || grid.SpentHours != 200 {
		t.Fatalf("unexpected hour totals: planned=%v spent=%v", grid.PlannedHours, grid.SpentHours)
	}
	if grid.ProgressPct == nil || *grid.ProgressPct != 25 {
		t.Fatalf("expected 25%% progress, got %v", grid.ProgressPct)
	}
	for i, week := range grid.Weeks {
		if week.Start != "2026-09-07" {
			continue
		}
		total := grid.WeekTotals[i]
		if total.Planned != 200 || total.Spent != 200 || total.Earned != 75 || total.CumulativePlanned != 200 || total.CumulativeSpent != 200 {
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
			if cell.WeekStart == "2026-09-14" {
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
	if _, err := weekly.Save(ctx, q, item.ID, "2026-01-05", weekly.Patch{PlannedHours: &hours}, time.Date(2026, 1, 10, 0, 0, 0, 0, time.UTC), nil); err != nil {
		t.Fatal(err)
	}
	secondID := second.ID
	_, err = task.Update(ctx, q, item.ID, task.Input{Name: item.Name, ProjectID: &secondID})
	httpErr, ok := errors.AsType[app.HTTPError](err)
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
	if _, err := weekly.Save(ctx, q, item.ID, "2026-09-07", weekly.Patch{PlannedHours: &planned, SpentHours: &spent, Progress: &progress}, now, nil); err != nil {
		t.Fatal(err)
	}
	grid, err := task.LoadGrid(ctx, q, strconv.FormatInt(p.ID, 10), &subprojectID, now)
	if err != nil {
		t.Fatal(err)
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
	if _, err := task.Create(ctx, q, task.Input{Name: "Idea"}); err != nil {
		t.Fatal(err)
	}
	grid, err := task.LoadGrid(ctx, q, "ideas", nil, time.Date(2026, 9, 26, 0, 0, 0, 0, time.UTC))
	if err != nil {
		t.Fatal(err)
	}
	if !grid.Ideas || len(grid.Weeks) != 0 || len(grid.Rows) != 1 || grid.Rows[0].ProjectName != "" {
		t.Fatalf("unexpected ideas grid: %#v", grid)
	}
}

func TestGridResetsSubprojectFromAnotherProject(t *testing.T) {
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
	grid, err := task.LoadGrid(ctx, q, strconv.FormatInt(first.ID, 10), &sp.ID, time.Date(2026, 1, 10, 0, 0, 0, 0, time.UTC))
	if err != nil {
		t.Fatal(err)
	}
	if grid.FilterSubproject != "" || grid.Subprojects[0].Selected == false {
		t.Fatalf("expected subproject filter reset: %#v", grid)
	}
}

func TestGridPastMonthsIncludesDisplayedMondayBeforeProjectStart(t *testing.T) {
	ctx := t.Context()
	q := testkit.Open(t)
	_, err := project.Create(ctx, q, project.Input{
		Name: "Earlier", TotalHours: new(10.0), StartDate: "2026-01-01", EndDate: "2026-01-31",
	})
	if err != nil {
		t.Fatal(err)
	}
	p, err := project.Create(ctx, q, project.Input{
		Name: "Selected", TotalHours: new(10.0), StartDate: "2026-02-02", EndDate: "2026-02-28",
	})
	if err != nil {
		t.Fatal(err)
	}
	grid, err := task.LoadGrid(ctx, q, strconv.FormatInt(p.ID, 10), nil, time.Date(2026, 2, 15, 0, 0, 0, 0, time.UTC))
	if err != nil {
		t.Fatal(err)
	}
	if len(grid.PastMonths) != 2 || grid.PastMonths[0] != "2025-12" || grid.PastMonths[1] != "2026-01" {
		t.Fatalf("expected months from first displayed Monday, got %v", grid.PastMonths)
	}
}
