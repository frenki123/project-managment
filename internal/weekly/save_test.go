package weekly_test

import (
	"math"
	"testing"
	"time"

	"cad-development/internal/app/testkit"
	"cad-development/internal/project"
	"cad-development/internal/task"
	"cad-development/internal/weekly"
)

func TestSaveProgressAndLock(t *testing.T) {
	ctx := t.Context()
	q := testkit.Open(t)
	p, err := project.Create(ctx, q, project.Input{
		Name: "P", TotalHours: 100, StartDate: "2026-01-05", EndDate: "2026-06-01",
	})
	if err != nil {
		t.Fatal(err)
	}
	pid := p.ID
	tk, err := task.Create(ctx, q, task.Input{Name: "T", ProjectID: &pid})
	if err != nil {
		t.Fatal(err)
	}
	now := time.Date(2026, 5, 2, 0, 0, 0, 0, time.UTC)
	unlocked := map[string]bool{}
	prog := 40.0
	_, err = weekly.Save(ctx, q, tk.ID, "2026-04-27", weekly.Patch{Progress: &prog}, now, unlocked)
	if err == nil {
		t.Fatal("expected locked April week to fail")
	}
	unlocked["2026-04"] = true
	if _, err = weekly.Save(ctx, q, tk.ID, "2026-04-27", weekly.Patch{Progress: &prog}, now, unlocked); err != nil {
		t.Fatal(err)
	}
	later := 30.0
	if _, err = weekly.Save(ctx, q, tk.ID, "2026-05-04", weekly.Patch{Progress: &later}, now, unlocked); err == nil {
		t.Fatal("progress cannot go down")
	}
	up := 50.0
	if _, err = weekly.Save(ctx, q, tk.ID, "2026-05-04", weekly.Patch{Progress: &up}, now, unlocked); err != nil {
		t.Fatal(err)
	}
	low := 45.0
	if _, err = weekly.Save(ctx, q, tk.ID, "2026-05-11", weekly.Patch{Progress: &low}, now, unlocked); err == nil {
		t.Fatal("cannot be below previous week")
	}
	idea, err := task.Create(ctx, q, task.Input{Name: "Idea"})
	if err != nil {
		t.Fatal(err)
	}
	h := 1.0
	if _, err = weekly.Save(ctx, q, idea.ID, "2026-05-04", weekly.Patch{PlannedHours: &h}, now, unlocked); err == nil {
		t.Fatal("ideas cannot be planned")
	}
}

func TestCascadeRejectsLockedConflict(t *testing.T) {
	ctx := t.Context()
	q := testkit.Open(t)
	p, err := project.Create(ctx, q, project.Input{
		Name: "P", TotalHours: 100, StartDate: "2026-03-02", EndDate: "2026-06-01",
	})
	if err != nil {
		t.Fatal(err)
	}
	pid := p.ID
	tk, err := task.Create(ctx, q, task.Input{Name: "T", ProjectID: &pid})
	if err != nil {
		t.Fatal(err)
	}
	now := time.Date(2026, 5, 2, 0, 0, 0, 0, time.UTC)
	allOpen := map[string]bool{"2026-03": true, "2026-04": true}
	p20 := 20.0
	p40 := 40.0
	if _, err = weekly.Save(ctx, q, tk.ID, "2026-03-30", weekly.Patch{Progress: &p20}, now, allOpen); err != nil {
		t.Fatal(err)
	}
	if _, err = weekly.Save(ctx, q, tk.ID, "2026-04-06", weekly.Patch{Progress: &p40}, now, allOpen); err != nil {
		t.Fatal(err)
	}
	p50 := 50.0
	if _, err = weekly.Save(ctx, q, tk.ID, "2026-03-30", weekly.Patch{Progress: &p50}, now, allOpen); err != nil {
		t.Fatal(err)
	}
	result, err := task.Get(ctx, q, tk.ID)
	if err != nil {
		t.Fatal(err)
	}
	if result.Weeks[1].Progress == nil || *result.Weeks[1].Progress != 50 {
		t.Fatalf("later progress should cascade to 50, got %#v", result.Weeks)
	}
	lockedApril := map[string]bool{"2026-03": true}
	p60 := 60.0
	if _, err = weekly.Save(ctx, q, tk.ID, "2026-03-30", weekly.Patch{Progress: &p60}, now, lockedApril); err == nil {
		t.Fatal("expected locked later week conflict")
	}
	result, err = task.Get(ctx, q, tk.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Weeks) != 2 || result.Weeks[1].Progress == nil || *result.Weeks[1].Progress != 50 {
		t.Fatalf("locked later week should stay 50, got %#v", result.Weeks)
	}
	if result.Weeks[0].Progress == nil || *result.Weeks[0].Progress != 50 {
		t.Fatalf("earlier week should remain 50, got %#v", result.Weeks)
	}
}

func TestSaveRejectsInvalidPatches(t *testing.T) {
	ctx := t.Context()
	q := testkit.Open(t)
	p, err := project.Create(ctx, q, project.Input{
		Name: "P", TotalHours: 100, StartDate: "2026-01-05", EndDate: "2026-06-01",
	})
	if err != nil {
		t.Fatal(err)
	}
	taskID := p.ID
	tk, err := task.Create(ctx, q, task.Input{Name: "T", ProjectID: &taskID})
	if err != nil {
		t.Fatal(err)
	}
	now := time.Date(2026, 1, 2, 0, 0, 0, 0, time.UTC)
	negative := -1.0
	infinite := math.Inf(1)
	tooMuch := 101.0
	values := []struct {
		name  string
		patch weekly.Patch
		week  string
	}{
		{"empty patch", weekly.Patch{}, "2026-01-05"},
		{"negative hours", weekly.Patch{PlannedHours: &negative}, "2026-01-05"},
		{"infinite hours", weekly.Patch{SpentHours: &infinite}, "2026-01-05"},
		{"progress above 100", weekly.Patch{Progress: &tooMuch}, "2026-01-05"},
		{"non-Monday", weekly.Patch{PlannedHours: &negative}, "2026-01-06"},
		{"outside project range", weekly.Patch{PlannedHours: &negative}, "2026-06-08"},
	}
	for _, tc := range values {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := weekly.Save(ctx, q, tk.ID, tc.week, tc.patch, now, nil); err == nil {
				t.Fatal("expected invalid patch to fail")
			}
		})
	}
	result, err := task.Get(ctx, q, tk.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Weeks) != 0 {
		t.Fatalf("invalid patches should not create weekly data, got %#v", result.Weeks)
	}
}
