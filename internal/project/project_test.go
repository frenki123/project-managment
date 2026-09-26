package project_test

import (
	"context"
	"math"
	"testing"
	"time"

	"cad-development/internal/app/testkit"
	"cad-development/internal/project"
	"cad-development/internal/task"
	"cad-development/internal/weekly"
)

func TestDeleteProjectMakesIdeasAndDeletesWeeks(t *testing.T) {
	ctx := context.Background()
	q := testkit.Open(t)
	p, err := project.Create(ctx, q, project.Input{Name: "P", TotalHours: 10, StartDate: "2026-09-07", EndDate: "2026-10-05"})
	if err != nil {
		t.Fatal(err)
	}
	tk, err := task.Create(ctx, q, task.Input{Name: "T", ProjectID: &p.ID})
	if err != nil {
		t.Fatal(err)
	}
	hours := 2.0
	if _, err := weekly.Save(ctx, q, tk.ID, "2026-09-07", weekly.Patch{PlannedHours: &hours}, testNow(), nil); err != nil {
		t.Fatal(err)
	}
	if err := project.Delete(ctx, q, p.ID); err != nil {
		t.Fatal(err)
	}
	result, err := task.Get(ctx, q, tk.ID)
	if err != nil {
		t.Fatal(err)
	}
	if result.ProjectID != nil {
		t.Fatal("task should become an idea")
	}
	if len(result.Weeks) != 0 {
		t.Fatalf("expected weekly data to be deleted, got %d rows", len(result.Weeks))
	}
}

func TestUpdateRejectsDatesOutsideWeeklyData(t *testing.T) {
	ctx := context.Background()
	q := testkit.Open(t)
	p, err := project.Create(ctx, q, project.Input{Name: "P", TotalHours: 10, StartDate: "2026-09-01", EndDate: "2026-10-31"})
	if err != nil {
		t.Fatal(err)
	}
	tk, err := task.Create(ctx, q, task.Input{Name: "T", ProjectID: &p.ID})
	if err != nil {
		t.Fatal(err)
	}
	hours := 1.0
	if _, err := weekly.Save(ctx, q, tk.ID, "2026-10-05", weekly.Patch{PlannedHours: &hours}, testNow(), nil); err != nil {
		t.Fatal(err)
	}
	p.EndDate = "2026-09-30"
	if _, err := project.Update(ctx, q, p.ID, project.Input{Name: p.Name, TotalHours: p.TotalHours, StartDate: p.StartDate, EndDate: p.EndDate}); err == nil {
		t.Fatal("expected date change to be rejected")
	}
}

func TestCreateRejectsNonFiniteHours(t *testing.T) {
	ctx := context.Background()
	q := testkit.Open(t)
	for _, value := range []float64{math.NaN(), math.Inf(1), math.Inf(-1)} {
		if _, err := project.Create(ctx, q, project.Input{Name: "P", TotalHours: value, StartDate: "2026-09-01", EndDate: "2026-09-30"}); err == nil {
			t.Fatalf("expected non-finite value %v to be rejected", value)
		}
	}
}

func testNow() time.Time {
	return time.Date(2026, 9, 26, 0, 0, 0, 0, time.UTC)
}
