package project_test

import (
	"errors"
	"math"
	"net/http"
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
	if _, err := weekly.Save(ctx, q, tk.ID, "2026-09-07", weekly.Patch{PlannedHours: nullable.Present(hours)}, testNow()); err != nil {
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
	if _, err := weekly.Save(ctx, q, tk.ID, "2026-10-05", weekly.Patch{PlannedHours: nullable.Present(hours)}, testNow()); err != nil {
		t.Fatal(err)
	}
	p.EndDate = "2026-09-30"
	_, err = project.Update(ctx, q, p.ID, project.Patch{
		Name: nullable.Present(p.Name), TotalHours: nullable.Present(p.TotalHours),
		StartDate: nullable.Present(p.StartDate), EndDate: nullable.Present(p.EndDate),
	})
	if err == nil {
		t.Fatal("expected date change to be rejected")
	} else if httpErr, ok := errors.AsType[web.HTTPError](err); !ok || httpErr.Reason != "project-dates-exclude-weekly-data" || httpErr.Message != "project dates cannot exclude existing weekly data" {
		t.Fatalf("unexpected conflict error: %v", err)
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
	} else {
		var httpErr web.HTTPError
		if !errors.As(err, &httpErr) || httpErr.Status != http.StatusConflict || httpErr.Message != "project name already exists" {
			t.Fatalf("unexpected duplicate error: %v", err)
		}
	}
}

func TestCreateReportsNameConflictBeforeInvalidHours(t *testing.T) {
	ctx := t.Context()
	q := testkit.Open(t)
	if _, err := project.Create(ctx, q, project.Input{Name: "Alpha", TotalHours: new(10.0), StartDate: "2026-09-01", EndDate: "2026-09-30"}); err != nil {
		t.Fatal(err)
	}
	_, err := project.Create(ctx, q, project.Input{Name: "Alpha", TotalHours: new(-1.0), StartDate: "2026-09-01", EndDate: "2026-09-30"})
	var httpErr web.HTTPError
	if !errors.As(err, &httpErr) || httpErr.Status != http.StatusConflict || httpErr.Message != "project name already exists" {
		t.Fatalf("expected name conflict before invalid hours, got %v", err)
	}
	projects, err := project.List(ctx, q)
	if err != nil {
		t.Fatal(err)
	}
	if len(projects) != 1 {
		t.Fatalf("rejected create persisted a project: %#v", projects)
	}
}

func TestUpdateReportsNameConflictBeforeInvalidHours(t *testing.T) {
	ctx := t.Context()
	q := testkit.Open(t)
	first, err := project.Create(ctx, q, project.Input{Name: "Alpha", TotalHours: new(10.0), StartDate: "2026-09-01", EndDate: "2026-09-30"})
	if err != nil {
		t.Fatal(err)
	}
	second, err := project.Create(ctx, q, project.Input{Name: "Beta", TotalHours: new(10.0), StartDate: "2026-09-01", EndDate: "2026-09-30"})
	if err != nil {
		t.Fatal(err)
	}
	_, err = project.Update(ctx, q, second.ID, project.Patch{
		Name: nullable.Present(first.Name), TotalHours: nullable.Present(-1.0),
		StartDate: nullable.Present(second.StartDate), EndDate: nullable.Present(second.EndDate),
	})
	var httpErr web.HTTPError
	if !errors.As(err, &httpErr) || httpErr.Status != http.StatusConflict || httpErr.Message != "project name already exists" {
		t.Fatalf("expected name conflict before invalid hours, got %v", err)
	}
	got, err := project.Get(ctx, q, second.ID)
	if err != nil {
		t.Fatal(err)
	}
	if got.Name != "Beta" || got.TotalHours != 10 {
		t.Fatalf("rejected update changed the project: %#v", got)
	}
}

func TestUpdateReportsNameConflictWithSurroundingWhitespace(t *testing.T) {
	ctx := t.Context()
	q := testkit.Open(t)
	if _, err := project.Create(ctx, q, project.Input{Name: "Alpha", TotalHours: new(10.0), StartDate: "2026-09-01", EndDate: "2026-09-30"}); err != nil {
		t.Fatal(err)
	}
	second, err := project.Create(ctx, q, project.Input{Name: "Beta", TotalHours: new(10.0), StartDate: "2026-09-01", EndDate: "2026-09-30"})
	if err != nil {
		t.Fatal(err)
	}
	_, err = project.Update(ctx, q, second.ID, project.Patch{
		Name: nullable.Present("  Alpha  "), TotalHours: nullable.Present(second.TotalHours),
		StartDate: nullable.Present(second.StartDate), EndDate: nullable.Present(second.EndDate),
	})
	var httpErr web.HTTPError
	if !errors.As(err, &httpErr) || httpErr.Status != http.StatusConflict || httpErr.Message != "project name already exists" {
		t.Fatalf("expected trimmed name conflict, got %v", err)
	}
}

func TestCreateEmptyNameReturnsBadRequest(t *testing.T) {
	ctx := t.Context()
	q := testkit.Open(t)
	_, err := project.Create(ctx, q, project.Input{Name: "   ", TotalHours: new(10.0), StartDate: "2026-09-01", EndDate: "2026-09-30"})
	var httpErr web.HTTPError
	if !errors.As(err, &httpErr) || httpErr.Status != http.StatusBadRequest || httpErr.Message != "name is required" {
		t.Fatalf("expected name required, got %v", err)
	}
	projects, err := project.List(ctx, q)
	if err != nil {
		t.Fatal(err)
	}
	if len(projects) != 0 {
		t.Fatalf("empty name create persisted a project: %#v", projects)
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
		Name: nullable.Present(p.Name), TotalHours: nullable.Present(5.0),
		StartDate: nullable.Present(p.StartDate), EndDate: nullable.Present(p.EndDate),
	})
	if err == nil {
		t.Fatal("expected project total below subprojects to be rejected")
	} else if httpErr, ok := errors.AsType[web.HTTPError](err); !ok || httpErr.Message != "project hours cannot be less than subproject hours" {
		t.Fatalf("unexpected conflict error: %v", err)
	}
	got, err := project.Get(ctx, q, p.ID)
	if err != nil {
		t.Fatal(err)
	}
	if got.TotalHours != 10 {
		t.Fatalf("rejected update changed project total: %v", got.TotalHours)
	}
}

func TestEarnedHoursKeepsFractionalHours(t *testing.T) {
	ctx := t.Context()
	q := testkit.Open(t)
	p, err := project.Create(ctx, q, project.Input{Name: "P", TotalHours: new(100.0), StartDate: "2026-09-01", EndDate: "2026-09-30"})
	if err != nil {
		t.Fatal(err)
	}
	tk, err := task.Create(ctx, q, task.Input{Name: "T", ProjectID: &p.ID})
	if err != nil {
		t.Fatal(err)
	}
	planned, progress := 100.0, 62.9
	if _, err := weekly.Save(ctx, q, tk.ID, "2026-09-07", weekly.Patch{PlannedHours: nullable.Present(planned), Progress: nullable.Present(progress)}, testNow()); err != nil {
		t.Fatal(err)
	}
	got, err := project.Get(ctx, q, p.ID)
	if err != nil {
		t.Fatal(err)
	}
	if math.Abs(got.EarnedHours-62.9) > 1e-9 {
		t.Fatalf("earned_hours should keep fractional hours 62.9, got %v", got.EarnedHours)
	}
}

func testNow() time.Time {
	return time.Date(2026, 9, 26, 0, 0, 0, 0, time.UTC)
}
