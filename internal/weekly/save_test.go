package weekly_test

import (
	"context"
	"testing"
	"time"

	"cad-development/internal/dbtest"
	"cad-development/internal/project"
	"cad-development/internal/task"
	"cad-development/internal/weekly"
)

func TestSaveProgressAndLock(t *testing.T) {
	ctx := context.Background()
	_, q := dbtest.Open(t)
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
	ctx := context.Background()
	_, q := dbtest.Open(t)
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
	lockedApril := map[string]bool{"2026-03": true}
	p50 := 50.0
	if _, err = weekly.Save(ctx, q, tk.ID, "2026-03-30", weekly.Patch{Progress: &p50}, now, lockedApril); err == nil {
		t.Fatal("expected locked later week conflict")
	}
	result, err := task.Get(ctx, q, tk.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Weeks) != 2 || result.Weeks[1].Progress == nil || *result.Weeks[1].Progress != 40 {
		t.Fatalf("locked later week should stay 40, got %#v", result.Weeks)
	}
	if result.Weeks[0].Progress == nil || *result.Weeks[0].Progress != 20 {
		t.Fatalf("earlier week should remain 20, got %#v", result.Weeks)
	}
}
