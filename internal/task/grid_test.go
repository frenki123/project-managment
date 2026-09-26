package task_test

import (
	"context"
	"strconv"
	"testing"
	"time"

	"cad-development/internal/dbtest"
	"cad-development/internal/project"
	"cad-development/internal/task"
	"cad-development/internal/weekly"
)

func TestGridReportsHoursAndProgressSeparately(t *testing.T) {
	ctx := context.Background()
	_, q := dbtest.Open(t)
	p, err := project.Create(ctx, q, project.Input{
		Name: "P", TotalHours: 200, StartDate: "2026-09-01", EndDate: "2026-10-31",
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
	planned := 100.0
	spent := 100.0
	progress := 50.0
	if _, err := weekly.Save(ctx, q, first.ID, "2026-09-07", weekly.Patch{PlannedHours: &planned, SpentHours: &spent, Progress: &progress}, now, nil); err != nil {
		t.Fatal(err)
	}
	if _, err := weekly.Save(ctx, q, second.ID, "2026-09-07", weekly.Patch{PlannedHours: &planned, SpentHours: &spent}, now, nil); err != nil {
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
