package project_test

import (
	"context"
	"testing"
	"time"

	"cad-development/internal/app/testkit"
	projectdomain "cad-development/internal/project"
	"cad-development/internal/task"
	"cad-development/internal/weekly"
)

func TestLoadSCurve(t *testing.T) {
	q := testkit.Open(t)
	ctx := context.Background()
	p, err := projectdomain.Create(ctx, q, projectdomain.Input{
		Name:       "Project",
		TotalHours: new(100.0),
		StartDate:  "2026-01-05",
		EndDate:    "2026-01-19",
	})
	if err != nil {
		t.Fatal(err)
	}
	tk, err := task.Create(ctx, q, task.Input{Name: "Tracked", ProjectID: new(p.ID)})
	if err != nil {
		t.Fatal(err)
	}
	now := time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)
	planned, spent, progress := 10.0, 4.0, 25.0
	if _, err := weekly.Save(ctx, q, tk.ID, "2026-01-05", weekly.Patch{PlannedHours: &planned, SpentHours: &spent, Progress: &progress}, now); err != nil {
		t.Fatal(err)
	}
	planned, spent = 20, 6
	if _, err := weekly.Save(ctx, q, tk.ID, "2026-01-12", weekly.Patch{PlannedHours: &planned, SpentHours: &spent}, now); err != nil {
		t.Fatal(err)
	}
	progress = 50
	if _, err := weekly.Save(ctx, q, tk.ID, "2026-01-19", weekly.Patch{Progress: &progress}, now); err != nil {
		t.Fatal(err)
	}

	curve, err := projectdomain.LoadSCurve(ctx, q, p.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(curve.Weeks) != 3 {
		t.Fatalf("expected 3 weeks, got %d", len(curve.Weeks))
	}
	want := []projectdomain.SCurveWeek{
		{WeekStart: "2026-01-05", PlannedHours: 10, SpentHours: 4, EarnedHours: 7.5},
		{WeekStart: "2026-01-12", PlannedHours: 30, SpentHours: 10, EarnedHours: 7.5},
		{WeekStart: "2026-01-19", PlannedHours: 30, SpentHours: 10, EarnedHours: 15},
	}
	for i, got := range curve.Weeks {
		if got != want[i] {
			t.Errorf("week %d: got %+v, want %+v", i, got, want[i])
		}
	}
}
