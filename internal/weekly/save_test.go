package weekly_test

import (
	"math"
	"testing"
	"time"

	"cad-development/internal/app/testkit"
	"cad-development/internal/db"
	"cad-development/internal/monthlock"
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
	unlocked := monthlock.Set{}
	prog := 40.0
	_, err = weekly.Save(ctx, q, tk.ID, "2026-04-27", weekly.Patch{Progress: &prog}, now, unlocked)
	if err == nil {
		t.Fatal("expected locked April week to fail")
	}
	unlocked = monthlock.Set{"2026-04": true}
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

func TestSaveReadsUnlockedPastMonthInsideTransaction(t *testing.T) {
	ctx := t.Context()
	q := testkit.Open(t)
	p, err := project.Create(ctx, q, project.Input{Name: "P", TotalHours: 10, StartDate: "2026-04-06", EndDate: "2026-05-04"})
	if err != nil {
		t.Fatal(err)
	}
	taskID := p.ID
	tk, err := task.Create(ctx, q, task.Input{Name: "T", ProjectID: &taskID})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := q.UpsertMonthLock(ctx, db.UpsertMonthLockParams{YearMonth: "2026-04", Unlocked: 1}); err != nil {
		t.Fatal(err)
	}
	hours := 1.0
	if _, err := weekly.Save(ctx, q, tk.ID, "2026-04-06", weekly.Patch{PlannedHours: &hours}, time.Date(2026, 5, 2, 0, 0, 0, 0, time.UTC), nil); err != nil {
		t.Fatalf("unlocked historical week was rejected: %v", err)
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
	allOpen := monthlock.Set{"2026-03": true, "2026-04": true}
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
	lockedApril := monthlock.Set{"2026-03": true}
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
			if _, err := weekly.Save(ctx, q, tk.ID, weekly.WeekStart(tc.week), tc.patch, now, nil); err == nil {
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

func TestSavePartialPatchPreservesExistingValues(t *testing.T) {
	ctx := t.Context()
	q := testkit.Open(t)
	p, err := project.Create(ctx, q, project.Input{Name: "P", TotalHours: 10, StartDate: "2026-09-01", EndDate: "2026-09-30"})
	if err != nil {
		t.Fatal(err)
	}
	tk, err := task.Create(ctx, q, task.Input{Name: "T", ProjectID: &p.ID})
	if err != nil {
		t.Fatal(err)
	}
	now := time.Date(2026, 9, 26, 0, 0, 0, 0, time.UTC)
	planned, spent, progress := 8.0, 3.0, 25.0
	if _, err := weekly.Save(ctx, q, tk.ID, "2026-09-07", weekly.Patch{PlannedHours: &planned, SpentHours: &spent, Progress: &progress}, now, nil); err != nil {
		t.Fatal(err)
	}
	updatedSpent := 5.0
	cell, err := weekly.Save(ctx, q, tk.ID, "2026-09-07", weekly.Patch{SpentHours: &updatedSpent}, now, nil)
	if err != nil {
		t.Fatal(err)
	}
	if cell.PlannedHours != planned || cell.SpentHours != updatedSpent || cell.Progress == nil || *cell.Progress != progress {
		t.Fatalf("partial patch changed untouched values: %#v", cell)
	}
}
