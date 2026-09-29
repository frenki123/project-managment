package project_test

import (
	"math"
	"testing"
	"time"

	"cad-development/internal/app/testkit"
	"cad-development/internal/nullable"
	"cad-development/internal/project"
	"cad-development/internal/subproject"
	"cad-development/internal/task"
	"cad-development/internal/weekly"
)

func TestDeleteProjectRejectsWeeklyHistory(t *testing.T) {
	ctx := t.Context()
	q := testkit.Open(t)
	p, err := project.Create(ctx, q, project.Input{Name: "P", TotalHours: new(10.0), StartDate: "2026-09-07", EndDate: "2026-10-05"})
	if err != nil {
		t.Fatal(err)
	}
	tk, err := task.Create(ctx, q, task.Input{Name: "T", ProjectID: &p.ID})
	if err != nil {
		t.Fatal(err)
	}
	hours := 2.0
	if _, err := weekly.Save(ctx, q, tk.ID, "2026-09-07", weekly.Patch{PlannedHours: &hours}, testNow()); err != nil {
		t.Fatal(err)
	}
	if err := project.Delete(ctx, q, p.ID); err == nil {
		t.Fatal("expected project deletion to be rejected")
	}
	result, err := task.Get(ctx, q, tk.ID)
	if err != nil {
		t.Fatal(err)
	}
	if result.ProjectID == nil {
		t.Fatal("rejected deletion changed task assignment")
	}
	if len(result.Weeks) != 1 {
		t.Fatalf("expected weekly data to remain, got %d rows", len(result.Weeks))
	}
}

func TestUpdateRejectsDatesOutsideWeeklyData(t *testing.T) {
	ctx := t.Context()
	q := testkit.Open(t)
	p, err := project.Create(ctx, q, project.Input{Name: "P", TotalHours: new(10.0), StartDate: "2026-09-01", EndDate: "2026-10-31"})
	if err != nil {
		t.Fatal(err)
	}
	tk, err := task.Create(ctx, q, task.Input{Name: "T", ProjectID: &p.ID})
	if err != nil {
		t.Fatal(err)
	}
	hours := 1.0
	if _, err := weekly.Save(ctx, q, tk.ID, "2026-10-05", weekly.Patch{PlannedHours: &hours}, testNow()); err != nil {
		t.Fatal(err)
	}
	p.EndDate = "2026-09-30"
	if _, err := project.Update(ctx, q, p.ID, project.Patch{
		Name: *nullable.Set(p.Name), TotalHours: *nullable.Set(p.TotalHours),
		StartDate: *nullable.Set(p.StartDate), EndDate: *nullable.Set(p.EndDate),
	}); err == nil {
		t.Fatal("expected date change to be rejected")
	}
	got, err := project.Get(ctx, q, p.ID)
	if err != nil {
		t.Fatal(err)
	}
	if got.StartDate != p.StartDate || got.EndDate != "2026-10-31" {
		t.Fatalf("rejected update changed dates: %#v", got)
	}
}

func TestCreateRejectsNonFiniteHours(t *testing.T) {
	ctx := t.Context()
	q := testkit.Open(t)
	for _, value := range []float64{math.NaN(), math.Inf(1), math.Inf(-1)} {
		if _, err := project.Create(ctx, q, project.Input{Name: "P", TotalHours: new(value), StartDate: "2026-09-01", EndDate: "2026-09-30"}); err == nil {
			t.Fatalf("expected non-finite value %v to be rejected", value)
		}
	}
	projects, err := project.List(ctx, q)
	if err != nil {
		t.Fatal(err)
	}
	if len(projects) != 0 {
		t.Fatalf("rejected creates persisted projects: %#v", projects)
	}
}

func TestProjectNamesAreCaseInsensitiveUnique(t *testing.T) {
	ctx := t.Context()
	q := testkit.Open(t)
	if _, err := project.Create(ctx, q, project.Input{Name: "Alpha", TotalHours: new(10.0), StartDate: "2026-09-01", EndDate: "2026-09-30"}); err != nil {
		t.Fatal(err)
	}
	if _, err := project.Create(ctx, q, project.Input{Name: "alpha", TotalHours: new(10.0), StartDate: "2026-09-01", EndDate: "2026-09-30"}); err == nil {
		t.Fatal("expected duplicate project name to be rejected")
	}
}

func TestUpdateRejectsTotalBelowSubprojects(t *testing.T) {
	ctx := t.Context()
	q := testkit.Open(t)
	p, err := project.Create(ctx, q, project.Input{Name: "P", TotalHours: new(10.0), StartDate: "2026-09-01", EndDate: "2026-09-30"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := subproject.Create(ctx, q, subproject.Input{ProjectID: p.ID, Name: "SP", TotalHours: new(6.0)}); err != nil {
		t.Fatal(err)
	}
	_, err = project.Update(ctx, q, p.ID, project.Patch{
		Name: *nullable.Set(p.Name), TotalHours: *nullable.Set(5.0),
		StartDate: *nullable.Set(p.StartDate), EndDate: *nullable.Set(p.EndDate),
	})
	if err == nil {
		t.Fatal("expected project total below subprojects to be rejected")
	}
	got, err := project.Get(ctx, q, p.ID)
	if err != nil {
		t.Fatal(err)
	}
	if got.TotalHours != 10 {
		t.Fatalf("rejected update changed project total: %v", got.TotalHours)
	}
}

func testNow() time.Time {
	return time.Date(2026, 9, 26, 0, 0, 0, 0, time.UTC)
}
